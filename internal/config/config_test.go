package config

import (
	"reflect"
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
