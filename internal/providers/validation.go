package providers

import (
	"errors"
	"fmt"
	"isllmalive/internal/config"
	"isllmalive/internal/status"
	"strings"
)

// incomplete preserves confirmed incidents without presenting uncertain data as healthy.
func incomplete(result MonitorResult, err error) MonitorResult {
	if err == nil {
		return result
	}
	result.Err = errors.Join(result.Err, err)
	if result.Status <= status.Normal {
		result.Status = status.Unknown
	}
	for i := range result.ComponentStatuses {
		if result.ComponentStatuses[i].Status == status.Normal {
			result.ComponentStatuses[i].Status = status.Unknown
			result.ComponentStatuses[i].Message = err.Error()
		}
	}
	result.Message += "; " + err.Error()
	return result
}

func evaluateOpenAI(data oaiProxyResponse, monitor config.MonitorConfig, result MonitorResult) MonitorResult {
	s := data.Summary
	selections := monitor.ComponentNames()
	if s.Components == nil || s.AffectedComponents == nil || (len(selections) == 0 && s.OngoingIncidents == nil) {
		return incomplete(result, fmt.Errorf("OpenAI summary missing required non-null arrays"))
	}
	affected := make(map[string]string, len(s.AffectedComponents))
	ids := make(map[string]bool, len(s.Components))
	var issues problems
	for _, c := range s.Components {
		if strings.TrimSpace(c.ID) == "" || strings.TrimSpace(c.Name) == "" || ids[c.ID] {
			issues.add(fmt.Errorf("invalid or duplicate component identity"))
		}
		ids[c.ID] = true
	}
	for _, c := range s.AffectedComponents {
		if c.ComponentID == "" || !ids[c.ComponentID] {
			issues.add(fmt.Errorf("affected component has no valid identity"))
		}
		if _, duplicate := affected[c.ComponentID]; duplicate {
			issues.add(fmt.Errorf("duplicate affected component"))
		}
		affected[c.ComponentID] = c.Status
	}
	if len(selections) > 0 {
		components, missing := resolveOAIComponentStatuses(data, selections, affected)
		result = aggregateComponentStatuses(result, components)
		if len(missing) > 0 {
			result.Err = errors.Join(result.Err, fmt.Errorf("components not found: %s", strings.Join(missing, ", ")))
		}
		return incomplete(result, issues.err())
	}
	parts := make([]ComponentStatus, 0, len(affected)+1)
	parts = append(parts, ComponentStatus{Name: "page", Status: status.Normal, Message: "operational"})
	for _, c := range s.AffectedComponents {
		parts = append(parts, ComponentStatus{Name: c.ComponentID, Status: mapOAIStatus(c.Status), Message: c.Status})
	}
	for _, incident := range s.OngoingIncidents {
		state := status.Unknown
		switch incident.Status {
		case "investigating", "identified", "monitoring":
			state = status.Degraded
		default:
			issues.add(fmt.Errorf("unknown ongoing incident status: %q", incident.Status))
		}
		parts = append(parts, ComponentStatus{Name: "incident", Status: state, Message: incident.Status})
	}
	result = aggregateComponentStatuses(result, parts)
	result.ComponentStatuses = nil
	return incomplete(result, issues.err())
}

// problems bounds diagnostics from untrusted lists instead of building a deeply nested error tree.
type problems struct {
	count int
	first error
}

func (p *problems) add(err error) {
	if err == nil {
		return
	}
	p.count++
	if p.first == nil {
		p.first = err
	}
}
func (p problems) err() error {
	if p.count == 0 {
		return nil
	}
	return fmt.Errorf("%d invalid records; first: %w", p.count, p.first)
}
