package providers

import (
	"encoding/xml"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"isllmalive/internal/config"
	"isllmalive/internal/status"
)

func deepSeekRoleRSS(t *testing.T, labels, title, lifecycle string) []byte {
	t.Helper()
	feed := deepSeekRSS{Channel: &deepSeekRSSChannel{Items: []deepSeekRSSItem{{
		Title: title, Link: "https://status.deepseek.com/incidents/role-test",
		Description: "<p><strong>Status:</strong> " + lifecycle + "</p><p><strong>Affected components:</strong> " + labels + "</p>",
		PubDate:     "Thu, 28 May 2026 19:24:51 +0800",
	}}}}
	data, err := xml.Marshal(feed)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDeepSeekRoleIsolationAndVersions(t *testing.T) {
	for _, tt := range []struct {
		labels     string
		want       []status.Status
		diagnostic string
	}{
		{"DeepSeek V4.1 Flash API服务(API Service), 对话服务(Chatservice)", []status.Status{status.Degraded, status.Normal, status.Degraded, status.Degraded}, ""},
		{"DeepSeek V4 Pro API服务(API Service)", []status.Status{status.Degraded, status.Degraded, status.Normal, status.Normal}, ""},
		{"Future V99.17-Pro API, Future V123 Flash API", []status.Status{status.Degraded, status.Degraded, status.Degraded, status.Normal}, ""},
		{"DeepSeek V4 Pro API, DeepSeek V100 Pro API", []status.Status{status.Degraded, status.Degraded, status.Normal, status.Normal}, ""},
		{"API Service", []status.Status{status.Degraded, status.Unknown, status.Unknown, status.Normal}, "does not distinguish"},
		{"Profile API, Process API", []status.Status{status.Degraded, status.Unknown, status.Unknown, status.Normal}, "Profile"},
		{"DeepSeek Turbo API Service", []status.Status{status.Degraded, status.Unknown, status.Unknown, status.Normal}, "Turbo"},
		{"Chat Service, Chatservice", []status.Status{status.Normal, status.Normal, status.Normal, status.Degraded}, ""},
		{"File Upload Service, Search Service", []status.Status{status.Normal, status.Normal, status.Normal, status.Normal}, ""},
		{"Flash API, Future Service", []status.Status{status.Degraded, status.Unknown, status.Degraded, status.Unknown}, "Future Service"},
	} {
		t.Run(tt.labels, func(t *testing.T) {
			names := []string{"API Service", "Pro API", "Flash API", "Web Chat Service"}
			rss := deepSeekRoleRSS(t, tt.labels, "DeepSeek Web/API Degraded Performance", "investigating")
			got := parseDeepSeekRSS(rss, config.MonitorConfig{Components: names}, deepSeekTestResult(), deepSeekTestNow())
			if len(got.ComponentStatuses) != len(tt.want) {
				t.Fatalf("components: %#v", got.ComponentStatuses)
			}
			for i, want := range tt.want {
				if got.ComponentStatuses[i].Status != want {
					t.Errorf("%s=%s, want %s", names[i], got.ComponentStatuses[i].Status, want)
				}
			}
			if tt.diagnostic == "" && got.Err != nil {
				t.Errorf("unexpected diagnostic: %v", got.Err)
			}
			if tt.diagnostic != "" && (got.Err == nil || !strings.Contains(got.Err.Error(), tt.diagnostic)) {
				t.Errorf("diagnostic=%v, want %q", got.Err, tt.diagnostic)
			}
		})
	}
}

func TestDeepSeekRoleResolvedAndUnknown(t *testing.T) {
	for _, tt := range []struct {
		labels, lifecycle string
		want              status.Status
		diagnostic        string
	}{
		{"Flash API, Future Service", "resolved", status.Normal, ""},
		{"Future Service", "investigating", status.Unknown, "Future Service"},
	} {
		got := parseDeepSeekRSS(deepSeekRoleRSS(t, tt.labels, "DeepSeek unavailable", tt.lifecycle), config.MonitorConfig{Components: []string{"Pro API", "Flash API"}}, deepSeekTestResult(), deepSeekTestNow())
		if got.Status != tt.want {
			t.Errorf("%s/%s=%s, want %s", tt.labels, tt.lifecycle, got.Status, tt.want)
		}
		if tt.diagnostic != "" && (got.Err == nil || !strings.Contains(got.Err.Error(), tt.diagnostic)) {
			t.Errorf("diagnostic=%v", got.Err)
		}
		if tt.diagnostic == "" && got.Err != nil {
			t.Errorf("resolved diagnostic=%v", got.Err)
		}
	}
}

func TestDeepSeekPartialOutageBeforeUnavailable(t *testing.T) {
	for _, title := range []string{"DeepSeek Web/API Partially Unavailable", "DeepSeek 网页/API 部分中断", "API partial outage"} {
		got := parseDeepSeekRSS(deepSeekRoleRSS(t, "Flash API", title, "monitoring"), config.MonitorConfig{Components: []string{"Flash API"}}, deepSeekTestResult(), deepSeekTestNow())
		if got.Status != status.Degraded {
			t.Errorf("%s=%s, want Degraded", title, got.Status)
		}
	}
	got := parseDeepSeekRSS(deepSeekRoleRSS(t, "Pro API", "API unavailable", "identified"), config.MonitorConfig{Components: []string{"Pro API"}}, deepSeekTestResult(), deepSeekTestNow())
	if got.Status != status.Outage {
		t.Fatalf("full outage=%s", got.Status)
	}
}

func TestDeepSeekCapturedFlashIncident(t *testing.T) {
	data, err := os.ReadFile("testdata/deepseek-flash-chat.rss")
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "> resolved", "> investigating", 1))
	got := parseDeepSeekRSS(data, config.MonitorConfig{Components: []string{"Pro API", "Flash API", "Chat Service"}}, deepSeekTestResult(), time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC))
	if got.Err != nil || len(got.ComponentStatuses) != 3 {
		t.Fatalf("captured event=%#v", got)
	}
	for i, want := range []status.Status{status.Normal, status.Degraded, status.Degraded} {
		if got.ComponentStatuses[i].Status != want {
			t.Errorf("%s=%s, want %s", got.ComponentStatuses[i].Name, got.ComponentStatuses[i].Status, want)
		}
	}
}

