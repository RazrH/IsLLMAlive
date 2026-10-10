package providers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"isllmalive/internal/config"
	"isllmalive/internal/status"
)

func TestResponseValidity(t *testing.T) {
	tests := []struct {
		name, kind, body string
		components       []string
		want             status.Status
	}{
		{"openai missing summary", "openai", `{}`, nil, status.Unknown},
		{"openai null summary", "openai", `{"summary":null}`, nil, status.Unknown},
		{"openai missing incidents", "openai", `{"summary":{"components":[],"affected_components":[]}}`, nil, status.Unknown},
		{"openai null affected", "openai", `{"summary":{"components":[],"affected_components":null,"ongoing_incidents":[]}}`, nil, status.Unknown},
		{"openai valid empty", "openai", `{"extra":true,"summary":{"components":[],"affected_components":[],"ongoing_incidents":[]}}`, nil, status.Normal},
		{"openai unknown affected", "openai", `{"summary":{"components":[{"id":"a","name":"API"}],"affected_components":[{"component_id":"a","status":"future"}],"ongoing_incidents":[]}}`, nil, status.Unknown},
		{"openai mixed components", "openai", `{"summary":{"components":[{"id":"a","name":"API"},{"id":"b","name":"Web"}],"affected_components":[{"component_id":"a","status":"future"}]}}`, []string{"API", "Web"}, status.Unknown},
		{"statuspage missing", "statuspage", `{}`, nil, status.Unknown},
		{"statuspage invalid components no fallback", "statuspage", `{"components":[{"name":"API","status":"future"}]}`, nil, status.Unknown},
		{"statuspage valid components no fallback", "statuspage", `{"components":[{"name":"API","status":"operational"}]}`, nil, status.Unknown},
		{"statuspage component independent", "statuspage", `{"components":[{"name":"API","status":"operational"}]}`, []string{"API"}, status.Normal},
		{"statuspage mixed", "statuspage", `{"components":[{"name":"API","status":"operational"},{"name":"Web","status":"future"}]}`, []string{"API", "Web"}, status.Unknown},
		{"statuspage known outage wins", "statuspage", `{"components":[{"name":"API","status":"major_outage"},{"name":"Web","status":"future"}]}`, []string{"API", "Web"}, status.Outage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, tt.body) }))
			defer server.Close()
			if tt.kind == "openai" {
				old := openAIEndpoint
				openAIEndpoint = server.URL
				t.Cleanup(func() { openAIEndpoint = old })
			}
			got := NewProvider(tt.kind).Fetch(t.Context(), config.MonitorConfig{Name: "test", Endpoint: server.URL, Components: tt.components})
			if got.Status != tt.want {
				t.Fatalf("status=%v want=%v message=%s", got.Status, tt.want, got.Message)
			}
			if got.Status == status.Unknown && got.Err == nil {
				t.Fatal("Unknown needs a diagnostic reason")
			}
		})
	}
}

func TestGoogleMalformedRecords(t *testing.T) {
	for _, tt := range []struct {
		incidents string
		want      status.Status
	}{
		{`[]`, status.Normal},
		{`[null]`, status.Unknown},
		{`[[],["id","resolved",1,[[4]]]]`, status.Unknown},
		{`[[],["id","outage",2,[[1]],null,[1]]]`, status.Outage},
		{`[["id","future",1,[[99]],null,[1]]]`, status.Unknown},
	} {
		t.Run(tt.incidents, func(t *testing.T) {
			var raw []any
			if err := json.Unmarshal([]byte("[["+tt.incidents+"]]"), &raw); err != nil {
				t.Fatal(err)
			}
			got := (&GoogleProvider{}).parseData(raw, config.MonitorConfig{}, MonitorResult{})
			if got.Status != tt.want {
				t.Fatalf("got %v want %v", got.Status, tt.want)
			}
			if tt.want != status.Normal && got.Err == nil {
				t.Fatal("malformed records need an error even with confirmed outage")
			}
		})
	}
}

func TestRSSValidity(t *testing.T) {
	valid := `<item><title>API unavailable</title><description>&lt;strong&gt;Status:&lt;/strong&gt; %s&lt;strong&gt;Affected components:&lt;/strong&gt; API Service</description><pubDate>Thu, 28 May 2026 19:24:51 +0800</pubDate></item>`
	for _, tt := range []struct {
		body string
		want status.Status
	}{
		{"<anything/>", status.Unknown}, {"<rss/>", status.Unknown},
		{"<rss><channel/></rss>", status.Normal},
		{"<rss><channel/></rss><other/>", status.Unknown},
		{"<rss><channel><item/></channel></rss>", status.Unknown},
		{"<rss><channel><item/>" + fmt.Sprintf(valid, "resolved") + "</channel></rss>", status.Unknown},
		{"<rss><channel><item/>" + fmt.Sprintf(valid, "investigating") + "</channel></rss>", status.Outage},
		{"<rss><channel>" + fmt.Sprintf(valid, "future") + "</channel></rss>", status.Unknown},
	} {
		t.Run(tt.body, func(t *testing.T) {
			got := parseDeepSeekRSS([]byte(tt.body), config.MonitorConfig{}, deepSeekTestResult(), deepSeekTestNow())
			if got.Status != tt.want {
				t.Fatalf("got %v want %v", got.Status, tt.want)
			}
			if tt.want != status.Normal && got.Err == nil {
				t.Fatal("invalid feed needs a diagnostic reason")
			}
		})
	}
}

func TestOpenAIQualifiedComponentMustExistInSummary(t *testing.T) {
	var data oaiProxyResponse
	if err := json.Unmarshal([]byte(`{"summary":{"components":[],"affected_components":[],"structure":{"items":[{"group":{"name":"APIs","components":[{"component_id":"ghost","name":"Login"}]}}]}}}`), &data); err != nil {
		t.Fatal(err)
	}
	result := evaluateOpenAI(data, config.MonitorConfig{Components: []string{"APIs/Login"}}, MonitorResult{})
	if result.Status != status.Unknown || result.Err == nil {
		t.Fatalf("unconfirmed component reported healthy: %#v", result)
	}
}

func TestMalformedListDiagnosticIsBounded(t *testing.T) {
	records := make([]any, 10000)
	raw := []any{[]any{records}}
	got := (&GoogleProvider{}).parseData(raw, config.MonitorConfig{}, MonitorResult{})
	if got.Status != status.Unknown || got.Err == nil {
		t.Fatal("invalid list accepted")
	}
	if len(got.Err.Error()) > 1024 {
		t.Fatal("untrusted malformed list produced unbounded diagnostic")
	}
}
