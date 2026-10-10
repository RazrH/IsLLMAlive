package providers

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"isllmalive/internal/config"
	"isllmalive/internal/status"
	"strings"
	"time"
)

type deepSeekSnapshot struct {
	incidents              map[string]deepSeekIncident
	feedItems, parsedItems int
	latestPublished        time.Time
	invalid                error
}

func decodeDeepSeekSnapshot(data []byte, now time.Time) (snapshot, error) {
	var feed deepSeekRSS
	decoder := xml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&feed); err != nil {
		return nil, err
	}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch token := token.(type) {
		case xml.Comment:
		case xml.CharData:
			if strings.TrimSpace(string(token)) != "" {
				return nil, fmt.Errorf("unexpected trailing XML text")
			}
		default:
			return nil, fmt.Errorf("unexpected XML after RSS root")
		}
	}
	if feed.Channel == nil {
		return nil, fmt.Errorf("RSS channel missing")
	}
	s := deepSeekSnapshot{incidents: make(map[string]deepSeekIncident), feedItems: len(feed.Channel.Items)}
	var issues problems
	for i, item := range feed.Channel.Items {
		incident, err := parseDeepSeekRSSItem(item)
		if err == nil && incident.Published.After(now.Add(5*time.Minute)) {
			err = fmt.Errorf("RSS item timestamp is in the future")
		}
		if err != nil {
			issues.add(fmt.Errorf("RSS item %d: %w", i, err))
			continue
		}
		s.parsedItems++
		if incident.Published.After(s.latestPublished) {
			s.latestPublished = incident.Published
		}
		key := incident.Link
		if key == "" {
			key = item.GUID
		}
		if key == "" {
			key = incident.Title
		}
		previous, found := s.incidents[key]
		if !found || incident.Published.After(previous.Published) {
			s.incidents[key] = incident
		}
	}
	s.invalid = issues.err()
	return s, nil
}
func (s deepSeekSnapshot) evaluate(monitor config.MonitorConfig, result MonitorResult) (out MonitorResult) {
	defer func() { out = incomplete(out, s.invalid) }()
	if componentNames := monitor.ComponentNames(); len(componentNames) > 0 {
		componentStatuses := make([]ComponentStatus, 0, len(componentNames))
		activeCounts := make([]int, 0, len(componentNames))
		var unsupported []string
		for _, name := range componentNames {
			target := normalizeDeepSeekComponent(name)
			if target == "" {
				componentStatuses = append(componentStatuses, ComponentStatus{Name: name, Status: status.Unknown, Message: "Unsupported component"})
				activeCounts = append(activeCounts, 0)
				unsupported = append(unsupported, name)
				continue
			}
			componentStatus, message, activeCount := evaluateDeepSeekIncidents(s.incidents, target)
			componentStatuses = append(componentStatuses, ComponentStatus{Name: name, Status: componentStatus, Message: message})
			activeCounts = append(activeCounts, activeCount)
		}
		result = aggregateComponentStatuses(result, componentStatuses)
		if len(componentStatuses) == 1 && len(unsupported) == 0 {
			result.Message = deepSeekRSSSummary(componentStatuses[0].Message, s.feedItems, s.parsedItems, activeCounts[0], s.latestPublished)
		}
		if len(unsupported) > 0 {
			result.Err = fmt.Errorf("unsupported deepseek components: %s", strings.Join(unsupported, ", "))
		}
		return result
	}

	result.Status, result.Message, _ = evaluateDeepSeekIncidents(s.incidents, "")
	if result.Status == status.Normal {
		result.Message = deepSeekRSSSummary("operational", s.feedItems, s.parsedItems, 0, s.latestPublished)
	} else {
		activeCount := 0
		for _, incident := range s.incidents {
			if !strings.EqualFold(strings.TrimSpace(incident.StatusText), "resolved") {
				activeCount++
			}
		}
		result.Message = deepSeekRSSSummary(result.Message, s.feedItems, s.parsedItems, activeCount, s.latestPublished)
	}
	return result
}

func (e *Engine) fetchDeepSeekSnapshot(ctx context.Context, source Source, targets []string) (snapshot, error) {
	client := e.standardClient()
	rssCtx, rssCancel := context.WithTimeout(ctx, 9*time.Second)
	defer rssCancel()
	var body []byte
	err := retry(rssCtx, func() error {
		var err error
		body, _, err = fetchDeepSeekRSS(rssCtx, client, source.URL, "standard")
		if err == nil {
			return nil
		}
		if errors.Is(err, ErrResponseTooLarge) {
			return err
		}
		body, _, err = fetchDeepSeekRSS(rssCtx, e.utlsClient(), source.URL, "utls-chrome")
		return err
	})
	rssCancel()
	if errors.Is(err, ErrResponseTooLarge) {
		return nil, err
	}
	if err == nil {
		return decodeDeepSeekSnapshot(body, time.Now())
	}
	probeCtx, probeCancel := context.WithTimeout(ctx, 6*time.Second)
	defer probeCancel()
	probes := make(map[string]deepSeekProbeResult, len(targets))
	for _, target := range targets {
		endpoint := deepSeekAPIProbeURL
		if target == "web_chat" {
			endpoint = deepSeekWebProbeURL
		}
		probes[target] = probeDeepSeekService(probeCtx, client, target, endpoint)
	}
	return deepSeekProbeSnapshot{probes, err}, nil
}

type deepSeekProbeSnapshot struct {
	probes map[string]deepSeekProbeResult
	rssErr error
}

func (s deepSeekProbeSnapshot) evaluate(m config.MonitorConfig, b MonitorResult) MonitorResult {
	names := m.ComponentNames()
	if len(names) == 0 {
		names = []string{"api", "web"}
	}
	components := make([]ComponentStatus, 0, len(names))
	for _, name := range names {
		target := normalizeDeepSeekComponent(name)
		probe, ok := s.probes[target]
		if !ok {
			components = append(components, ComponentStatus{Name: name, Status: status.Unknown, Message: "Unsupported or unavailable probe"})
			continue
		}
		components = append(components, ComponentStatus{Name: name, Status: probe.Status, Message: probe.Message})
		b.Err = errors.Join(b.Err, probe.Err)
	}
	b = aggregateComponentStatuses(b, components)
	b.Err = errors.Join(b.Err, fmt.Errorf("RSS fetch failed before direct probe fallback: %w", s.rssErr))
	b.Message = "RSS unavailable; direct probe fallback used; " + b.Message
	return b
}
