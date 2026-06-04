package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"isllmalive/internal/config"
	"isllmalive/internal/providers"
	"isllmalive/internal/status"
)

func TestDiagnosticsIncludesProviderFailureDetails(t *testing.T) {
	a := New(&config.Config{
		Language:               "en-US",
		RefreshIntervalMinutes: 10,
		GlobalNotifyOn:         true,
		Monitors: []config.MonitorConfig{
			{
				Type:       "deepseek",
				Name:       "DeepSeek",
				Enabled:    true,
				Endpoint:   "https://status.deepseek.com/feed.rss",
				Component:  "API Service",
				StatusPage: "https://status.deepseek.com",
				NotifyOn:   true,
			},
		},
	})
	a.lastResults = []providers.MonitorResult{
		{
			Name:       "DeepSeek",
			Type:       "deepseek",
			Endpoint:   "https://status.deepseek.com/feed.rss",
			Component:  "API Service",
			Status:     status.Unknown,
			Message:    "Network error fetching https://status.deepseek.com/feed.rss",
			CheckedAt:  time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC),
			StatusPage: "https://status.deepseek.com",
			Err:        errors.New("tls handshake timeout"),
		},
	}

	got := a.Diagnostics()

	for _, want := range []string{
		"config.type: deepseek",
		"config.endpoint: https://status.deepseek.com/feed.rss",
		"result.status: Unknown",
		"result.message: Network error fetching https://status.deepseek.com/feed.rss",
		"result.error: tls handshake timeout",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("Diagnostics() missing %q in:\n%s", want, got)
		}
	}
}
