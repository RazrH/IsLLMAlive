package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
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
