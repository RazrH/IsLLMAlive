package providers

import "testing"

func TestApiGetProbeURLAppendsModelsPathForHostOnlyEndpoint(t *testing.T) {
	got := apiGetProbeURL("https://api.deepseek.com")
	want := "https://api.deepseek.com/v1/models"

	if got != want {
		t.Fatalf("apiGetProbeURL() = %q, want %q", got, want)
	}
}

func TestApiGetProbeURLKeepsExplicitProbePath(t *testing.T) {
	got := apiGetProbeURL("https://api.z.ai/api/paas/v4/chat/completions")
	want := "https://api.z.ai/api/paas/v4/chat/completions"

	if got != want {
		t.Fatalf("apiGetProbeURL() = %q, want %q", got, want)
	}
}
