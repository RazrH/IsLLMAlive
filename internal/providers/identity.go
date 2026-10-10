package providers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"isllmalive/internal/config"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

var googleHistoryURL = "https://alkalimakersuite-pa.clients6.google.com/$rpc/google.internal.alkali.applications.makersuite.v1.MakerSuiteService/ListIncidentsHistory"

type Source struct{ Type, URL, Method, Body string }

// StateSource includes only inputs that affect upstream data.
func StateSource(m config.MonitorConfig) Source {
	s := Source{Type: m.Type, Method: http.MethodGet, URL: strings.TrimSpace(m.Endpoint)}
	switch m.Type {
	case "openai":
		s.URL = openAIEndpoint
	case "google":
		s.URL = googleHistoryURL
		s.Method = http.MethodPost
		s.Body = "[]"
	case "statuspage":
		s.URL = strings.TrimSuffix(s.URL, "/")
		if !strings.HasSuffix(s.URL, "/api/v2/summary.json") {
			s.URL += "/api/v2/summary.json"
		}
	case "deepseek":
		if s.URL == "" {
			s.URL = deepSeekDefaultFeedURL
		}
	case "apiget":
		s.URL = apiGetProbeURL(s.URL)
	}
	return s
}
func hashIdentity(v any) string {
	data, _ := json.Marshal(v)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func monitorBase(m config.MonitorConfig) string {
	components := m.ComponentNames()
	for i, c := range components {
		components[i] = normalizeOAIComponentName(c)
	}
	slices.Sort(components)
	components = slices.Compact(components)
	return hashIdentity(struct {
		Source     Source
		Components []string
		Name       string
	}{StateSource(m), components, m.Name})
}

// MonitorKeys preserves identity through ordering, language and notification changes.
func MonitorKeys(monitors []config.MonitorConfig) []string {
	keys := make([]string, len(monitors))
	counts := make(map[string]int)
	for i, m := range monitors {
		base := monitorBase(m)
		keys[i] = base + ":" + strconv.Itoa(counts[base])
		counts[base]++
	}
	return keys
}
func CompleteResult(m config.MonitorConfig, key string, r MonitorResult) MonitorResult {
	if key == "" {
		key = monitorBase(m) + ":0"
	}
	r.Key = key
	r.Name = m.Name
	r.Type = m.Type
	r.Endpoint = m.Endpoint
	r.Component = m.Component
	r.Components = m.ComponentNames()
	if r.CheckedAt.IsZero() {
		r.CheckedAt = time.Now()
	}
	if m.StatusPage != "" {
		r.StatusPage = m.StatusPage
	}
	if r.StatusPage == "" {
		switch m.Type {
		case "openai":
			r.StatusPage = "https://status.openai.com"
		case "google":
			r.StatusPage = "https://aistudio.google.com/status"
		case "deepseek":
			r.StatusPage = deepSeekDefaultStatusPage
		case "statuspage":
			r.StatusPage = strings.TrimSuffix(StateSource(m).URL, "/api/v2/summary.json")
		case "apiget":
			r.StatusPage = m.Endpoint
		}
	}
	return r
}
