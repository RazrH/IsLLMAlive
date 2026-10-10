package diagnostics

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSamplingIsOptInCancelableAndDoesNotOverwrite(t *testing.T) {
	if err := Sample(t.Context(), "", time.Second, func() string { t.Fatal("disabled sample called"); return "" }); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "metrics.jsonl")
	ctx, cancel := context.WithCancel(t.Context())
	if err := Sample(ctx, path, time.Second, func() string { cancel(); return "heap data" }); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	if value["diagnostics"] != "heap data" {
		t.Fatal("sample missing")
	}
	if err := Sample(t.Context(), path, time.Second, func() string { return "overwrite" }); err == nil {
		t.Fatal("overwrote existing metrics")
	}
}
