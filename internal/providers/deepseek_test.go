package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"isllmalive/internal/config"
	"isllmalive/internal/status"
)

func TestParseDeepSeekRSSResolvedOnlyReturnsNormal(t *testing.T) {
	rss := `<rss version="2.0"><channel><item>
<title>API 性能下降（API Degraded Performance）</title>
<link>https://status.deepseek.com/incidents/1</link>
<description>&lt;p&gt;&lt;strong&gt;Status:&lt;/strong&gt; resolved&lt;/p&gt;&lt;p&gt;&lt;strong&gt;Affected components:&lt;/strong&gt; API 服务 (API Service)&lt;/p&gt;</description>
<pubDate>Thu, 28 May 2026 19:24:51 +0800</pubDate>
</item></channel></rss>`

	got := parseDeepSeekRSS([]byte(rss), config.MonitorConfig{Name: "DeepSeek"}, deepSeekTestResult(), deepSeekTestNow())

	if got.Status != status.Normal {
		t.Fatalf("Status = %s, want Normal", got.Status)
	}
}

func TestParseDeepSeekRSSActiveDegradedForAPIComponent(t *testing.T) {
	rss := `<rss version="2.0"><channel><item>
<title>API 性能下降（API Degraded Performance）</title>
<link>https://status.deepseek.com/incidents/1</link>
<description>&lt;p&gt;&lt;strong&gt;Status:&lt;/strong&gt; investigating&lt;/p&gt;&lt;p&gt;&lt;strong&gt;Affected components:&lt;/strong&gt; API 服务 (API Service)&lt;/p&gt;</description>
<pubDate>Thu, 28 May 2026 19:24:51 +0800</pubDate>
</item></channel></rss>`

	got := parseDeepSeekRSS([]byte(rss), config.MonitorConfig{Name: "DeepSeek", Component: "API Service"}, deepSeekTestResult(), deepSeekTestNow())

	if got.Status != status.Degraded {
		t.Fatalf("Status = %s, want Degraded", got.Status)
	}
}

func TestParseDeepSeekRSSActiveOutageForWebComponent(t *testing.T) {
	rss := `<rss version="2.0"><channel><item>
<title>DeepSeek 网页端/API 不可用（DeepSeek WEB/API Unavailable）</title>
<link>https://status.deepseek.com/incidents/1</link>
<description>&lt;p&gt;&lt;strong&gt;Status:&lt;/strong&gt; identified&lt;/p&gt;&lt;p&gt;&lt;strong&gt;Affected components:&lt;/strong&gt; API 服务 (API Service), 网页对话服务 (Web Chat Service)&lt;/p&gt;</description>
<pubDate>Thu, 28 May 2026 19:24:51 +0800</pubDate>
</item></channel></rss>`

	got := parseDeepSeekRSS([]byte(rss), config.MonitorConfig{Name: "DeepSeek", Component: "网页对话服务"}, deepSeekTestResult(), deepSeekTestNow())

	if got.Status != status.Outage {
		t.Fatalf("Status = %s, want Outage", got.Status)
	}
}

func TestParseDeepSeekRSSComponentFilterIgnoresUnaffectedIncident(t *testing.T) {
	rss := `<rss version="2.0"><channel><item>
<title>API 性能下降（API Degraded Performance）</title>
<link>https://status.deepseek.com/incidents/1</link>
<description>&lt;p&gt;&lt;strong&gt;Status:&lt;/strong&gt; investigating&lt;/p&gt;&lt;p&gt;&lt;strong&gt;Affected components:&lt;/strong&gt; API 服务 (API Service)&lt;/p&gt;</description>
<pubDate>Thu, 28 May 2026 19:24:51 +0800</pubDate>
</item></channel></rss>`

	got := parseDeepSeekRSS([]byte(rss), config.MonitorConfig{Name: "DeepSeek", Component: "Web Chat Service"}, deepSeekTestResult(), deepSeekTestNow())

	if got.Status != status.Normal {
		t.Fatalf("Status = %s, want Normal", got.Status)
	}
}

func TestParseDeepSeekRSSKeepsNewestIncidentChange(t *testing.T) {
	rss := `<rss version="2.0"><channel>
<item>
<title>API 性能下降（API Degraded Performance）</title>
<link>https://status.deepseek.com/incidents/1</link>
<description>&lt;p&gt;&lt;strong&gt;Status:&lt;/strong&gt; investigating&lt;/p&gt;&lt;p&gt;&lt;strong&gt;Affected components:&lt;/strong&gt; API 服务 (API Service)&lt;/p&gt;</description>
<pubDate>Thu, 28 May 2026 18:24:51 +0800</pubDate>
</item>
<item>
<title>API 性能下降（API Degraded Performance）</title>
<link>https://status.deepseek.com/incidents/1</link>
<description>&lt;p&gt;&lt;strong&gt;Status:&lt;/strong&gt; resolved&lt;/p&gt;&lt;p&gt;&lt;strong&gt;Affected components:&lt;/strong&gt; API 服务 (API Service)&lt;/p&gt;</description>
<pubDate>Thu, 28 May 2026 19:24:51 +0800</pubDate>
</item>
</channel></rss>`

	got := parseDeepSeekRSS([]byte(rss), config.MonitorConfig{Name: "DeepSeek"}, deepSeekTestResult(), deepSeekTestNow())

	if got.Status != status.Normal {
		t.Fatalf("Status = %s, want Normal", got.Status)
	}
}

