package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MonitorConfig represents a single monitor's configuration.
type MonitorConfig struct {
	Type       string `json:"type"`
	Name       string `json:"name"`
	Enabled    bool   `json:"enabled"`
	Endpoint   string `json:"endpoint,omitempty"`
	Component  string `json:"component,omitempty"`
	StatusPage string `json:"status_page,omitempty"`
	NotifyOn   bool   `json:"notify_on"`
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
				Type:      "openai",
				Name:      "OpenAI",
				Enabled:   true,
				NotifyOn:  true,
				Component: "none",
			},
			{
				Type:       "statuspage",
				Name:       "Claude",
				Enabled:    true,
				NotifyOn:   true,
				Endpoint:   "https://status.claude.com",
				StatusPage: "https://status.claude.com",
				Component:  "none",
			},
			{
				Type:      "google",
				Name:      "Google AI",
				Enabled:   true,
				NotifyOn:  true,
				Component: "none",
			},
			{
				Type:       "apiget",
				Name:       "DeepSeek",
				Enabled:    true,
				NotifyOn:   true,
				Endpoint:   "https://api.deepseek.com",
				StatusPage: "https://status.deepseek.com",
			},
		},
	}
}

// Load loads the config.json from the same directory as the executable.
// If it doesn't exist, it creates one with the default configuration.
func Load() (*Config, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	configDir := filepath.Dir(exePath)
	configPath := filepath.Join(configDir, "config.json")

	// Check if config exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// File does not exist, create it with defaults
		defaultCfg := DefaultConfig()
		data, err := json.MarshalIndent(defaultCfg, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal default config: %w", err)
		}

		if err := os.WriteFile(configPath, data, 0644); err != nil {
			return nil, fmt.Errorf("failed to write default config: %w", err)
		}

		return defaultCfg, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to check config file: %w", err)
	}

	// File exists, read and parse
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
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
