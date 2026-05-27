package providers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"isllmalive/internal/config"
	"isllmalive/internal/status"
)

// ApiGetProvider implements a reusable Provider that directly probes a core API endpoint.
// It bypasses aggressive WAFs on status pages by directly checking if the API returns 500/503.
type ApiGetProvider struct{}

// Fetch fetches the current status from the API.
func (p *ApiGetProvider) Fetch(ctx context.Context, monitor config.MonitorConfig) MonitorResult {
	client := &http.Client{Timeout: 10 * time.Second}

	var lastErr error
	// Retry logic (3 attempts max)
	for attempt := 1; attempt <= 3; attempt++ {
		select {
		case <-ctx.Done():
			return MonitorResult{Status: status.Unknown, Err: ctx.Err(), Name: monitor.Name}
		default:
		}

		result := p.attemptFetch(ctx, client, monitor)
		if result.Status != status.Unknown || result.Err == nil {
			return result
		}

		lastErr = result.Err

		if attempt < 3 {
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return MonitorResult{Status: status.Unknown, Err: ctx.Err(), Name: monitor.Name}
			}
		}
	}

	return MonitorResult{
		Status: status.Unknown,
		Err:    fmt.Errorf("failed after 3 attempts, last error: %w", lastErr),
		Name:   monitor.Name,
	}
}

// attemptFetch performs a single core API probe attempt.
func (p *ApiGetProvider) attemptFetch(ctx context.Context, client *http.Client, monitor config.MonitorConfig) MonitorResult {
	url := strings.TrimRight(monitor.Endpoint, "/") + "/v1/models"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return MonitorResult{Status: status.Unknown, Err: err, Name: monitor.Name}
	}
	
	req.Header.Set("User-Agent", "IsLLMAlive-Monitor/1.0")

	resp, err := client.Do(req)
	if err != nil {
		// Connection issues, DNS failures, etc.
		return MonitorResult{Status: status.Unknown, Err: err, Name: monitor.Name}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return MonitorResult{
			Status: status.Outage,
			Err:    fmt.Errorf("core API returned server error: %d", resp.StatusCode),
			Name:   monitor.Name,
		}
	}

	// 401 Unauthorized, 429 Too Many Requests, or 200 OK all indicate the API gateway is alive and responding perfectly.
	return MonitorResult{Status: status.Normal, Err: nil, Name: monitor.Name}
}
