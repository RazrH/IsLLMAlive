package providers

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"isllmalive/internal/config"
	"isllmalive/internal/status"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDecompressedResponseLimit(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		io.Copy(gz, strings.NewReader(strings.Repeat(" ", int(maxResponseBytes)+1)))
		gz.Close()
	}))
	defer server.Close()
	m := config.MonitorConfig{Type: "statuspage", Endpoint: server.URL}
	e := NewEngine()
	defer e.Close()
	got := e.NewRound(t.Context(), []config.MonitorConfig{m}).Fetch(t.Context(), m)
	if got.Status != status.Unknown || !errors.Is(got.Err, ErrResponseTooLarge) {
		t.Fatalf("status=%v err=%v", got.Status, got.Err)
	}
	if hits.Load() != 1 {
		t.Fatal("oversized response retried")
	}
	if _, err := readResponse(strings.NewReader(strings.Repeat("x", int(maxResponseBytes)))); err != nil {
		t.Fatal("exact limit rejected")
	}
}
func TestTransportReuseAndClose(t *testing.T) {
	var connections, closed atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"status":{"indicator":"none"}}`) }))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			connections.Add(1)
		}
		if state == http.StateClosed {
			closed.Add(1)
		}
	}
	server.Start()
	defer server.Close()
	m := config.MonitorConfig{Type: "statuspage", Endpoint: server.URL}
	e := NewEngine()
	for range 3 {
		if r := e.NewRound(t.Context(), []config.MonitorConfig{m}).Fetch(t.Context(), m); r.Status != status.Normal {
			t.Fatal(r.Err)
		}
	}
	if connections.Load() != 1 {
		t.Fatalf("connections=%d want reuse", connections.Load())
	}
	if e.utls != nil {
		t.Fatal("uTLS initialized on standard path")
	}
	e.Close()
	deadline := time.After(time.Second)
	for closed.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("idle connection not closed")
		case <-time.After(time.Millisecond):
		}
	}
}
func TestRSSDeadlineLeavesDirectProbeBudget(t *testing.T) {
	var probes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rss" {
			<-r.Context().Done()
			return
		}
		probes.Add(1)
		w.WriteHeader(401)
	}))
	defer server.Close()
	withDeepSeekProbeURLs(t, server.URL+"/api", server.URL+"/web")
	ctx, cancel := context.WithTimeout(t.Context(), 13*time.Second)
	defer cancel()
	m := config.MonitorConfig{Type: "deepseek", Endpoint: server.URL + "/rss", Components: []string{"API"}}
	e := NewEngine()
	defer e.Close()
	started := time.Now()
	got := e.NewRound(ctx, []config.MonitorConfig{m}).Fetch(ctx, m)
	if got.Status != status.Normal || probes.Load() != 1 {
		t.Fatalf("fallback did not run: status=%s probes=%d err=%v", got.Status, probes.Load(), got.Err)
	}
	if time.Since(started) > 12*time.Second {
		t.Fatal("RSS consumed fallback budget")
	}
}
