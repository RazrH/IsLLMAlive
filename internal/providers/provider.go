package providers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"isllmalive/internal/config"
	"isllmalive/internal/status"
)

// ComponentStatus is the normalized status of one configured component.
type ComponentStatus struct {
	Name    string
	Status  status.Status
	Message string
}

// MonitorResult is the model passed between app, tray, and notify.
type MonitorResult struct {
	Key               string
	Name              string
	Type              string
	Endpoint          string
	Component         string
	Components        []string
	ComponentStatuses []ComponentStatus
	Status            status.Status
	Message           string
	CheckedAt         time.Time
	Err               error
	StatusPage        string
}

func aggregateComponentStatuses(result MonitorResult, components []ComponentStatus) MonitorResult {
	result.ComponentStatuses = components
	result.Status = status.Unknown
	var abnormal []string
	hasUnknown := len(components) == 0
	for _, component := range components {
		hasUnknown = hasUnknown || component.Status == status.Unknown
		if component.Status > result.Status {
			result.Status = component.Status
		}
		if component.Status != status.Normal {
			detail := component.Message
			if detail == "" {
				detail = component.Status.String()
			}
			abnormal = append(abnormal, fmt.Sprintf("%s: %s", component.Name, detail))
		}
	}

	if hasUnknown {
		if result.Status == status.Normal {
			result.Status = status.Unknown
		}
		if result.Err == nil {
			result.Err = fmt.Errorf("one or more selected components are unavailable: %s", strings.Join(abnormal, "; "))
		}
	}
	if len(components) == 0 {
		result.Message = "No interpretable components"
	} else if len(components) == 1 {
		result.Message = components[0].Message
	} else if len(abnormal) == 0 {
		result.Message = "operational"
	} else {
		result.Message = strings.Join(abnormal, "; ")
	}
	return result
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
