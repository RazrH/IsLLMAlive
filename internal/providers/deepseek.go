package providers

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"isllmalive/internal/config"
	"isllmalive/internal/status"

	utls "github.com/refraction-networking/utls"
)

const (
	deepSeekDefaultFeedURL    = "https://status.deepseek.com/feed.rss"
	deepSeekDefaultStatusPage = "https://status.deepseek.com"
)

var (
	deepSeekAPIProbeURL = "https://api.deepseek.com/v1/models"
	deepSeekWebProbeURL = "https://chat.deepseek.com"
)

// DeepSeekProvider implements DeepSeek status monitoring from its Flashduty RSS feed.
type DeepSeekProvider struct{}

type deepSeekRSS struct {
	Channel deepSeekRSSChannel `xml:"channel"`
}

type deepSeekRSSChannel struct {
	Items []deepSeekRSSItem `xml:"item"`
}

type deepSeekRSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	GUID        string `xml:"guid"`
	PubDate     string `xml:"pubDate"`
}

type deepSeekIncident struct {
	Title      string
	Link       string
	StatusText string
	Components []string
	Published  time.Time
	Severity   status.Status
}

func (p *DeepSeekProvider) Fetch(ctx context.Context, monitor config.MonitorConfig) MonitorResult {
	result := MonitorResult{
		Key:        monitor.Name,
		Name:       monitor.Name,
		Status:     status.Unknown,
		CheckedAt:  time.Now(),
		StatusPage: deepSeekDefaultStatusPage,
	}

	endpoint := strings.TrimSpace(monitor.Endpoint)
	if endpoint == "" {
		endpoint = deepSeekDefaultFeedURL
	}

	client := &http.Client{Timeout: 3 * time.Second}
	utlsClient := newDeepSeekUTLSHTTPClient(3 * time.Second)
	var lastErr error
	var lastMessage string

	for attempt := 1; attempt <= 3; attempt++ {
		body, message, err := fetchDeepSeekRSS(ctx, client, endpoint, "standard")
		if err == nil {
			return parseDeepSeekRSS(body, monitor, result, time.Now())
		}

		lastMessage = message
		lastErr = err

		utlsBody, utlsMessage, utlsErr := fetchDeepSeekRSS(ctx, utlsClient, endpoint, "utls-chrome")
		if utlsErr == nil {
			parsed := parseDeepSeekRSS(utlsBody, monitor, result, time.Now())
			if parsed.Err == nil {
				parsed.Message = "utls-chrome RSS fetch succeeded after standard fetch failed; " + parsed.Message
			} else {
				parsed.Message = "utls-chrome RSS fetch succeeded after standard fetch failed, but RSS parse failed; " + parsed.Message
			}
			return parsed
		}

		if lastErr != nil {
			lastMessage = fmt.Sprintf("%s; utls_fallback=%s", lastMessage, utlsMessage)
			lastErr = fmt.Errorf("standard fetch failed: %w; utls-chrome fetch failed: %v", lastErr, utlsErr)
		} else {
			lastMessage = utlsMessage
			lastErr = utlsErr
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
	return fallbackDeepSeekDirectProbe(ctx, monitor, result, lastMessage, lastErr)
}

func fetchDeepSeekRSS(ctx context.Context, client *http.Client, endpoint, mode string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, "Failed to create RSS request for " + endpoint, err
	}
	req.Header.Set("User-Agent", deepSeekRSSUserAgent(mode))
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Sprintf("%s network error fetching %s", mode, endpoint), err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Sprintf("%s RSS request returned HTTP %d from %s", mode, resp.StatusCode, endpoint), fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Sprintf("%s read error from %s", mode, endpoint), err
	}
	return body, "", nil
}

func deepSeekRSSUserAgent(mode string) string {
	if mode == "utls-chrome" {
		return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36"
	}
	return "IsLLMAlive/1.0"
}

func newDeepSeekUTLSHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: timeout}
	transport := &http.Transport{
		ForceAttemptHTTP2: false,
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			rawConn, err := dialer.DialContext(ctx, network, addr)
			if err != nil {
				return nil, err
			}

			serverName, _, err := net.SplitHostPort(addr)
			if err != nil {
				_ = rawConn.Close()
				return nil, err
			}

			tlsConn := utls.UClient(rawConn, &utls.Config{
				ServerName: serverName,
				NextProtos: []string{
					"http/1.1",
				},
			}, utls.HelloChrome_Auto)
			if err := tlsConn.HandshakeContext(ctx); err != nil {
				_ = rawConn.Close()
				return nil, err
			}

			return tlsConn, nil
		},
	}
	return &http.Client{Timeout: timeout, Transport: transport}
}

