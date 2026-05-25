package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

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
	result := MonitorResult{
		Key:       monitor.Name,
		Name:      monitor.Name,
		Status:    status.Unknown,
		CheckedAt: time.Now(),
	}

	endpoint := strings.TrimSuffix(monitor.Endpoint, "/")
	if !strings.HasSuffix(endpoint, "/api/v2/summary.json") {
		endpoint += "/api/v2/summary.json"
	}

	// Set a short local timeout for each HTTP request (e.g., 3s)
	// so that a complete network stall triggers the retry loop
	// rather than blocking until the parent ctx (15s) times out.
	client := &http.Client{Timeout: 3 * time.Second}
	var lastErr error
	var lastMessage string

	for attempt := 1; attempt <= 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			result.Message = "Failed to create request"
			result.Err = err
			return result
		}

		resp, err := client.Do(req)
		if err == nil {
			if resp.StatusCode == http.StatusOK {
				var summary spSummary
				err = json.NewDecoder(resp.Body).Decode(&summary)
				resp.Body.Close()

				if err == nil {
					// Component level
					if monitor.Component != "" {
						for _, comp := range summary.Components {
							if strings.EqualFold(comp.Name, monitor.Component) {
								result.Status = mapComponentStatus(comp.Status)
								result.Message = comp.Status
								return result
							}
						}
						result.Message = "Component not found"
						result.Err = fmt.Errorf("component '%s' not found", monitor.Component)
						return result
					}

					// Page level
					result.Status = mapIndicator(summary.Status.Indicator)
					result.Message = summary.Status.Description
					return result
				} else {
					lastMessage = "Parse error"
					lastErr = err
				}
			} else {
				lastMessage = fmt.Sprintf("HTTP %d", resp.StatusCode)
				lastErr = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
				resp.Body.Close()
			}
		} else {
			lastMessage = "Network error"
			lastErr = err
		}

		if attempt < 3 {
			backoff := time.Duration(1<<attempt) * time.Second // 2s, then 4s
			select {
			case <-ctx.Done():
				result.Message = "Context cancelled"
				result.Err = ctx.Err()
				return result
			case <-time.After(backoff):
			}
		}
	}

	result.Message = lastMessage
	result.Err = lastErr
	return result
}

// mapComponentStatus maps Statuspage component status to our Status model
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
