package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"isllmalive/internal/config"
	"isllmalive/internal/status"
)

func TestOpenAIMultipleComponentsUseWorstStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"summary":{"components":[{"id":"1","name":"API"},{"id":"2","name":"Web"}],"affected_components":[{"component_id":"1","status":"degraded_performance"},{"component_id":"2","status":"major_outage"}]}}`))
	}))
	defer server.Close()
	originalEndpoint := openAIEndpoint
	openAIEndpoint = server.URL
	t.Cleanup(func() { openAIEndpoint = originalEndpoint })

	got := (&OpenAIProvider{}).Fetch(context.Background(), config.MonitorConfig{Name: "OpenAI", Components: []string{"API", "Web"}})

	if got.Status != status.Outage {
		t.Fatalf("Status = %s, want Outage", got.Status)
	}
	if len(got.ComponentStatuses) != 2 || got.ComponentStatuses[0].Status != status.Degraded {
		t.Fatalf("ComponentStatuses = %#v", got.ComponentStatuses)
	}
}

func TestOpenAIQualifiedComponentsAndDuplicateNames(t *testing.T) {
	var proxyResp oaiProxyResponse
	data := []byte(`{"summary":{
"components":[{"id":"api-login","name":"Login"},{"id":"chat-login","name":"Login"},{"id":"codex-api","name":"Codex API"},{"id":"connectors","name":"Connectors/Apps"}],
"structure":{"items":[
{"group":{"name":"APIs","components":[{"component_id":"api-login","name":"Login"}]}},
{"group":{"name":"ChatGPT","components":[{"component_id":"chat-login","name":"Login"},{"component_id":"connectors","name":"Connectors/Apps"}]}},
{"group":{"name":"Codex","components":[{"component_id":"codex-api","name":"Codex API"}]}}
]}}}`)
	if err := json.Unmarshal(data, &proxyResp); err != nil {
		t.Fatal(err)
	}
	affected := map[string]string{
		"chat-login": "major_outage",
		"codex-api":  "degraded_performance",
	}

	got, missing := resolveOAIComponentStatuses(proxyResp, []string{
		"APIs/Login", "ChatGPT/Login", "Codex/Codex API", "Codex API", "Connectors/Apps",
	}, affected)
	if len(missing) != 0 {
		t.Fatalf("missing = %#v", missing)
	}
	wants := []status.Status{status.Normal, status.Outage, status.Degraded, status.Degraded, status.Normal}
	for i, want := range wants {
		if got[i].Status != want {
			t.Fatalf("status[%d] = %s, want %s; results=%#v", i, got[i].Status, want, got)
		}
	}

	login, missing := resolveOAIComponentStatuses(proxyResp, []string{"Login"}, affected)
	if len(missing) != 0 || login[0].Status != status.Outage {
		t.Fatalf("unqualified Login = %#v, missing=%#v; want aggregated Outage", login, missing)
	}
	if !strings.Contains(login[0].Message, "ChatGPT/Login: major_outage") {
		t.Fatalf("unqualified Login message = %q, want qualified failing component", login[0].Message)
	}
}