type deepSeekProbeResult struct {
	Name       string
	Status     status.Status
	Message    string
	Err        error
	HTTPStatus int
}

func fallbackDeepSeekDirectProbe(ctx context.Context, monitor config.MonitorConfig, result MonitorResult, rssMessage string, rssErr error) MonitorResult {
	targetComponent := normalizeDeepSeekComponent(monitor.Component)
	if monitor.Component != "" && !strings.EqualFold(monitor.Component, "none") && targetComponent == "" {
		result.Message = "Unsupported component"
		result.Err = fmt.Errorf("unsupported deepseek component: %s; rss_error: %w", monitor.Component, rssErr)
		return result
	}

	client := &http.Client{Timeout: 3 * time.Second}
	var probes []deepSeekProbeResult
	switch targetComponent {
	case "api":
		probes = append(probes, probeDeepSeekService(ctx, client, "api", deepSeekAPIProbeURL))
	case "web_chat":
		probes = append(probes, probeDeepSeekService(ctx, client, "web", deepSeekWebProbeURL))
	default:
		probes = append(probes, probeDeepSeekService(ctx, client, "api", deepSeekAPIProbeURL))
		probes = append(probes, probeDeepSeekService(ctx, client, "web", deepSeekWebProbeURL))
	}

	worstStatus := status.Normal
	allUnknown := true
	parts := make([]string, 0, len(probes)+1)
	for _, probe := range probes {
		parts = append(parts, probe.Message)
		if probe.Status != status.Unknown {
			allUnknown = false
		}
		if probe.Status > worstStatus {
			worstStatus = probe.Status
		}
	}
	if allUnknown {
		worstStatus = status.Unknown
	}

	result.Status = worstStatus
	result.Message = fmt.Sprintf("RSS unavailable; direct probe fallback used; %s; rss_message=%s", strings.Join(parts, "; "), rssMessage)
	if rssErr != nil {
		result.Err = fmt.Errorf("rss fetch failed before direct probe fallback: %w", rssErr)
	} else {
		result.Err = nil
	}
	return result
}

func probeDeepSeekService(ctx context.Context, client *http.Client, name, endpoint string) deepSeekProbeResult {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return deepSeekProbeResult{
			Name:    name,
			Status:  status.Unknown,
			Message: fmt.Sprintf("%s=Unknown(create request failed)", name),
			Err:     err,
		}
	}
	req.Header.Set("User-Agent", "IsLLMAlive-Monitor/1.0")
	req.Header.Set("Accept", "*/*")

	resp, err := client.Do(req)
	if err != nil {
		return deepSeekProbeResult{
			Name:    name,
			Status:  status.Unknown,
			Message: fmt.Sprintf("%s=Unknown(network error: %v)", name, err),
			Err:     err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		err := fmt.Errorf("direct probe %s returned server error: %d", endpoint, resp.StatusCode)
		return deepSeekProbeResult{
			Name:       name,
			Status:     status.Outage,
			Message:    fmt.Sprintf("%s=Outage(HTTP %d)", name, resp.StatusCode),
			Err:        err,
			HTTPStatus: resp.StatusCode,
		}
	}

	return deepSeekProbeResult{
		Name:       name,
		Status:     status.Normal,
		Message:    fmt.Sprintf("%s=Normal(HTTP %d)", name, resp.StatusCode),
		HTTPStatus: resp.StatusCode,
	}
}