func TestDeepSeekRolesNeverUseGatewayHealth(t *testing.T) {
	cfg := config.MonitorConfig{Type: "deepseek", Components: []string{"Pro API", "Flash API"}}
	engine := NewEngine()
	t.Cleanup(engine.Close)
	round := engine.NewRound(t.Context(), []config.MonitorConfig{cfg})
	if len(round.probes[StateSource(cfg)]) != 0 {
		t.Errorf("model-only direct probe targets=%v", round.probes[StateSource(cfg)])
	}
	rssErr := errors.New("RSS unavailable")
	snapshot := deepSeekProbeSnapshot{probes: map[string]deepSeekProbeResult{"api": {Status: status.Normal}}, rssErr: rssErr}
	got := snapshot.evaluate(cfg, deepSeekTestResult())
	if got.Status != status.Unknown || got.Err == nil {
		t.Fatalf("gateway promoted to model health: %#v", got)
	}
	for _, component := range got.ComponentStatuses {
		if component.Status != status.Unknown || !strings.Contains(component.Message, "model family") {
			t.Errorf("model fallback=%#v", component)
		}
	}
}

func TestDeepSeekRolesFollowLatestIncidentUpdate(t *testing.T) {
	for _, newestState := range []string{"resolved", "investigating"} {
		var feed deepSeekRSS
		if err := xml.Unmarshal(deepSeekRoleRSS(t, "DeepSeek V4 Flash API", "API unavailable", "investigating"), &feed); err != nil {
			t.Fatal(err)
		}
		old := feed.Channel.Items[0]
		old.GUID = "old-change"
		newest := old
		newest.GUID = "new-change"
		newest.PubDate = "Thu, 28 May 2026 20:24:51 +0800"
		newest.Description = "<strong>Status:</strong> " + newestState + "<strong>Affected components:</strong> DeepSeek V900 Flash API"
		feed.Channel.Items = []deepSeekRSSItem{newest, old}
		payload, err := xml.Marshal(feed)
		if err != nil {
			t.Fatal(err)
		}
		got := parseDeepSeekRSS(payload, config.MonitorConfig{Components: []string{"Pro API", "Flash API"}}, deepSeekTestResult(), deepSeekTestNow())
		wantFlash := status.Outage
		if newestState == "resolved" {
			wantFlash = status.Normal
		}
		if got.Err != nil || got.ComponentStatuses[0].Status != status.Normal || got.ComponentStatuses[1].Status != wantFlash {
			t.Errorf("latest %s: %#v", newestState, got)
		}
	}
}

func TestDeepSeekKnownOutageSurvivesUnknownLabels(t *testing.T) {
	got := parseDeepSeekRSS(deepSeekRoleRSS(t, "Pro API, Future Service", "API unavailable", "investigating"), config.MonitorConfig{Components: []string{"Pro API", "Flash API"}}, deepSeekTestResult(), deepSeekTestNow())
	if got.Status != status.Outage || got.ComponentStatuses[0].Status != status.Outage || got.ComponentStatuses[1].Status != status.Unknown || got.Err == nil || !strings.Contains(got.Err.Error(), "Future Service") {
		t.Fatalf("known outage or unknown-label evidence lost: %#v", got)
	}
	page := parseDeepSeekRSS(deepSeekRoleRSS(t, "File Upload Service, Search Service", "Service unavailable", "identified"), config.MonitorConfig{}, deepSeekTestResult(), deepSeekTestNow())
	if page.Status != status.Outage || page.Err != nil {
		t.Fatalf("page-level auxiliary service outage lost: %#v", page)
	}
}

func TestDeepSeekExplicitRoleOverridesGeneralAPILabel(t *testing.T) {
	got := parseDeepSeekRSS(deepSeekRoleRSS(t, "API Service, Pro API", "API degraded performance", "identified"), config.MonitorConfig{Components: []string{"Pro API"}}, deepSeekTestResult(), deepSeekTestNow())
	if got.Status != status.Degraded || got.Err != nil {
		t.Fatalf("explicit Pro label must retain certainty: %#v", got)
	}
}
