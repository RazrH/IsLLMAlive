package providers

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"isllmalive/internal/config"
	"isllmalive/internal/status"
)

// mapGoogleComponent returns the internal ID for known Google AI Studio components.
func mapGoogleComponent(name string) float64 {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "api":
		return 1
	case "multimodal live api":
		return 2
	case "google ai studio":
		return 3
	}
	return -1 // Unknown component
}

var googleStatusURL = "https://aistudio.google.com/status"

type GoogleProvider struct{}

var googleKeyPattern = regexp.MustCompile(`"WIu0Nc"\s*:\s*"([^"]+)"`)
var googleFallbackKeyPattern = regexp.MustCompile(`AIza[0-9A-Za-z-_]+`)

func (p *GoogleProvider) fetchAPIKey(ctx context.Context, client *http.Client) (string, error) {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleStatusURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")

	resp, err := doRequest(client, req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch status page, status: %d", resp.StatusCode)
	}

	body, err := readResponse(resp.Body)
	if err != nil {
		return "", err
	}

	html := string(body)
	matches := googleKeyPattern.FindStringSubmatch(html)
	if len(matches) > 1 {
		return strings.Clone(matches[1]), nil
	}

	// Fallback to first AIza key
	allKeys := googleFallbackKeyPattern.FindAllString(html, 2)
	if len(allKeys) > 1 {
		return strings.Clone(allKeys[1]), nil
	} else if len(allKeys) > 0 {
		return strings.Clone(allKeys[0]), nil
	}

	return "", fmt.Errorf("API key not found in HTML")
}

func (p *GoogleProvider) Fetch(ctx context.Context, monitor config.MonitorConfig) MonitorResult {
	monitor.Type = "google"
	return fetchStandalone(ctx, monitor)
}

func (p *GoogleProvider) parseData(raw []any, m config.MonitorConfig, b MonitorResult) MonitorResult {
	s, err := decodeGoogleSnapshot(raw)
	if err != nil {
		return incomplete(b, err)
	}
	return s.evaluate(m, b)
}

func parseGoogleIncident(raw any) (status.Status, string, []float64, bool, error) {
	bad := func(reason string) (status.Status, string, []float64, bool, error) {
		return status.Unknown, "", nil, false, fmt.Errorf("%s", reason)
	}
	inc, ok := raw.([]any)
	if !ok || len(inc) < 4 {
		return bad("invalid incident record")
	}
	title, ok := inc[1].(string)
	if !ok || strings.TrimSpace(title) == "" {
		return bad("title missing")
	}
	severity, ok := inc[2].(float64)
	if !ok || (severity != 1 && severity != 2) {
		return bad("unknown severity")
	}
	updates, ok := inc[3].([]any)
	if !ok || len(updates) == 0 {
		return bad("updates missing")
	}
	update, ok := updates[len(updates)-1].([]any)
	if !ok || len(update) == 0 {
		return bad("invalid last update")
	}
	code, ok := update[0].(float64)
	if !ok || (code != 1 && code != 2 && code != 3 && code != 4) {
		return bad("unknown incident state")
	}
	var components []float64
	if len(inc) > 5 {
		values, ok := inc[5].([]any)
		if !ok {
			return bad("invalid components")
		}
		components = make([]float64, 0, len(values))
		for _, v := range values {
			c, ok := v.(float64)
			if !ok || c < 1 || c != float64(int64(c)) {
				return bad("invalid component identity")
			}
			components = append(components, c)
		}
	}
	mapped := status.Degraded
	if severity == 2 {
		mapped = status.Outage
	}
	return mapped, title, components, code == 4, nil
}

func failSafe(result MonitorResult, reason string) MonitorResult {
	result.Status = status.Unknown
	result.Message = "Format error: " + reason
	result.Err = fmt.Errorf("google format changed: %s", reason)
	return result
}
