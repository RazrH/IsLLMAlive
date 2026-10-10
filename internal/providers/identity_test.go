package providers

import (
	"context"
	"isllmalive/internal/config"
	"testing"
)

func TestMonitorIdentity(t *testing.T) {
	a := config.MonitorConfig{Type: "statuspage", Name: "same", Endpoint: "https://example.test", Components: []string{" API ", "Web"}}
	b := a
	b.Endpoint = "https://other.test"
	keys := MonitorKeys([]config.MonitorConfig{a, b, a})
	if keys[0] == keys[1] || keys[0] == keys[2] {
		t.Fatal("distinct monitors collided")
	}
	reordered := MonitorKeys([]config.MonitorConfig{b, a, a})
	if reordered[0] != keys[1] || reordered[1] != keys[0] || reordered[2] != keys[2] {
		t.Fatal("reordering changed identity")
	}
	changed := a
	changed.NotifyOn = true
	changed.StatusPage = "https://custom.test"
	changed.Components = []string{"web", "api", "API"}
	if MonitorKeys([]config.MonitorConfig{changed})[0] != keys[0] {
		t.Fatal("display/selection order changed identity")
	}
	changed.Components = []string{"API"}
	if MonitorKeys([]config.MonitorConfig{changed})[0] == keys[0] {
		t.Fatal("component change must reset identity")
	}
	if StateSource(a) != StateSource(changed) {
		t.Fatal("components must not change source")
	}
}
func TestAPIGetCancellationHasMetadata(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, name := range []string{"first", "second"} {
		m := config.MonitorConfig{Name: name, Type: "apiget", Endpoint: "https://example.test", StatusPage: "https://status.test"}
		r := (&ApiGetProvider{}).Fetch(ctx, m)
		if r.Key == "" || r.Name != name || r.CheckedAt.IsZero() || r.StatusPage != m.StatusPage || r.Err == nil {
			t.Fatalf("incomplete: %#v", r)
		}
	}
}
