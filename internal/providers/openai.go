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

// OpenAIProvider implements the Provider interface for OpenAI's proxy API (Incident.io).
type OpenAIProvider struct{}

type oaiProxyResponse struct {
	Summary struct {
		Components []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"components"`
		AffectedComponents []struct {
			ComponentID string `json:"component_id"`
			Status      string `json:"status"`
		} `json:"affected_components"`
		OngoingIncidents []struct {
			Status string `json:"status"`
		} `json:"ongoing_incidents"`
	} `json:"summary"`
}

func (p *OpenAIProvider) Fetch(ctx context.Context, monitor config.MonitorConfig) MonitorResult {
	result := MonitorResult{
		Key:        monitor.Name,
		Name:       monitor.Name,
		Status:     status.Unknown,
		CheckedAt:  time.Now(),
		StatusPage: "https://status.openai.com",
	}

	endpoint := "https://status.openai.com/proxy/status.openai.com"
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

		req.Header.Set("User-Agent", "IsLLMAlive/1.0")
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err == nil {
			if resp.StatusCode == http.StatusOK {
				var proxyResp oaiProxyResponse
				err = json.NewDecoder(resp.Body).Decode(&proxyResp)
				resp.Body.Close()

				if err == nil {
					// Build a map of affected components
					affectedMap := make(map[string]string)
					for _, ac := range proxyResp.Summary.AffectedComponents {
						affectedMap[ac.ComponentID] = ac.Status
					}

					// Component level
					if monitor.Component != "" && !strings.EqualFold(monitor.Component, "none") {
						for _, comp := range proxyResp.Summary.Components {
							if strings.EqualFold(comp.Name, monitor.Component) {
								if compStatus, isAffected := affectedMap[comp.ID]; isAffected {
									result.Status = mapOAIStatus(compStatus)
									result.Message = compStatus
								} else {
									result.Status = status.Normal
									result.Message = "operational"
								}
								return result
							}
						}
						result.Message = "Component not found"
						result.Err = fmt.Errorf("component '%s' not found", monitor.Component)
						return result
					}

					// Page level
					if len(proxyResp.Summary.OngoingIncidents) > 0 {
						worstStatus := status.Normal
						worstStatusStr := "investigating"
						for _, ac := range proxyResp.Summary.AffectedComponents {
							s := mapOAIStatus(ac.Status)
							if s > worstStatus {
								worstStatus = s
								worstStatusStr = ac.Status
							}
						}

						if worstStatus == status.Normal {
							result.Status = status.Degraded
							result.Message = "incident ongoing"
						} else {
							result.Status = worstStatus
							result.Message = worstStatusStr
						}
						return result
					}

					if len(proxyResp.Summary.AffectedComponents) > 0 {
						worstStatus := status.Normal
						worstStatusStr := "affected"
						for _, ac := range proxyResp.Summary.AffectedComponents {
							s := mapOAIStatus(ac.Status)
							if s > worstStatus {
								worstStatus = s
								worstStatusStr = ac.Status
							}
						}
						result.Status = worstStatus
						result.Message = worstStatusStr
						return result
					}

					result.Status = status.Normal
					result.Message = "All Systems Operational"
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
			backoff := time.Duration(1<<attempt) * time.Second
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

func mapOAIStatus(oaiStatusStr string) status.Status {
	switch oaiStatusStr {
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
