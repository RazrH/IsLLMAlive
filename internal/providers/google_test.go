package providers

import (
	"testing"

	"isllmalive/internal/config"
	"isllmalive/internal/status"
)

func TestGoogleMultipleComponentsUseWorstStatus(t *testing.T) {
	incidents := []interface{}{
		[]interface{}{nil, "API degraded", float64(1), []interface{}{[]interface{}{float64(1)}}, nil, []interface{}{float64(1)}},
		[]interface{}{nil, "Studio outage", float64(2), []interface{}{[]interface{}{float64(1)}}, nil, []interface{}{float64(3)}},
	}
	rawData := []interface{}{[]interface{}{[]interface{}{incidents}}}

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
