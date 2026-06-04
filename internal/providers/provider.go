package providers

import (
	"context"
	"time"

	"isllmalive/internal/config"
	"isllmalive/internal/status"
)

// MonitorResult is the model passed between app, tray, and notify.
type MonitorResult struct {
	Key        string
	Name       string
	Type       string
	Endpoint   string
	Component  string
	Status     status.Status
	Message    string
	CheckedAt  time.Time
	Err        error
	StatusPage string
}

// Provider represents an interface for fetching the status of a specific monitor type.
type Provider interface {
	Fetch(ctx context.Context, monitor config.MonitorConfig) MonitorResult
}

// NewProvider is a factory method to return the correct Provider based on type.
func NewProvider(providerType string) Provider {
	switch providerType {
	case "openai":
		return &OpenAIProvider{}
	case "statuspage":
		return &StatuspageProvider{}
	case "google":
		return &GoogleProvider{}
	case "deepseek":
		return &DeepSeekProvider{}
	case "apiget":
		return &ApiGetProvider{}
	default:
		// Unknown provider types currently return nil or could return a NoOpProvider
		return nil
	}
}
