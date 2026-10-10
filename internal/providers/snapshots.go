package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"isllmalive/internal/config"
	"isllmalive/internal/status"
	"net/http"
	"strings"
	"time"
)

type openAISnapshot struct{ data oaiProxyResponse }

func (s openAISnapshot) evaluate(m config.MonitorConfig, b MonitorResult) MonitorResult {
	return evaluateOpenAI(s.data, m, b)
}

type statuspageSnapshot struct{ data spSummary }

func (s statuspageSnapshot) evaluate(m config.MonitorConfig, b MonitorResult) MonitorResult {
	if names := m.ComponentNames(); len(names) > 0 {
		components := make([]ComponentStatus, 0, len(names))
		for _, name := range names {
			var matches []ComponentStatus
			for _, c := range s.data.Components {
				if strings.EqualFold(c.Name, name) {
					matches = append(matches, ComponentStatus{Name: name, Status: mapComponentStatus(c.Status), Message: c.Status})
				}
			}
			if len(matches) == 0 {
				matches = append(matches, ComponentStatus{Name: name, Status: status.Unknown, Message: "Component not found"})
			}
			result := aggregateComponentStatuses(MonitorResult{}, matches)
			components = append(components, ComponentStatus{Name: name, Status: result.Status, Message: result.Message})
		}
		return aggregateComponentStatuses(b, components)
	}
	b.Status = mapIndicator(s.data.Status.Indicator)
	b.Message = s.data.Status.Description
	if b.Status == status.Unknown {
		return incomplete(b, fmt.Errorf("missing or unknown page indicator: %q", s.data.Status.Indicator))
	}
	return b
}

type probeSnapshot struct{ code int }

func (s probeSnapshot) evaluate(_ config.MonitorConfig, b MonitorResult) MonitorResult {
	b.Status = status.Normal
	b.Message = fmt.Sprintf("Probe returned HTTP %d", s.code)
	if s.code >= 500 {
		b.Status = status.Outage
		b.Err = fmt.Errorf("core API returned server error: %d", s.code)
	}
	return b
}
func retry(ctx context.Context, fn func() error) error {
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err = fn(); err == nil {
			return nil
		}
		if errors.Is(err, ErrResponseTooLarge) {
			return err
		}
		if attempt < 3 {
			timer := time.NewTimer(time.Duration(1<<attempt) * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return errors.Join(err, ctx.Err())
			case <-timer.C:
			}
		}
	}
	return err
}
func readSource(ctx context.Context, client *http.Client, s Source, headers http.Header, readBody bool) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, s.Method, s.URL, strings.NewReader(s.Body))
	if err != nil {
		return nil, 0, err
	}
	req.Header = headers.Clone()
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "IsLLMAlive/1.0")
	}
	resp, err := doRequest(client, req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if !readBody {
		return nil, resp.StatusCode, nil
	}
	body, err := readResponse(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode != http.StatusOK {
		return body, resp.StatusCode, fmt.Errorf("HTTP %d from status source", resp.StatusCode)
	}
	return body, resp.StatusCode, err
}
func (e *Engine) fetchSnapshot(ctx context.Context, s Source, probes []string) (snapshot, error) {
	client := e.standardClient()
	if s.Type == "deepseek" {
		return e.fetchDeepSeekSnapshot(ctx, s, probes)
	}
	if s.Type == "google" {
		return e.fetchGoogleSnapshot(ctx, client, s)
	}
	var data []byte
	var code int
	err := retry(ctx, func() error {
		var err error
		data, code, err = readSource(ctx, client, s, nil, s.Type != "apiget")
		return err
	})
	if err != nil {
		return nil, err
	}
	switch s.Type {
	case "openai":
		var value oaiProxyResponse
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, err
		}
		return openAISnapshot{value}, nil
	case "statuspage":
		var value spSummary
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, err
		}
		return statuspageSnapshot{value}, nil
	case "apiget":
		return probeSnapshot{code}, nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", s.Type)
	}
}

func (e *Engine) fetchGoogleSnapshot(ctx context.Context, client *http.Client, s Source) (snapshot, error) {
	key, err := e.keys.get(ctx, func() (string, error) { return (&GoogleProvider{}).fetchAPIKey(ctx, client) })
	if err != nil {
		return nil, err
	}
	var data []byte
	refreshed := false
	err = retry(ctx, func() error {
		headers := http.Header{"Content-Type": {"application/json+protobuf"}, "X-Goog-Api-Key": {key}, "Origin": {"https://aistudio.google.com"}, "Referer": {"https://aistudio.google.com/"}}
		var code int
		var err error
		data, code, err = readSource(ctx, client, s, headers, true)
		if !refreshed && (code == http.StatusUnauthorized || code == http.StatusForbidden || (code == http.StatusBadRequest && (bytes.Contains(data, []byte("API_KEY_INVALID")) || bytes.Contains(data, []byte("API key not valid"))))) {
			refreshed = true
			e.keys.invalidate()
			key, err = e.keys.get(ctx, func() (string, error) { return (&GoogleProvider{}).fetchAPIKey(ctx, client) })
			if err != nil {
				return err
			}
			headers.Set("X-Goog-Api-Key", key)
			data, _, err = readSource(ctx, client, s, headers, true)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	var raw []any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return decodeGoogleSnapshot(raw)
}
