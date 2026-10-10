package providers

import (
	"testing"

	"isllmalive/internal/config"
	"isllmalive/internal/status"
)

func TestGoogleMultipleComponentsUseWorstStatus(t *testing.T) {
	incidents := []any{
		[]any{nil, "API degraded", float64(1), []any{[]any{float64(1)}}, nil, []any{float64(1)}},
		[]any{nil, "Studio outage", float64(2), []any{[]any{float64(1)}}, nil, []any{float64(3)}},
	}
	rawData := []any{[]any{incidents}}

	got := (&GoogleProvider{}).parseData(rawData, config.MonitorConfig{
		Components: []string{"api", "google ai studio", "unsupported"},
	}, MonitorResult{})

	if got.Status != status.Outage {
		t.Fatalf("Status = %s, want Outage", got.Status)
	}
	if len(got.ComponentStatuses) != 3 || got.ComponentStatuses[0].Status != status.Degraded || got.ComponentStatuses[2].Status != status.Unknown {
		t.Fatalf("ComponentStatuses = %#v", got.ComponentStatuses)
	}
	if got.Err == nil {
		t.Fatal("Err is nil, want unsupported component error")
	}
}