func parseDeepSeekRSS(data []byte, monitor config.MonitorConfig, result MonitorResult, now time.Time) MonitorResult {
	var feed deepSeekRSS
	decoder := xml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&feed); err != nil {
		result.Message = "Parse error"
		result.Err = err
		return result
	}

	targetComponent := normalizeDeepSeekComponent(monitor.Component)
	if monitor.Component != "" && !strings.EqualFold(monitor.Component, "none") && targetComponent == "" {
		result.Message = "Unsupported component"
		result.Err = fmt.Errorf("unsupported deepseek component: %s", monitor.Component)
		return result
	}

	latestByIncident := make(map[string]deepSeekIncident)
	parsedItems := 0
	var latestPublished time.Time
	for _, item := range feed.Channel.Items {
		incident, err := parseDeepSeekRSSItem(item)
		if err != nil {
			continue
		}
		parsedItems++
		if incident.Published.After(latestPublished) {
			latestPublished = incident.Published
		}
		if incident.Published.After(now.Add(5 * time.Minute)) {
			continue
		}

		key := incident.Link
		if key == "" {
			key = item.GUID
		}
		if key == "" {
			key = incident.Title
		}

		prev, exists := latestByIncident[key]
		if !exists || incident.Published.After(prev.Published) {
			latestByIncident[key] = incident
		}
	}

	if len(feed.Channel.Items) > 0 && parsedItems == 0 {
		result.Message = fmt.Sprintf("Parse error: feed_items=%d parsed_items=0", len(feed.Channel.Items))
		result.Err = fmt.Errorf("no parseable deepseek rss items")
		return result
	}

	worstStatus := status.Normal
	var activeTitles []string
	for _, incident := range latestByIncident {
		if strings.EqualFold(strings.TrimSpace(incident.StatusText), "resolved") {
			continue
		}
		if targetComponent != "" && !deepSeekIncidentAffectsComponent(incident, targetComponent) {
			continue
		}

		if incident.Severity > worstStatus {
			worstStatus = incident.Severity
		}
		activeTitles = append(activeTitles, incident.Title)
	}

	result.Status = worstStatus
	if len(activeTitles) == 0 {
		result.Message = deepSeekRSSSummary("operational", len(feed.Channel.Items), parsedItems, 0, latestPublished)
	} else {
		result.Message = deepSeekRSSSummary(strings.Join(activeTitles, ", "), len(feed.Channel.Items), parsedItems, len(activeTitles), latestPublished)
	}
	return result
}

func deepSeekRSSSummary(prefix string, feedItems, parsedItems, activeItems int, latestPublished time.Time) string {
	if latestPublished.IsZero() {
		return fmt.Sprintf("%s; feed_items=%d parsed_items=%d active_items=%d", prefix, feedItems, parsedItems, activeItems)
	}
	return fmt.Sprintf("%s; feed_items=%d parsed_items=%d active_items=%d latest=%s", prefix, feedItems, parsedItems, activeItems, latestPublished.Format(time.RFC3339))
}

func parseDeepSeekRSSItem(item deepSeekRSSItem) (deepSeekIncident, error) {
	published, err := time.Parse(time.RFC1123Z, strings.TrimSpace(item.PubDate))
	if err != nil {
		return deepSeekIncident{}, err
	}

	description := html.UnescapeString(item.Description)
	statusText := extractDeepSeekDescriptionField(description, "Status")
	componentsText := extractDeepSeekDescriptionField(description, "Affected components")

	return deepSeekIncident{
		Title:      strings.TrimSpace(item.Title),
		Link:       strings.TrimSpace(item.Link),
		StatusText: strings.TrimSpace(statusText),
		Components: parseDeepSeekComponents(componentsText),
		Published:  published,
		Severity:   classifyDeepSeekSeverity(item.Title),
	}, nil
}

func extractDeepSeekDescriptionField(description, label string) string {
	pattern := fmt.Sprintf(`(?is)<strong>\s*%s:\s*</strong>\s*([^<]+)`, regexp.QuoteMeta(label))
	matches := regexp.MustCompile(pattern).FindStringSubmatch(description)
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

func parseDeepSeekComponents(text string) []string {
	parts := strings.Split(text, ",")
	components := make([]string, 0, len(parts))
	for _, part := range parts {
		component := normalizeDeepSeekComponent(part)
		if component != "" {
			components = append(components, component)
		}
	}
	return components
}

func deepSeekIncidentAffectsComponent(incident deepSeekIncident, targetComponent string) bool {
	for _, component := range incident.Components {
		if component == targetComponent {
			return true
		}
	}
	return false
}

func normalizeDeepSeekComponent(component string) string {
	normalized := strings.ToLower(strings.TrimSpace(component))
	if normalized == "" || normalized == "none" {
		return ""
	}
	if strings.Contains(normalized, "api") || strings.Contains(normalized, "接口") {
		return "api"
	}
	if strings.Contains(normalized, "web") || strings.Contains(normalized, "网页") || strings.Contains(normalized, "对话") {
		return "web_chat"
	}
	return ""
}

func classifyDeepSeekSeverity(title string) status.Status {
	normalized := strings.ToLower(title)
	switch {
	case strings.Contains(normalized, "unavailable"),
		strings.Contains(normalized, "not available"),
		strings.Contains(normalized, "不可用"),
		strings.Contains(normalized, "宕机"):
		return status.Outage
	case strings.Contains(normalized, "degraded performance"),
		strings.Contains(normalized, "performance abnormal"),
		strings.Contains(normalized, "性能下降"),
		strings.Contains(normalized, "性能异常"),
		strings.Contains(normalized, "abnormal"),
		strings.Contains(normalized, "异常"):
		return status.Degraded
	default:
		return status.Degraded
	}
}
