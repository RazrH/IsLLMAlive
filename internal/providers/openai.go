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

var openAIEndpoint = "https://status.openai.com/proxy/status.openai.com"

type oaiComponent struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type oaiProxyResponse struct {
	Summary struct {
		Components         []oaiComponent `json:"components"`
		AffectedComponents []struct {
			ComponentID string `json:"component_id"`
			Status      string `json:"status"`
		} `json:"affected_components"`
		OngoingIncidents []struct {
			Status string `json:"status"`
		} `json:"ongoing_incidents"`
		Structure struct {
			Items []struct {
				Group struct {
					Name       string `json:"name"`
					Components []struct {
						ComponentID string `json:"component_id"`
						Name        string `json:"name"`
					} `json:"components"`
				} `json:"group"`
			} `json:"items"`
		} `json:"structure"`
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

	endpoint := openAIEndpoint
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
					if componentNames := monitor.ComponentNames(); len(componentNames) > 0 {
						componentStatuses, missing := resolveOAIComponentStatuses(proxyResp, componentNames, affectedMap)
						result = aggregateComponentStatuses(result, componentStatuses)
						if len(missing) > 0 {
							result.Err = fmt.Errorf("components not found: %s", strings.Join(missing, ", "))
						}
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

func resolveOAIComponentStatuses(proxyResp oaiProxyResponse, selections []string, affectedMap map[string]string) ([]ComponentStatus, []string) {
	byName := make(map[string][]oaiComponent)
	for _, component := range proxyResp.Summary.Components {
		key := strings.ToLower(strings.TrimSpace(component.Name))
		byName[key] = append(byName[key], component)
	}

	byQualifiedName := make(map[string]oaiComponent)
	groupByID := make(map[string]string)
	for _, item := range proxyResp.Summary.Structure.Items {
		for _, component := range item.Group.Components {
			qualifiedName := item.Group.Name + "/" + component.Name
			byQualifiedName[normalizeOAIComponentName(qualifiedName)] = oaiComponent{ID: component.ComponentID, Name: component.Name}
			groupByID[component.ComponentID] = item.Group.Name
		}
	}

	componentStatuses := make([]ComponentStatus, 0, len(selections))
	var missing []string
	for _, selection := range selections {
		matches := byName[strings.ToLower(strings.TrimSpace(selection))]
		if len(matches) == 0 {
			if component, found := byQualifiedName[normalizeOAIComponentName(selection)]; found {
				matches = []oaiComponent{component}
			}
		}
		if len(matches) == 0 {
			componentStatuses = append(componentStatuses, ComponentStatus{Name: selection, Status: status.Unknown, Message: "Component not found"})
			missing = append(missing, selection)
			continue
		}
		componentStatuses = append(componentStatuses, evaluateOAIComponents(selection, matches, affectedMap, groupByID))
	}
	return componentStatuses, missing
}

func normalizeOAIComponentName(name string) string {
	parts := strings.SplitN(name, "/", 2)
	if len(parts) == 2 {
		return strings.ToLower(strings.TrimSpace(parts[0]) + "/" + strings.TrimSpace(parts[1]))
	}
	return strings.ToLower(strings.TrimSpace(name))
}

func evaluateOAIComponents(selection string, components []oaiComponent, affectedMap map[string]string, groupByID map[string]string) ComponentStatus {
	worstStatus := status.Unknown
	var abnormal []string
	message := "operational"
	for i, component := range components {
		componentStatus := status.Normal
		componentMessage := "operational"
		if rawStatus, affected := affectedMap[component.ID]; affected {
			componentStatus = mapOAIStatus(rawStatus)
			componentMessage = rawStatus
		}
		if i == 0 || componentStatus > worstStatus {
			worstStatus = componentStatus
			message = componentMessage
		}
		if len(components) > 1 && componentStatus != status.Normal {
			name := component.Name
			if group := groupByID[component.ID]; group != "" {
				name = group + "/" + component.Name
			}
			abnormal = append(abnormal, name+": "+componentMessage)
		}
	}
	if len(abnormal) > 0 {
		message = strings.Join(abnormal, "; ")
	}
	return ComponentStatus{Name: selection, Status: worstStatus, Message: message}
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
