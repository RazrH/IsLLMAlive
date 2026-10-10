package providers

import (
	"errors"
	"fmt"
	"isllmalive/internal/config"
	"isllmalive/internal/status"
	"slices"
	"strings"
)

type googleIncident struct {
	severity   status.Status
	title      string
	components []float64
	resolved   bool
}
type googleSnapshot struct {
	incidents []googleIncident
	invalid   error
}

func decodeGoogleSnapshot(raw []any) (snapshot, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("Google root missing")
	}
	first, ok := raw[0].([]any)
	if !ok || len(first) == 0 {
		return nil, fmt.Errorf("Google level 1 missing")
	}
	records, ok := first[0].([]any)
	if !ok {
		return nil, fmt.Errorf("Google incidents array missing")
	}
	s := googleSnapshot{incidents: make([]googleIncident, 0, len(records))}
	var issues problems
	for i, raw := range records {
		severity, title, components, resolved, err := parseGoogleIncident(raw)
		if err != nil {
			issues.add(fmt.Errorf("incident %d: %w", i, err))
			continue
		}
		s.incidents = append(s.incidents, googleIncident{severity, title, components, resolved})
	}
	s.invalid = issues.err()
	return s, nil
}
func (s googleSnapshot) evaluate(m config.MonitorConfig, b MonitorResult) MonitorResult {
	if names := m.ComponentNames(); len(names) > 0 {
		components := make([]ComponentStatus, 0, len(names))
		for _, name := range names {
			id := mapGoogleComponent(name)
			r := MonitorResult{Status: status.Unknown}
			if id < 0 {
				r.Err = fmt.Errorf("unsupported Google component %q", name)
				r.Message = r.Err.Error()
			} else {
				r = s.forComponent(id)
			}
			b.Err = errors.Join(b.Err, r.Err)
			components = append(components, ComponentStatus{Name: name, Status: r.Status, Message: r.Message})
		}
		return aggregateComponentStatuses(b, components)
	}
	result := s.forComponent(-1)
	b.Status = result.Status
	b.Message = result.Message
	b.Err = result.Err
	return b
}
func (s googleSnapshot) forComponent(id float64) MonitorResult {
	r := MonitorResult{Status: status.Normal, Message: "operational"}
	var issues problems
	issues.add(s.invalid)
	var titles []string
	for _, inc := range s.incidents {
		if inc.resolved {
			continue
		}
		if id != -1 {
			if inc.components == nil {
				issues.add(fmt.Errorf("incident components missing"))
				continue
			}
			found := slices.Contains(inc.components, id)
			if !found {
				continue
			}
		}
		if inc.severity > r.Status {
			r.Status = inc.severity
		}
		titles = append(titles, inc.title)
	}
	if len(titles) > 0 {
		r.Message = strings.Join(titles, ", ")
	}
	return incomplete(r, issues.err())
}
