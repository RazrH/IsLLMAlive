package providers

import (
	"testing"

	"isllmalive/internal/status"
)

func TestAggregateComponentStatusesUsesExistingSeverityOrder(t *testing.T) {
	tests := []struct {
		name       string
		components []ComponentStatus
		want       status.Status
	}{
		{"all normal", []ComponentStatus{{Status: status.Normal}, {Status: status.Normal}}, status.Normal},
		{"unknown below normal", []ComponentStatus{{Status: status.Unknown}, {Status: status.Normal}}, status.Normal},
		{"degraded below outage", []ComponentStatus{{Status: status.Degraded}, {Status: status.Outage}}, status.Outage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := aggregateComponentStatuses(MonitorResult{}, tt.components).Status; got != tt.want {
				t.Fatalf("Status = %s, want %s", got, tt.want)
			}
		})
	}
}
