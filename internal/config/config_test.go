package config

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestMonitorConfigComponentNames(t *testing.T) {
	tests := []struct {
		name    string
		monitor MonitorConfig
		want    []string
	}{
		{"singular", MonitorConfig{Component: " API "}, []string{"API"}},
		{"plural takes precedence", MonitorConfig{Component: "old", Components: []string{"API", "Web"}}, []string{"API", "Web"}},
		{"empty plural falls back", MonitorConfig{Component: "API", Components: []string{}}, []string{"API"}},
		{"none means page level", MonitorConfig{Components: []string{"", "none"}}, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.monitor.ComponentNames(); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ComponentNames() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestDefaultConfigUsesPluralComponents(t *testing.T) {
	cfg := DefaultConfig()
	wants := [][]string{
		{"none"},
		{"none"},
		{"none"},
		{"API Service", "Web Chat Service"},
		nil,
	}
	for i, monitor := range cfg.Monitors {
		if monitor.Component != "" {
			t.Fatalf("monitor %q uses legacy component %q", monitor.Name, monitor.Component)
		}
		if !reflect.DeepEqual(monitor.Components, wants[i]) {
			t.Fatalf("monitor %q components = %#v, want %#v", monitor.Name, monitor.Components, wants[i])
		}
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"component":`) {
		t.Fatalf("default config contains legacy component field: %s", data)
	}
}
