package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

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

type GoogleProvider struct {
	cachedAPIKey string
}

func (p *GoogleProvider) fetchAPIKey(ctx context.Context, client *http.Client) (string, error) {
	if p.cachedAPIKey != "" {
		return p.cachedAPIKey, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://aistudio.google.com/status", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch status page, status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	html := string(body)
	matches := regexp.MustCompile(`"WIu0Nc"\s*:\s*"([^"]+)"`).FindStringSubmatch(html)
	if len(matches) > 1 {
		p.cachedAPIKey = matches[1]
		return p.cachedAPIKey, nil
	}
	
	// Fallback to first AIza key
	allKeys := regexp.MustCompile(`AIza[0-9A-Za-z-_]+`).FindAllString(html, -1)
	if len(allKeys) > 1 {
		p.cachedAPIKey = allKeys[1]
		return p.cachedAPIKey, nil
	} else if len(allKeys) > 0 {
		p.cachedAPIKey = allKeys[0]
		return p.cachedAPIKey, nil
	}

	return "", fmt.Errorf("API key not found in HTML")
}

func (p *GoogleProvider) Fetch(ctx context.Context, monitor config.MonitorConfig) MonitorResult {
	result := MonitorResult{
		Key:       monitor.Name,
		Name:      monitor.Name,
		Status:    status.Unknown,
		CheckedAt: time.Now(),
	}

	endpoint := "https://alkalimakersuite-pa.clients6.google.com/$rpc/google.internal.alkali.applications.makersuite.v1.MakerSuiteService/ListIncidentsHistory"

	// 3s local timeout per request to avoid blocking parent ctx for too long on stalls
	client := &http.Client{Timeout: 3 * time.Second}
	var lastErr error
	var lastMessage string

	// 1. First fetch the main page to get the API Key
	apiKey, err := p.fetchAPIKey(ctx, client)
	if err != nil {
		result.Message = "Failed to get API Key"
		result.Err = err
		return result
	}

	payload := []byte(`[]`)

	for attempt := 1; attempt <= 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(payload))
		if err != nil {
			result.Message = "Failed to create request"
			result.Err = err
			return result
		}
		req.Header.Set("Content-Type", "application/json+protobuf")
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")
		req.Header.Set("Origin", "https://aistudio.google.com")
		req.Header.Set("Referer", "https://aistudio.google.com/")
		req.Header.Set("Accept", "*/*")
		req.Header.Set("x-goog-api-key", apiKey)

		resp, err := client.Do(req)
		if err == nil {
			if resp.StatusCode == http.StatusOK {
				var rawData []interface{}
				err = json.NewDecoder(resp.Body).Decode(&rawData)
				resp.Body.Close()

				if err == nil {
					return p.parseData(rawData, monitor, result)
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

// parseData uses strict type assertions to navigate the deeply nested JSON array.
// It avoids panics and returns Unknown gracefully if the format unexpectedly changes.
func (p *GoogleProvider) parseData(rawData []interface{}, monitor config.MonitorConfig, result MonitorResult) MonitorResult {
	// Expected structure: [ [ [ [ incidents... ] ] ] ]
	if len(rawData) == 0 {
		return failSafe(result, "Empty root array")
	}
	level1, ok := rawData[0].([]interface{})
	if !ok || len(level1) == 0 {
		return failSafe(result, "Level 1 missing")
	}
	level2, ok := level1[0].([]interface{})
	if !ok || len(level2) == 0 {
		return failSafe(result, "Level 2 missing")
	}
	incidents, ok := level2[0].([]interface{})
	if !ok {
		return failSafe(result, "Incidents array missing")
	}

	targetCompID := float64(-1)
	if monitor.Component != "" && !strings.EqualFold(monitor.Component, "none") {
		targetCompID = mapGoogleComponent(monitor.Component)
		if targetCompID == -1 {
			result.Status = status.Unknown
			result.Message = "Unsupported component"
			result.Err = fmt.Errorf("unsupported google component: %s", monitor.Component)
			return result
		}
	}

	var activeIncidents []string
	maxSev := status.Normal

	for _, incRaw := range incidents {
		inc, ok := incRaw.([]interface{})
		if !ok || len(inc) < 4 {
			continue // Skip malformed incident
		}

		// incident[1] is title
		title := "Unknown Incident"
		if t, ok := inc[1].(string); ok {
			title = t
		}

		// incident[2] is severity (1=Degraded, 2=Outage)
		sevVal := float64(1)
		if s, ok := inc[2].(float64); ok {
			sevVal = s
		}

		// incident[3] is updates array
		updates, ok := inc[3].([]interface{})
		if !ok || len(updates) == 0 {
			continue
		}

		// Get last update
		lastUpdateRaw := updates[len(updates)-1]
		lastUpdate, ok := lastUpdateRaw.([]interface{})
		if !ok || len(lastUpdate) == 0 {
			continue
		}

		statusCode, ok := lastUpdate[0].(float64)
		if !ok {
			continue
		}

		// 4 = Resolved
		if statusCode == 4 {
			continue
		}

		// Active Incident! Now check component filtering if needed.
		if targetCompID != -1 && len(inc) > 5 {
			comps, ok := inc[5].([]interface{})
			if ok {
				found := false
				for _, c := range comps {
					if cFloat, ok := c.(float64); ok && cFloat == targetCompID {
						found = true
						break
					}
				}
				if !found {
					continue // This active incident doesn't affect the monitored component
				}
			}
		}

		// Map Severity
		var mappedSev status.Status
		if sevVal == 2 {
			mappedSev = status.Outage
		} else {
			mappedSev = status.Degraded
		}

		if mappedSev > maxSev {
			maxSev = mappedSev
		}
		activeIncidents = append(activeIncidents, title)
	}

	result.Status = maxSev
	if maxSev == status.Normal {
		result.Message = "operational"
	} else {
		result.Message = strings.Join(activeIncidents, ", ")
	}

	return result
}

func failSafe(result MonitorResult, reason string) MonitorResult {
	result.Status = status.Unknown
	result.Message = "Format error: " + reason
	result.Err = fmt.Errorf("google format changed: %s", reason)
	return result
}
