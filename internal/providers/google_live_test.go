package providers

import (
	"context"
	"encoding/json"
	"isllmalive/internal/config"
	"isllmalive/internal/status"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestGoogleLiveShape(t *testing.T) {
	output := os.Getenv("ISLLMALIVE_GOOGLE_RESPONSE_FILE")
	if output == "" {
		t.Skip("explicit public-response capture only")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	client := newStandardClient()
	defer client.CloseIdleConnections()
	key, err := (&GoogleProvider{}).fetchAPIKey(ctx, client)
	if err != nil {
		t.Fatal(err)
	}
	body, _, err := readSource(ctx, client, Source{Type: "google", URL: googleHistoryURL, Method: "POST", Body: "[]"}, http.Header{"Content-Type": {"application/json+protobuf"}, "X-Goog-Api-Key": {key}, "Origin": {"https://aistudio.google.com"}, "Referer": {"https://aistudio.google.com/"}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, body, 0600); err != nil {
		t.Fatal(err)
	}
	t.Log("captured public response bytes", len(body))
}

func TestGoogleCapturedHistoryShape(t *testing.T) {
	data, err := os.ReadFile("testdata/google-history.json")
	if err != nil {
		t.Fatal(err)
	}
	var raw []any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	result := (&GoogleProvider{}).parseData(raw, config.MonitorConfig{}, MonitorResult{})
	if result.Err != nil || result.Status != status.Normal {
		t.Fatalf("captured resolved history: %s %v", result.Status, result.Err)
	}
}
