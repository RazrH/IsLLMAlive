package providers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"isllmalive/internal/config"
	"isllmalive/internal/status"
)

// ApiGetProvider implements a reusable Provider that directly probes a core API endpoint.
// It bypasses aggressive WAFs on status pages by directly checking if the API returns 500/503.
type ApiGetProvider struct{}

// Fetch fetches the current status from the API.
func (p *ApiGetProvider) Fetch(ctx context.Context, monitor config.MonitorConfig) MonitorResult {
	monitor.Type = "apiget"
	return fetchStandalone(ctx, monitor)
}

func (p *ApiGetProvider) attemptFetch(ctx context.Context, client *http.Client, monitor config.MonitorConfig) MonitorResult {
	probeURL := apiGetProbeURL(monitor.Endpoint)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
	if err != nil {
		return MonitorResult{Status: status.Unknown, Message: "Failed to create request for " + probeURL, Err: err, Name: monitor.Name}
	}

	req.Header.Set("User-Agent", "IsLLMAlive-Monitor/1.0")

	resp, err := doRequest(client, req)
	if err != nil {
		// Connection issues, DNS failures, etc.
		return MonitorResult{Status: status.Unknown, Message: "Network error probing " + probeURL, Err: err, Name: monitor.Name}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return MonitorResult{
			Status:  status.Outage,
			Message: fmt.Sprintf("Server error probing %s", probeURL),
			Err:     fmt.Errorf("core API returned server error: %d", resp.StatusCode),
			Name:    monitor.Name,
		}
	}

	// 401 Unauthorized, 429 Too Many Requests, or 200 OK all indicate the API gateway is alive and responding perfectly.
	return MonitorResult{Status: status.Normal, Message: fmt.Sprintf("Probe returned HTTP %d", resp.StatusCode), Err: nil, Name: monitor.Name}
}

func apiGetProbeURL(endpoint string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	parsed, err := url.Parse(trimmed)
	if err == nil && parsed.Path != "" && parsed.Path != "/" {
		return trimmed
	}
	return trimmed + "/v1/models"
}