func TestParseDeepSeekRSSReturnsUnknownWhenItemsAreUnparseable(t *testing.T) {
	rss := `<rss version="2.0"><channel><item>
<title>API 性能下降（API Degraded Performance）</title>
<link>https://status.deepseek.com/incidents/1</link>
<description>&lt;p&gt;&lt;strong&gt;Status:&lt;/strong&gt; investigating&lt;/p&gt;&lt;p&gt;&lt;strong&gt;Affected components:&lt;/strong&gt; API 服务 (API Service)&lt;/p&gt;</description>
<pubDate>bad date</pubDate>
</item></channel></rss>`

	got := parseDeepSeekRSS([]byte(rss), config.MonitorConfig{Name: "DeepSeek"}, deepSeekTestResult(), deepSeekTestNow())

	if got.Status != status.Unknown {
		t.Fatalf("Status = %s, want Unknown", got.Status)
	}
	if got.Err == nil {
		t.Fatal("Err is nil, want parse error")
	}
}

func TestFetchDeepSeekRSSUsesChromeUserAgentInUTLSMode(t *testing.T) {
	var gotUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.UserAgent()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<rss version="2.0"><channel></channel></rss>`))
	}))
	defer server.Close()

	_, _, err := fetchDeepSeekRSS(context.Background(), server.Client(), server.URL, "utls-chrome")
	if err != nil {
		t.Fatalf("fetchDeepSeekRSS() error = %v", err)
	}

	if !strings.Contains(gotUA, "Chrome/133.0.0.0") {
		t.Fatalf("User-Agent = %q, want Chrome UA", gotUA)
	}
}

func TestFallbackDeepSeekDirectProbeAPIComponentUsesAPIOnly(t *testing.T) {
	apiHits := 0
	webHits := 0
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiHits++
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer apiServer.Close()
	webServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webHits++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer webServer.Close()
	withDeepSeekProbeURLs(t, apiServer.URL, webServer.URL)

	got := fallbackDeepSeekDirectProbe(context.Background(), config.MonitorConfig{
		Name:      "DeepSeek",
		Component: "API Service",
	}, deepSeekTestResult(), "rss failed", context.DeadlineExceeded)

	if got.Status != status.Normal {
		t.Fatalf("Status = %s, want Normal", got.Status)
	}
	if apiHits != 1 {
		t.Fatalf("apiHits = %d, want 1", apiHits)
	}
	if webHits != 0 {
		t.Fatalf("webHits = %d, want 0", webHits)
	}
	if !strings.Contains(got.Message, "api=Normal(HTTP 401)") {
		t.Fatalf("Message = %q, want API fallback detail", got.Message)
	}
	if got.Err == nil {
		t.Fatal("Err is nil, want RSS failure preserved")
	}
}

func TestFallbackDeepSeekDirectProbePageLevelTakesWorstStatus(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer apiServer.Close()
	webServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer webServer.Close()
	withDeepSeekProbeURLs(t, apiServer.URL, webServer.URL)

	got := fallbackDeepSeekDirectProbe(context.Background(), config.MonitorConfig{
		Name:      "DeepSeek",
		Component: "none",
	}, deepSeekTestResult(), "rss failed", context.DeadlineExceeded)

	if got.Status != status.Outage {
		t.Fatalf("Status = %s, want Outage", got.Status)
	}
	if !strings.Contains(got.Message, "api=Normal(HTTP 401)") || !strings.Contains(got.Message, "web=Outage(HTTP 502)") {
		t.Fatalf("Message = %q, want API and web fallback details", got.Message)
	}
}

func TestDeepSeekRSSLive(t *testing.T) {
	if os.Getenv("ISLLMALIVE_LIVE_DEEPSEEK") != "1" {
		t.Skip("set ISLLMALIVE_LIVE_DEEPSEEK=1 to run live DeepSeek RSS fetch")
	}

	provider := &DeepSeekProvider{}
	got := provider.Fetch(context.Background(), config.MonitorConfig{
		Name:     "DeepSeek",
		Type:     "deepseek",
		Endpoint: deepSeekDefaultFeedURL,
	})

	if got.Status == status.Unknown {
		t.Fatalf("DeepSeekProvider.Fetch() returned Unknown: message=%q err=%v", got.Message, got.Err)
	}
	t.Logf("status=%s message=%s", got.Status, got.Message)
}

func deepSeekTestResult() MonitorResult {
	return MonitorResult{Name: "DeepSeek", Status: status.Unknown, StatusPage: deepSeekDefaultStatusPage}
}

func withDeepSeekProbeURLs(t *testing.T, apiURL, webURL string) {
	t.Helper()
	origAPI := deepSeekAPIProbeURL
	origWeb := deepSeekWebProbeURL
	deepSeekAPIProbeURL = apiURL
	deepSeekWebProbeURL = webURL
	t.Cleanup(func() {
		deepSeekAPIProbeURL = origAPI
		deepSeekWebProbeURL = origWeb
	})
}

func deepSeekTestNow() time.Time {
	return time.Date(2026, 5, 29, 0, 0, 0, 0, time.FixedZone("CST", 8*60*60))
}
