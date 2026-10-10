package providers

import (
	"context"

	"isllmalive/internal/config"
	"isllmalive/internal/status"
)

// StatuspageProvider implements the Provider interface for Atlassian Statuspage.
type StatuspageProvider struct{}

// JSON structures for Atlassian Statuspage /api/v2/summary.json
type spSummary struct {
	Status     spStatus      `json:"status"`
	Components []spComponent `json:"components"`
}

type spStatus struct {
	Indicator   string `json:"indicator"`
	Description string `json:"description"`
}

type spComponent struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

func (p *StatuspageProvider) Fetch(ctx context.Context, monitor config.MonitorConfig) MonitorResult {
	monitor.Type = "statuspage"
	return fetchStandalone(ctx, monitor)
}

func mapComponentStatus(spStatusStr string) status.Status {
	switch spStatusStr {
	case "operational":
		return status.Normal
	case "degraded_performance", "partial_outage":
		return status.Degraded
	case "major_outage", "under_maintenance":
		return status.Outage
	default:
		return status.Unknown
	}
}

// mapIndicator maps Statuspage page indicator to our Status model
func mapIndicator(indicator string) status.Status {
	switch indicator {
	case "none":
		return status.Normal
	case "minor":
		return status.Degraded
	case "major", "critical", "maintenance":
		return status.Outage
	default:
		return status.Unknown
	}
}
