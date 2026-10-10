package app

import (
	"context"
	"isllmalive/internal/config"
	"isllmalive/internal/providers"
	"isllmalive/internal/status"
	"sync/atomic"
	"testing"
	"time"
)

func testApp(t *testing.T) *App {
	t.Helper()
	a := New(&config.Config{Monitors: []config.MonitorConfig{{Type: "apiget", Name: "test", Endpoint: "https://old.test", Enabled: true}}})
	a.render = func([]providers.MonitorResult, string) {}
	a.settings = func(*config.Config) {}
	t.Cleanup(a.Stop)
	return a
}
func awaitSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}
}
func TestManualRefreshMergesAndDoesNotBlock(t *testing.T) {
	a := testApp(t)
	entered := make(chan struct{}, 4)
	release := make(chan struct{}, 4)
	var calls atomic.Int32
	a.fetch = func(ctx context.Context, m config.MonitorConfig) providers.MonitorResult {
		calls.Add(1)
		entered <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
		}
		return providers.MonitorResult{Status: status.Normal}
	}
	a.Start()
	awaitSignal(t, entered)
	returned := make(chan struct{})
	go func() {
		for range 50 {
			a.PollAll()
		}
		close(returned)
	}()
	awaitSignal(t, returned)
	// Give the coordinator time to consume the coalesced signal while the fetch is gated.
	time.Sleep(20 * time.Millisecond)
	release <- struct{}{}
	awaitSignal(t, entered)
	release <- struct{}{}
	time.Sleep(30 * time.Millisecond)
	if got := calls.Load(); got != 2 {
		t.Fatalf("got %d rounds, want 2", got)
	}
	a.Stop()
	a.Stop()
}
func TestReloadCancelsAndRejectsLateResults(t *testing.T) {
	a := testApp(t)
	oldEntered := make(chan struct{})
	oldCancelled := make(chan struct{})
	allowLate := make(chan struct{})
	newRendered := make(chan struct{}, 1)
	a.fetch = func(ctx context.Context, m config.MonitorConfig) providers.MonitorResult {
		if m.Endpoint == "https://old.test" {
			close(oldEntered)
			<-ctx.Done()
			close(oldCancelled)
			<-allowLate
			return providers.MonitorResult{Status: status.Outage}
		}
		return providers.MonitorResult{Status: status.Normal}
	}
	var stale atomic.Bool
	a.render = func(results []providers.MonitorResult, _ string) {
		for _, r := range results {
			if r.Status == status.Outage {
				stale.Store(true)
			}
			if r.Endpoint == "https://new.test" {
				select {
				case newRendered <- struct{}{}:
				default:
				}
			}
		}
	}
	a.Start()
	awaitSignal(t, oldEntered)
	cfg := cloneConfig(a.cfg)
	cfg.Monitors[0].Endpoint = "https://new.test"
	a.applyConfig(cfg)
	awaitSignal(t, oldCancelled)
	close(allowLate)
	awaitSignal(t, newRendered)
	if stale.Load() {
		t.Fatal("old configuration rendered")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.lastStatus) != 1 || a.lastResults[0].Endpoint != "https://new.test" {
		t.Fatal("old state survived")
	}
}
func TestStopCancelsRequests(t *testing.T) {
	a := testApp(t)
	entered := make(chan struct{})
	cancelled := make(chan struct{})
	a.fetch = func(ctx context.Context, _ config.MonitorConfig) providers.MonitorResult {
		close(entered)
		<-ctx.Done()
		close(cancelled)
		return providers.MonitorResult{Err: ctx.Err()}
	}
	a.Start()
	awaitSignal(t, entered)
	stopped := make(chan struct{})
	go func() { a.Stop(); close(stopped) }()
	awaitSignal(t, cancelled)
	awaitSignal(t, stopped)
}
