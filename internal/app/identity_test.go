package app

import (
	"isllmalive/internal/config"
	"isllmalive/internal/providers"
	"isllmalive/internal/status"
	"testing"
)

func TestReconcileKeepsOnlyMatchingMonitors(t *testing.T) {
	first := config.MonitorConfig{Type: "apiget", Name: "same", Endpoint: "https://first.test", Enabled: true}
	second := first
	second.Endpoint = "https://second.test"
	a := New(&config.Config{Monitors: []config.MonitorConfig{first, second}})
	defer a.StopNotifications()
	keys := providers.MonitorKeys(a.cfg.Monitors)
	a.lastStatus[keys[0]] = status.Outage
	a.lastStatus[keys[1]] = status.Normal
	a.lastResults = []providers.MonitorResult{providers.CompleteResult(first, keys[0], providers.MonitorResult{Status: status.Outage}), providers.CompleteResult(second, keys[1], providers.MonitorResult{Status: status.Normal})}
	a.reconcileLocked(&config.Config{Monitors: []config.MonitorConfig{second, first}})
	if a.lastResults[0].Key != keys[1] || a.lastResults[1].Status != status.Outage {
		t.Fatal("state did not follow identity")
	}
	first.Endpoint = "https://new.test"
	a.reconcileLocked(&config.Config{Monitors: []config.MonitorConfig{first}})
	if len(a.lastStatus) != 0 || a.lastResults[0].Key != "" {
		t.Fatal("old source state survived")
	}
}
