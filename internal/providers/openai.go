package providers

import (
	"context"
	"strings"

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
	monitor.Type = "openai"
	return fetchStandalone(ctx, monitor)
}

func resolveOAIComponentStatuses(proxyResp oaiProxyResponse, selections []string, affectedMap map[string]string) ([]ComponentStatus, []string) {
	byName := make(map[string][]oaiComponent)
	for _, component := range proxyResp.Summary.Components {
		key := strings.ToLower(strings.TrimSpace(component.Name))
		byName[key] = append(byName[key], component)
	}

	byQualifiedName := make(map[string][]oaiComponent)
	byID := make(map[string]oaiComponent, len(proxyResp.Summary.Components))
	for _, component := range proxyResp.Summary.Components {
		byID[component.ID] = component
	}
	groupByID := make(map[string]string)
	for _, item := range proxyResp.Summary.Structure.Items {
		for _, component := range item.Group.Components {
			canonical, valid := byID[component.ComponentID]
			if !valid || canonical.Name != component.Name || strings.TrimSpace(item.Group.Name) == "" {
				continue
			}
			qualifiedName := normalizeOAIComponentName(item.Group.Name + "/" + component.Name)
			byQualifiedName[qualifiedName] = append(byQualifiedName[qualifiedName], canonical)
			groupByID[component.ComponentID] = item.Group.Name
		}
	}

	componentStatuses := make([]ComponentStatus, 0, len(selections))
	var missing []string
	for _, selection := range selections {
		matches := byName[strings.ToLower(strings.TrimSpace(selection))]
		if len(matches) == 0 {
			if component, found := byQualifiedName[normalizeOAIComponentName(selection)]; found {
				matches = component
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
	group, component, qualified := strings.Cut(name, "/")
	if qualified {
		return strings.ToLower(strings.TrimSpace(group) + "/" + strings.TrimSpace(component))
	}
	return strings.ToLower(strings.TrimSpace(name))
}

func evaluateOAIComponents(selection string, components []oaiComponent, affectedMap map[string]string, groupByID map[string]string) ComponentStatus {
	worstStatus := status.Unknown
	hasUnknown := false
	var abnormal []string
	message := "operational"
	for i, component := range components {
		componentStatus := status.Normal
		componentMessage := "operational"
		if rawStatus, affected := affectedMap[component.ID]; affected {
			componentStatus = mapOAIStatus(rawStatus)
			componentMessage = rawStatus
		}
		hasUnknown = hasUnknown || componentStatus == status.Unknown
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
	if hasUnknown && worstStatus == status.Normal {
		worstStatus = status.Unknown
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
