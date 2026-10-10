package providers

import (
	"context"
	"fmt"
	"isllmalive/internal/config"
	"isllmalive/internal/status"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRoundSharesSourceButNotComponents(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		fmt.Fprint(w, `{"status":{"indicator":"none"},"components":[{"name":"API","status":"operational"},{"name":"Web","status":"major_outage"}]}`)
	}))
	defer server.Close()
	monitors := []config.MonitorConfig{{Type: "statuspage", Name: "api", Endpoint: server.URL, Components: []string{"API"}}, {Type: "statuspage", Name: "web", Endpoint: server.URL, Components: []string{"Web"}}}
	e := NewEngine()
	round := e.NewRound(t.Context(), monitors)
	got := make([]MonitorResult, 2)
	var wg sync.WaitGroup
	for i, m := range monitors {
		wg.Go(func() { got[i] = round.Fetch(t.Context(), m) })
	}
	wg.Wait()
	if hits.Load() != 1 || got[0].Status != status.Normal || got[1].Status != status.Outage {
		t.Fatalf("hits=%d results=%v", hits.Load(), got)
	}
	got[0].ComponentStatuses[0].Status = status.Outage
	if again := round.Fetch(t.Context(), monitors[0]); again.Status != status.Normal {
		t.Fatal("shared snapshot mutated")
	}
	_ = e.NewRound(t.Context(), monitors).Fetch(t.Context(), monitors[0])
	if hits.Load() != 2 {
		t.Fatal("new round reused health")
	}
}
func TestRoundFailureNeverReplaysNormal(t *testing.T) {
	var hits atomic.Int32
	var invalid atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if invalid.Load() {
			fmt.Fprint(w, "{}")
		} else {
			fmt.Fprint(w, `{"status":{"indicator":"none"}}`)
		}
	}))
	defer server.Close()
	m := config.MonitorConfig{Type: "statuspage", Name: "test", Endpoint: server.URL}
	e := NewEngine()
	if e.NewRound(t.Context(), []config.MonitorConfig{m}).Fetch(t.Context(), m).Status != status.Normal {
		t.Fatal("expected initial normal")
	}
	invalid.Store(true)
	if e.NewRound(t.Context(), []config.MonitorConfig{m}).Fetch(t.Context(), m).Status != status.Unknown {
		t.Fatal("old normal replayed")
	}
	if hits.Load() != 2 {
		t.Fatal(hits.Load())
	}
}
func TestRoundSharesRetryBudget(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); w.WriteHeader(503) }))
	defer server.Close()
	m := config.MonitorConfig{Type: "statuspage", Name: "test", Endpoint: server.URL}
	r := NewEngine().NewRound(t.Context(), []config.MonitorConfig{m})
	var wg sync.WaitGroup
	for range 5 {
		wg.Go(func() {
			if got := r.Fetch(t.Context(), m); got.Status != status.Unknown || got.Err == nil {
				t.Error("expected failure")
			}
		})
	}
	wg.Wait()
	if hits.Load() != 3 {
		t.Fatalf("hits=%d want shared 3 attempts", hits.Load())
	}
}
func TestDistinctSourcesAreNotMerged(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		fmt.Fprint(w, `{"status":{"indicator":"none"}}`)
	}))
	defer server.Close()
	a := config.MonitorConfig{Type: "statuspage", Endpoint: server.URL + "/a"}
	b := a
	b.Endpoint = server.URL + "/b"
	r := NewEngine().NewRound(t.Context(), []config.MonitorConfig{a, b})
	r.Fetch(t.Context(), a)
	r.Fetch(t.Context(), b)
	if hits.Load() != 2 {
		t.Fatal("distinct source merged")
	}
}
func TestGoogleKeyCacheExpiryInvalidationAndSharing(t *testing.T) {
	var cache googleKeyCache
	var loads atomic.Int32
	load := func() (string, error) { loads.Add(1); return "key", nil }
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			if _, err := cache.get(t.Context(), load); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if loads.Load() != 1 {
		t.Fatal("concurrent key downloads")
	}
	cache.invalidate()
	cache.get(t.Context(), load)
	cache.mu.Lock()
	cache.expires = time.Now().Add(-time.Second)
	cache.mu.Unlock()
	cache.get(t.Context(), load)
	if loads.Load() != 3 {
		t.Fatal("key not invalidated/expired")
	}
}
func TestRoundWaiterCanCancel(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		fmt.Fprint(w, `{"status":{"indicator":"none"}}`)
	}))
	defer server.Close()
	m := config.MonitorConfig{Type: "statuspage", Endpoint: server.URL}
	r := NewEngine().NewRound(t.Context(), []config.MonitorConfig{m})
	done := make(chan struct{})
	go func() { r.Fetch(t.Context(), m); close(done) }()
	<-entered
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if got := r.Fetch(ctx, m); got.Err == nil {
		t.Fatal("waiter ignored cancellation")
	}
	close(release)
	<-done
}

func TestGoogleAuthenticationRefreshesKeyOnce(t *testing.T) {
	var pages, rpcs atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/page" {
			n := pages.Add(1)
			fmt.Fprintf(w, `"WIu0Nc":"key%d"`, n)
			return
		}
		rpcs.Add(1)
		if r.Header.Get("X-Goog-Api-Key") == "key1" {
			w.WriteHeader(400)
			fmt.Fprint(w, `{"reason":"API_KEY_INVALID"}`)
			return
		}
		fmt.Fprint(w, "[[[]]]")
	}))
	defer server.Close()
	oldPage, oldRPC := googleStatusURL, googleHistoryURL
	googleStatusURL = server.URL + "/page"
	googleHistoryURL = server.URL + "/rpc"
	t.Cleanup(func() { googleStatusURL = oldPage; googleHistoryURL = oldRPC })
	e := NewEngine()
	m := config.MonitorConfig{Type: "google"}
	for range 2 {
		got := e.NewRound(t.Context(), []config.MonitorConfig{m}).Fetch(t.Context(), m)
		if got.Status != status.Normal {
			t.Fatalf("status=%s err=%v", got.Status, got.Err)
		}
	}
	if pages.Load() != 2 || rpcs.Load() != 3 {
		t.Fatalf("pages=%d rpcs=%d", pages.Load(), rpcs.Load())
	}
}
func TestDeepSeekFallbackUnionProbesEachTargetOnce(t *testing.T) {
	var feeds, apis, webs atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/feed":
			feeds.Add(1)
			w.WriteHeader(503)
		case "/api":
			apis.Add(1)
			w.WriteHeader(401)
		case "/web":
			webs.Add(1)
			w.WriteHeader(502)
		}
	}))
	defer server.Close()
	withDeepSeekProbeURLs(t, server.URL+"/api", server.URL+"/web")
	a := config.MonitorConfig{Type: "deepseek", Enabled: true, Endpoint: server.URL + "/feed", Components: []string{"API Service"}}
	b := a
	b.Components = []string{"Web Chat Service"}
	r := NewEngine().NewRound(t.Context(), []config.MonitorConfig{a, b})
	var one, two MonitorResult
	var wg sync.WaitGroup
	wg.Go(func() { one = r.Fetch(t.Context(), a) })
	wg.Go(func() { two = r.Fetch(t.Context(), b) })
	wg.Wait()
	if apis.Load() != 1 || webs.Load() != 1 || feeds.Load() != 6 {
		t.Fatalf("feed=%d api=%d web=%d", feeds.Load(), apis.Load(), webs.Load())
	}
	if one.Status != status.Normal || two.Status != status.Outage {
		t.Fatalf("api=%s web=%s", one.Status, two.Status)
	}
}
