package app

import (
	"testing"

	"isllmalive/internal/config"
)

func TestResolveStatusPageUsesConfiguredURLWhenPresent(t *testing.T) {
	monitor := config.MonitorConfig{StatusPage: "https://example.com/status"}

	got := resolveStatusPage(monitor, "https://provider.example/status")

	if got != monitor.StatusPage {
		t.Fatalf("resolveStatusPage() = %q, want configured URL %q", got, monitor.StatusPage)
	}
}

func TestResolveStatusPageKeepsProviderDefaultWhenConfigIsEmpty(t *testing.T) {
	providerURL := "https://provider.example/status"

	got := resolveStatusPage(config.MonitorConfig{}, providerURL)

	if got != providerURL {
		t.Fatalf("resolveStatusPage() = %q, want provider URL %q", got, providerURL)
	}
}
