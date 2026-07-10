package tray

import (
	"strings"
	"testing"

	"isllmalive/internal/providers"
	"isllmalive/internal/status"
)

func TestMonitorTooltipListsOnlyAbnormalComponents(t *testing.T) {
	got := monitorTooltip(providers.MonitorResult{
		Name:       "Example",
		Type:       "statuspage",
		Components: []string{"API", "Web", "Missing"},
		ComponentStatuses: []providers.ComponentStatus{
			{Name: "API", Status: status.Normal, Message: "operational"},
			{Name: "Web", Status: status.Degraded, Message: "partial_outage"},
			{Name: "Missing", Status: status.Unknown, Message: "Component not found"},
		},
		Status: status.Degraded,
	}, "en-US")

	for _, want := range []string{"Components: API, Web, Missing", "Web: Degraded — partial_outage", "Missing: Unknown — Component not found"} {
		if !strings.Contains(got, want) {
			t.Fatalf("tooltip missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "API: Normal") {
		t.Fatalf("tooltip includes normal component:\n%s", got)
	}
}
