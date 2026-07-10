package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"isllmalive/internal/config"
	"isllmalive/internal/status"
)

func TestStatuspageMultipleComponentsUseWorstStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":{"indicator":"none"},"components":[{"name":"API","status":"operational"},{"name":"Web","status":"partial_outage"},{"name":"Login","status":"major_outage"}]}`))
	}))
	defer server.Close()

	got := (&StatuspageProvider{}).Fetch(context.Background(), config.MonitorConfig{
		Name: "Test", Endpoint: server.URL, Components: []string{"API", "Web", "Login", "Missing"},
	})

	if got.Status != status.Outage {
		t.Fatalf("Status = %s, want Outage", got.Status)
	}
	if len(got.ComponentStatuses) != 4 || got.ComponentStatuses[3].Status != status.Unknown {
		t.Fatalf("ComponentStatuses = %#v, want missing component as Unknown", got.ComponentStatuses)
	}
	if got.Err == nil {
		t.Fatal("Err is nil, want missing component error")
	}
}
