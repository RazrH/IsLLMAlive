package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInvalidReloadKeepsValidConfigAndDoesNotCreateFile(t *testing.T) {
	a := testApp(t)
	path := filepath.Join(t.TempDir(), "config.json")
	a.configPath = path
	original := a.cfg.Monitors[0].Endpoint
	for _, data := range []string{"{", `{"monitors":[{"enabled":true,"type":"unknown","name":"x"}]}`} {
		os.WriteFile(path, []byte(data), 0600)
		a.ReloadConfig()
		if a.cfg.Monitors[0].Endpoint != original || a.configError == "" {
			t.Fatal("invalid reload replaced config or lost diagnostic")
		}
		stored, _ := os.ReadFile(path)
		if string(stored) != data {
			t.Fatal("file overwritten")
		}
	}
	os.Remove(path)
	a.ReloadConfig()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("reload recreated missing file")
	}
	os.WriteFile(path, []byte(`{"language":"en-US","monitors":[{"enabled":true,"type":"apiget","name":"new","endpoint":"https://new.test"}]}`), 0600)
	a.ReloadConfig()
	if a.cfg.Monitors[0].Name != "new" || a.configError != "" {
		t.Fatal("valid reload not applied")
	}
}
