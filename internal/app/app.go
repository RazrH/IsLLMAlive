package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"isllmalive/internal/config"
	"isllmalive/internal/notify"
	"isllmalive/internal/providers"
	"isllmalive/internal/status"
	"isllmalive/internal/tray"
)

// App orchestrates the monitoring flow between Config, Providers, and Tray.
type App struct {
	cfg        *config.Config
	mu         sync.Mutex
	cancel     context.CancelFunc
	lastStatus map[string]status.Status
}

// New creates a new App instance.
func New(cfg *config.Config) *App {
	return &App{
		cfg:        cfg,
		lastStatus: make(map[string]status.Status),
	}
}

// Start initiates the background polling ticker and translates the menu.
func (a *App) Start() {
	a.mu.Lock()
	lang := a.cfg.Language
	globalNotify := a.cfg.GlobalNotifyOn
	a.mu.Unlock()
	tray.TranslateMenu(lang)
	tray.UpdateToggleNotifyState(globalNotify, lang)

	a.startPolling()
}

func (a *App) startPolling() {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Stop existing polling if any
	if a.cancel != nil {
		a.cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel

	interval := a.cfg.RefreshIntervalMinutes
	if interval <= 0 {
		interval = 10
	}

	ticker := time.NewTicker(time.Duration(interval) * time.Minute)

	// Trigger first poll
	go a.pollAllLocked()

	go func() {
		for {
			select {
			case <-ticker.C:
				a.mu.Lock()
				a.pollAllLocked()
				a.mu.Unlock()
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

// ReloadConfig is called when the config file changes on disk.
func (a *App) ReloadConfig() {
	newCfg, err := config.Load()
	if err != nil {
		// Do not update a.cfg, but push an error state to the Tray
		errRes := providers.MonitorResult{
			Key:       "config_error",
			Name:      "Config Error",
			Status:    status.Outage,
			Message:   fmt.Sprintf("Invalid JSON: %v", err),
			CheckedAt: time.Now(),
		}
		// Render just the error in tray
		tray.Update([]providers.MonitorResult{errRes}, "en-US") // fallback lang
		return
	}

	a.mu.Lock()
	oldInterval := a.cfg.RefreshIntervalMinutes
	a.cfg = newCfg
	newInterval := a.cfg.RefreshIntervalMinutes
	lang := a.cfg.Language
	globalNotify := a.cfg.GlobalNotifyOn
	a.mu.Unlock()

	tray.TranslateMenu(lang)
	tray.UpdateToggleNotifyState(globalNotify, lang)

	if oldInterval != newInterval {
		// Restart ticker with new interval
		a.startPolling()
	} else {
		// Just poll immediately to reflect new config changes
		a.PollAll()
	}
}

// PollAll is exported for manual triggers (e.g. Refresh menu).
func (a *App) PollAll() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pollAllLocked()
}

// pollAllLocked concurrently fetches the status for all enabled monitors.
func (a *App) pollAllLocked() {
	var wg sync.WaitGroup
	
	// Create a slice with the exact length of configured monitors to preserve ordering
	results := make([]providers.MonitorResult, len(a.cfg.Monitors))

	for i, m := range a.cfg.Monitors {
		if !m.Enabled {
			continue
		}

		prov := providers.NewProvider(m.Type)
		if prov == nil {
			continue
		}

		wg.Add(1)
		go func(idx int, monitor config.MonitorConfig, p providers.Provider) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			res := p.Fetch(ctx, monitor)
			res.StatusPage = monitor.StatusPage // Inject status page URL

			results[idx] = res
		}(i, m, prov)
	}

	wg.Wait()
	
	// Filter out empty results (from disabled or unknown providers)
	var finalResults []providers.MonitorResult
	for i, res := range results {
		if res.Name != "" {
			finalResults = append(finalResults, res)
			
			// Notification Logic
			monitor := a.cfg.Monitors[i]
			prev, exists := a.lastStatus[res.Key]
			
			// Unknown and Normal are treated equally in cross-level checks
			effPrev := prev
			if effPrev == status.Unknown {
				effPrev = status.Normal
			}
			effCurr := res.Status
			if effCurr == status.Unknown {
				effCurr = status.Normal
			}

			if exists && effPrev != effCurr {
				// Cross-level change detected (since Normal/Unknown are same, any difference here is cross-level)
				if a.cfg.GlobalNotifyOn && monitor.NotifyOn {
					title := fmt.Sprintf("IsLLMAlive: %s", res.Name)
					var message string
					
					isZh := false
					if len(a.cfg.Language) >= 2 && (a.cfg.Language[:2] == "zh" || a.cfg.Language[:2] == "Zh") {
						isZh = true
					}

					// Build status string
					statusStr := res.Status.String()
					if isZh {
						switch res.Status {
						case status.Normal:
							statusStr = "已恢复正常"
						case status.Degraded:
							statusStr = "服务降级"
						case status.Outage:
							statusStr = "服务宕机"
						case status.Unknown:
							statusStr = "状态未知"
						}
					} else {
						switch res.Status {
						case status.Normal:
							statusStr = "Recovered (Normal)"
						case status.Degraded:
							statusStr = "Degraded"
						case status.Outage:
							statusStr = "Outage"
						case status.Unknown:
							statusStr = "Unknown"
						}
					}

					if res.Message != "" {
						message = fmt.Sprintf("Status: %s\nDetail: %s", statusStr, res.Message)
					} else {
						message = fmt.Sprintf("Status: %s", statusStr)
					}
					
					_ = notify.Send(title, message)
				}
			}

			// Save last status
			a.lastStatus[res.Key] = res.Status
		}
	}
	
	// tray.Update() handles showing/hiding dynamically, so we don't need to recreate the menu.
	tray.Update(finalResults, a.cfg.Language)
}

// ToggleNotify flips the global notification toggle and saves the config.
func (a *App) ToggleNotify() {
	a.mu.Lock()
	a.cfg.GlobalNotifyOn = !a.cfg.GlobalNotifyOn
	newVal := a.cfg.GlobalNotifyOn
	lang := a.cfg.Language
	err := a.cfg.Save()
	a.mu.Unlock()

	if err != nil {
		fmt.Printf("Failed to save config on toggle: %v\n", err)
	}

	tray.UpdateToggleNotifyState(newVal, lang)
}
