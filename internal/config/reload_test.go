package config

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestLoadFileCreationAndValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if _, err := LoadFile(path, false); err == nil {
		t.Fatal("missing reload succeeded")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("reload created file")
	}
	if _, err := LoadFile(path, true); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"null", "{", `{"monitors":[{"enabled":true,"name":"x","type":"unknown"}]}`, `{"monitors":[{"enabled":true,"name":" ","type":"openai"}]}`, `{"monitors":[{"enabled":true,"name":"x","type":"apiget","endpoint":"file:///tmp"}]}`} {
		os.WriteFile(path, []byte(bad), 0600)
		if _, err := LoadFile(path, false); err == nil {
			t.Fatalf("accepted %s", bad)
		}
		data, _ := os.ReadFile(path)
		if string(data) != bad {
			t.Fatal("invalid file overwritten")
		}
	}
}
func TestDirectoryWatchDebouncesAtomicReplacementAndStops(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte("{}"), 0600)
	var calls atomic.Int32
	events := make(chan struct{}, 8)
	stop, err := Watch(t.Context(), path, func() { calls.Add(1); events <- struct{}{} }, func(err error) { t.Error(err) })
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	os.WriteFile(filepath.Join(dir, "unrelated"), []byte("x"), 0600)
	select {
	case <-events:
		t.Fatal("unrelated event")
	case <-time.After(150 * time.Millisecond):
	}
	for range 5 {
		os.WriteFile(path, []byte("{}"), 0600)
	}
	select {
	case <-events:
	case <-time.After(2 * time.Second):
		t.Fatal("no write event")
	}
	if calls.Load() != 1 {
		t.Fatalf("burst callbacks=%d", calls.Load())
	}
	temp := filepath.Join(dir, "replacement")
	os.WriteFile(temp, []byte("{}"), 0600)
	os.Remove(path)
	if err := os.Rename(temp, path); err != nil {
		t.Fatal(err)
	}
	select {
	case <-events:
	case <-time.After(2 * time.Second):
		t.Fatal("atomic replacement lost watcher")
	}
	stop()
	stop()
	before := calls.Load()
	os.WriteFile(path, []byte("{}"), 0600)
	time.Sleep(150 * time.Millisecond)
	if calls.Load() != before {
		t.Fatal("callback after stop")
	}
}
func TestDefaultConfigValid(t *testing.T) {
	if err := DefaultConfig().Validate(); err != nil {
		t.Fatal(err)
	}
}
func TestDisabledDraftAllowed(t *testing.T) {
	c := &Config{Monitors: []MonitorConfig{{Type: "draft", Name: ""}}}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := validateURL("https://user:password@example.test"); err == nil || !strings.Contains(err.Error(), "credentials") {
		t.Fatal("credential URL accepted")
	}
}
