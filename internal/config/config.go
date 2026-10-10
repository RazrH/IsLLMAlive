package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// MonitorConfig represents a single monitor's configuration.
type MonitorConfig struct {
	Type       string   `json:"type"`
	Name       string   `json:"name"`
	Enabled    bool     `json:"enabled"`
	Endpoint   string   `json:"endpoint,omitempty"`
	Component  string   `json:"component,omitempty"`
	Components []string `json:"components,omitempty"`
	StatusPage string   `json:"status_page,omitempty"`
	NotifyOn   bool     `json:"notify_on"`
}

// ComponentNames returns the configured component filters. The plural field
// takes precedence while the singular field remains backward compatible.
func (m MonitorConfig) ComponentNames() []string {
	components := m.Components
	if len(components) == 0 {
		components = []string{m.Component}
	}

	result := make([]string, 0, len(components))
	for _, component := range components {
		component = strings.TrimSpace(component)
		if component != "" && !strings.EqualFold(component, "none") {
			result = append(result, component)
		}
	}
	return result
}

type Config struct {
	Language               string          `json:"language"`
	RefreshIntervalMinutes int             `json:"refresh_interval_minutes"`
	GlobalNotifyOn         bool            `json:"global_notify_on"`
	Monitors               []MonitorConfig `json:"monitors"`
}

// DefaultConfig returns the required default configuration.
func DefaultConfig() *Config {
	return &Config{
		Language:               "en-US",
		RefreshIntervalMinutes: 10,
		GlobalNotifyOn:         true,
		Monitors: []MonitorConfig{
			{
				Type:       "openai",
				Name:       "OpenAI",
				Enabled:    true,
				NotifyOn:   true,
				StatusPage: "https://status.openai.com",
				Components: []string{"none"},
			},
			{
				Type:       "statuspage",
				Name:       "Claude",
				Enabled:    true,
				NotifyOn:   true,
				Endpoint:   "https://status.claude.com",
				StatusPage: "https://status.claude.com",
				Components: []string{"none"},
			},
			{
				Type:       "google",
				Name:       "Google AI",
				Enabled:    true,
				NotifyOn:   true,
				StatusPage: "https://aistudio.google.com/status",
				Components: []string{"none"},
			},
			{
				Type:       "deepseek",
				Name:       "DeepSeek",
				Enabled:    true,
				NotifyOn:   true,
				Endpoint:   "https://status.deepseek.com/feed.rss",
				StatusPage: "https://status.deepseek.com",
				Components: []string{"API Service", "Web Chat Service"},
			},
			{
				Type:       "apiget",
				Name:       "Z.ai",
				Enabled:    true,
				NotifyOn:   true,
				Endpoint:   "https://api.z.ai/api/paas/v4/chat/completions",
				StatusPage: "https://z.ai",
			},
		},
	}
}

// Load loads the config.json from the same directory as the executable.
// If it doesn't exist, it creates one with the default configuration.
func Path() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exe), "config.json"), nil
}
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	return LoadFile(path, true)
}
func Reload() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	return LoadFile(path, false)
}
func LoadFile(path string, create bool) (*Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) && create {
		cfg := DefaultConfig()
		data, err = json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return nil, err
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if os.IsExist(err) {
			return LoadFile(path, false)
		}
		if err != nil {
			return nil, err
		}
		_, writeErr := file.Write(data)
		closeErr := file.Close()
		if writeErr != nil {
			return nil, writeErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read configuration: %w", err)
	}
	var cfg *Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("decode configuration: %w", err)
	}
	if cfg == nil {
		return nil, fmt.Errorf("configuration must be an object")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}
func (c *Config) Validate() error {
	for i, m := range c.Monitors {
		if !m.Enabled {
			continue
		}
		if strings.TrimSpace(m.Name) == "" {
			return fmt.Errorf("monitor %d: enabled monitor name is empty", i+1)
		}
		switch m.Type {
		case "openai", "google", "deepseek", "statuspage", "apiget":
		default:
			return fmt.Errorf("monitor %d: unknown type %q", i+1, m.Type)
		}
		if m.Type == "statuspage" || m.Type == "apiget" || (m.Type == "deepseek" && m.Endpoint != "") {
			if err := validateURL(m.Endpoint); err != nil {
				return fmt.Errorf("monitor %d endpoint: %w", i+1, err)
			}
		}
		if m.StatusPage != "" {
			if err := validateURL(m.StatusPage); err != nil {
				return fmt.Errorf("monitor %d status_page: %w", i+1, err)
			}
		}
	}
	return nil
}
func validateURL(value string) error {
	u, err := url.Parse(value)
	if err != nil || u.Hostname() == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
		return fmt.Errorf("expected absolute HTTP(S) URL without credentials")
	}
	return nil
}

// Save writes the current configuration to config.json.
func (c *Config) Save() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	configDir := filepath.Dir(exePath)
	configPath := filepath.Join(configDir, "config.json")

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to save config file: %w", err)
	}

	return nil
}
