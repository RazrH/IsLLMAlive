package providers

import (
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"net"
	"net/http"
	"regexp"
	"slices"
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
	deepSeekStatusPattern     = regexp.MustCompile(`(?is)<strong>\s*Status:\s*</strong>\s*([^<]+)`)
	deepSeekComponentsPattern = regexp.MustCompile(`(?is)<strong>\s*Affected components:\s*</strong>\s*([^<]+)`)
	deepSeekAPIProbeURL       = "https://api.deepseek.com/v1/models"
	deepSeekWebProbeURL       = "https://chat.deepseek.com"
)

// DeepSeekProvider implements DeepSeek status monitoring from its Flashduty RSS feed.
type DeepSeekProvider struct{}

type deepSeekRSS struct {
	XMLName xml.Name            `xml:"rss"`
	Channel *deepSeekRSSChannel `xml:"channel"`
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
	Title             string
	Link              string
	StatusText        string
	Components        []string
	UnknownComponents []string
	ComponentLabels   string
	Published         time.Time
	Severity          status.Status
}

func (p *DeepSeekProvider) Fetch(ctx context.Context, monitor config.MonitorConfig) MonitorResult {
	monitor.Type = "deepseek"
	return fetchStandalone(ctx, monitor)
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

	resp, err := doRequest(client, req)
	if err != nil {
		return nil, fmt.Sprintf("%s network error fetching %s", mode, endpoint), err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Sprintf("%s RSS request returned HTTP %d from %s", mode, resp.StatusCode, endpoint), fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := readResponse(resp.Body)
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
	configureTransport(transport)
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
	client := &http.Client{Timeout: 3 * time.Second}
	if componentNames := monitor.ComponentNames(); len(componentNames) > 0 {
		componentStatuses := make([]ComponentStatus, 0, len(componentNames))
		probes := make(map[string]deepSeekProbeResult)
		var unsupported []string
		for _, name := range componentNames {
			target := normalizeDeepSeekComponent(name)
			if target == "" {
				componentStatuses = append(componentStatuses, ComponentStatus{Name: name, Status: status.Unknown, Message: "Unsupported component"})
				unsupported = append(unsupported, name)
				continue
			}
			if target != "api" && target != "web_chat" {
				componentStatuses = append(componentStatuses, ComponentStatus{Name: name, Status: status.Unknown, Message: "RSS unavailable; no direct probe for this service or model family"})
				continue
			}
			probe, ok := probes[target]
			if !ok {
				if target == "api" {
					probe = probeDeepSeekService(ctx, client, "api", deepSeekAPIProbeURL)
				} else {
					probe = probeDeepSeekService(ctx, client, "web", deepSeekWebProbeURL)
				}
				probes[target] = probe
			}
			componentStatuses = append(componentStatuses, ComponentStatus{Name: name, Status: probe.Status, Message: probe.Message})
		}
		result = aggregateComponentStatuses(result, componentStatuses)
		result.Message = fmt.Sprintf("RSS unavailable; direct probe fallback used; %s; rss_message=%s", result.Message, rssMessage)
		if len(unsupported) > 0 {
			result.Err = fmt.Errorf("unsupported deepseek components: %s; rss_error: %v", strings.Join(unsupported, ", "), rssErr)
		} else if rssErr != nil {
			result.Err = fmt.Errorf("rss fetch failed before direct probe fallback: %w", rssErr)
		}
		return result
	}

	var probes []deepSeekProbeResult
	probes = append(probes, probeDeepSeekService(ctx, client, "api", deepSeekAPIProbeURL))
	probes = append(probes, probeDeepSeekService(ctx, client, "web", deepSeekWebProbeURL))

	worstStatus := status.Normal
	hasUnknown := false
	parts := make([]string, 0, len(probes)+1)
	for _, probe := range probes {
		parts = append(parts, probe.Message)
		if probe.Status == status.Unknown {
			hasUnknown = true
		}
		if probe.Status > worstStatus {
			worstStatus = probe.Status
		}
	}
	if hasUnknown && worstStatus == status.Normal {
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

	resp, err := doRequest(client, req)
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

func parseDeepSeekRSS(data []byte, m config.MonitorConfig, b MonitorResult, now time.Time) MonitorResult {
	s, err := decodeDeepSeekSnapshot(data, now)
	if err != nil {
		return incomplete(b, err)
	}
	return s.evaluate(m, b)
}

func evaluateDeepSeekIncidents(latestByIncident map[string]deepSeekIncident, targetComponent string) (status.Status, string, int, error) {
	worstStatus := status.Normal
	var activeTitles []string
	var issues problems
	for _, incident := range latestByIncident {
		if strings.EqualFold(strings.TrimSpace(incident.StatusText), "resolved") {
			continue
		}
		if len(incident.UnknownComponents) > 0 {
			issues.add(fmt.Errorf("unrecognized RSS components: %s", strings.Join(incident.UnknownComponents, ", ")))
		}
		if (targetComponent == "api_pro" || targetComponent == "api_flash") && slices.Contains(incident.Components, "api") && !slices.Contains(incident.Components, targetComponent) {
			issues.add(fmt.Errorf("RSS incident does not distinguish Pro/Flash: %s", incident.ComponentLabels))
		}
		if targetComponent != "" && !deepSeekIncidentAffectsComponent(incident, targetComponent) {
			continue
		}
		if incident.Severity > worstStatus {
			worstStatus = incident.Severity
		}
		activeTitles = append(activeTitles, incident.Title)
	}
	message := "operational"
	if len(activeTitles) > 0 {
		message = strings.Join(activeTitles, ", ")
	}
	result := incomplete(MonitorResult{Status: worstStatus, Message: message}, issues.err())
	return result.Status, result.Message, len(activeTitles), result.Err
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
	components, unknown := parseDeepSeekComponents(componentsText)
	if strings.TrimSpace(item.Title) == "" {
		return deepSeekIncident{}, fmt.Errorf("RSS item title missing")
	}
	switch strings.ToLower(strings.TrimSpace(statusText)) {
	case "resolved", "investigating", "identified", "monitoring":
	default:
		return deepSeekIncident{}, fmt.Errorf("unknown RSS incident status: %q", statusText)
	}
	if !strings.EqualFold(strings.TrimSpace(statusText), "resolved") && len(components) == 0 {
		return deepSeekIncident{}, fmt.Errorf("active RSS incident has no interpretable components: %s", componentsText)
	}

	return deepSeekIncident{
		Title:             strings.TrimSpace(item.Title),
		Link:              strings.TrimSpace(item.Link),
		StatusText:        strings.TrimSpace(statusText),
		Components:        components,
		UnknownComponents: unknown,
		ComponentLabels:   strings.Clone(componentsText),
		Published:         published,
		Severity:          classifyDeepSeekSeverity(item.Title),
	}, nil
}

func extractDeepSeekDescriptionField(description, label string) string {
	pattern := deepSeekStatusPattern
	switch label {
	case "Status":
	case "Affected components":
		pattern = deepSeekComponentsPattern
	default:
		return ""
	}
	matches := pattern.FindStringSubmatch(description)
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

func parseDeepSeekComponents(text string) (components, unknown []string) {
	for part := range strings.SplitSeq(text, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		component := normalizeDeepSeekComponent(part)
		if component == "" {
			unknown = append(unknown, strings.Clone(part))
		} else if !slices.Contains(components, component) {
			components = append(components, component)
		}
	}
	return components, unknown
}

func deepSeekIncidentAffectsComponent(incident deepSeekIncident, target string) bool {
	if target == "api" {
		return slices.Contains(incident.Components, "api") || slices.Contains(incident.Components, "api_pro") || slices.Contains(incident.Components, "api_flash")
	}
	return slices.Contains(incident.Components, target)
}

func normalizeDeepSeekComponent(component string) string {
	normalized := strings.ToLower(strings.TrimSpace(component))
	if normalized == "" || normalized == "none" {
		return ""
	}
	if strings.Contains(normalized, "api") || strings.Contains(normalized, "接口") {
		words := strings.FieldsFunc(normalized, func(r rune) bool { return r < 'a' || r > 'z' })
		pro, flash := slices.Contains(words, "pro"), slices.Contains(words, "flash")
		if pro && !flash {
			return "api_pro"
		}
		if flash && !pro {
			return "api_flash"
		}
		return "api"
	}
	if strings.Contains(normalized, "web") || strings.Contains(normalized, "网页") || strings.Contains(normalized, "对话") || strings.Contains(normalized, "chat service") || strings.Contains(normalized, "chatservice") {
		return "web_chat"
	}
	if strings.Contains(normalized, "file upload") || strings.Contains(normalized, "上传文件") {
		return "file_upload"
	}
	if strings.Contains(normalized, "search service") || strings.Contains(normalized, "搜索服务") {
		return "search"
	}
	return ""
}

func classifyDeepSeekSeverity(title string) status.Status {
	normalized := strings.ToLower(title)
	switch {
	case strings.Contains(normalized, "partially unavailable"), strings.Contains(normalized, "partial outage"), strings.Contains(normalized, "部分中断"):
		return status.Degraded
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
