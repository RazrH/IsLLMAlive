package app

import (
	"context"
	"fmt"
	"isllmalive/internal/config"
	"isllmalive/internal/diagnostics"
	"isllmalive/internal/notify"
	"isllmalive/internal/providers"
	"isllmalive/internal/status"
	"isllmalive/internal/tray"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type App struct {
	closers          []func()
	configPath       string
	engine           *providers.Engine
	mu               sync.Mutex
	viewMu           sync.Mutex
	cfg              *config.Config
	ctx              context.Context
	cancel           context.CancelFunc
	roundCancel      context.CancelFunc
	startOnce        sync.Once
	wg               sync.WaitGroup
	manual           chan struct{}
	reload           chan struct{}
	generation       uint64
	notifications    *notify.Queue
	lastStatus       map[string]status.Status
	lastResults      []providers.MonitorResult
	lastPollDuration time.Duration
	lastPollRequests uint64
	configError      string
	fetch            func(context.Context, config.MonitorConfig) providers.MonitorResult
	render           func([]providers.MonitorResult, string)
	settings         func(*config.Config)
}

func cloneConfig(cfg *config.Config) *config.Config {
	out := *cfg
	out.Monitors = append([]config.MonitorConfig(nil), cfg.Monitors...)
	for i := range out.Monitors {
		out.Monitors[i].Components = append([]string(nil), cfg.Monitors[i].Components...)
	}
	return &out
}
func New(cfg *config.Config) *App {
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{cfg: cloneConfig(cfg), ctx: ctx, cancel: cancel, manual: make(chan struct{}, 1), reload: make(chan struct{}, 1), lastStatus: make(map[string]status.Status), generation: 1}
	a.engine = providers.NewEngine()
	a.notifications = notify.NewQueue(ctx, notify.Send)
	a.render = tray.Update
	a.settings = func(c *config.Config) {
		tray.TranslateMenu(c.Language)
		tray.UpdateToggleNotifyState(c.GlobalNotifyOn, c.Language)
	}
	return a
}
func (a *App) Start() {
	a.startOnce.Do(func() {
		a.mu.Lock()
		if a.ctx.Err() != nil {
			a.mu.Unlock()
			return
		}
		a.wg.Add(1)
		a.mu.Unlock()
		a.viewMu.Lock()
		a.mu.Lock()
		cfg := cloneConfig(a.cfg)
		a.mu.Unlock()
		a.settings(cfg)
		a.viewMu.Unlock()
		go a.coordinate()
	})
}
func (a *App) Stop() {
	a.mu.Lock()
	a.cancel()
	if a.roundCancel != nil {
		a.roundCancel()
	}
	closers := a.closers
	a.closers = nil
	a.mu.Unlock()
	for _, close := range closers {
		close()
	}
	a.notifications.Stop()
	a.wg.Wait()
	a.engine.Close()
}
func (a *App) StopNotifications() { a.notifications.Stop() }

// PollAll schedules work without blocking the UI. Repeated requests merge.
func (a *App) PollAll() {
	select {
	case a.manual <- struct{}{}:
	default:
	}
}
func (a *App) ReloadConfig() {
	a.mu.Lock()
	path := a.configPath
	a.mu.Unlock()
	var cfg *config.Config
	var err error
	if path == "" {
		cfg, err = config.Reload()
	} else {
		cfg, err = config.LoadFile(path, false)
	}
	if err != nil {
		a.reportConfigError(err)
		return
	}
	a.applyConfig(cfg)
}
func (a *App) applyConfig(cfg *config.Config) {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	a.mu.Lock()
	if a.ctx.Err() != nil {
		a.mu.Unlock()
		return
	}
	a.generation++
	if a.roundCancel != nil {
		a.roundCancel()
	}
	a.reconcileLocked(cfg)
	a.cfg = cloneConfig(cfg)
	a.configError = ""
	current := cloneConfig(a.cfg)
	results := append([]providers.MonitorResult(nil), a.lastResults...)
	a.mu.Unlock()
	a.notifications.Clear()
	a.engine.Invalidate()
	a.settings(current)
	a.render(visibleResults(results), current.Language)
	select {
	case a.reload <- struct{}{}:
	default:
	}
}

type roundResult struct {
	generation uint64
	cfg        *config.Config
	results    []providers.MonitorResult
	duration   time.Duration
	requests   uint64
}

func interval(cfg *config.Config) time.Duration {
	minutes := cfg.RefreshIntervalMinutes
	if minutes <= 0 {
		minutes = 10
	}
	return time.Duration(minutes) * time.Minute
}
func (a *App) coordinate() {
	defer a.wg.Done()
	completed := make(chan roundResult, 1)
	a.mu.Lock()
	period := interval(a.cfg)
	a.mu.Unlock()
	ticker := time.NewTicker(period)
	defer ticker.Stop()
	running, pending := false, false
	start := func() {
		a.mu.Lock()
		cfg := cloneConfig(a.cfg)
		version := a.generation
		ctx, cancel := context.WithCancel(a.ctx)
		a.roundCancel = cancel
		a.mu.Unlock()
		running = true
		go func() { defer cancel(); completed <- a.fetchRound(ctx, cfg, version) }()
	}
	start()
	for {
		select {
		case <-a.ctx.Done():
			if running {
				<-completed
			}
			return
		case <-a.manual:
			if running {
				pending = true
			} else {
				start()
			}
		case <-a.reload:
			a.mu.Lock()
			period = interval(a.cfg)
			a.mu.Unlock()
			ticker.Reset(period)
			if running {
				pending = true
			} else {
				start()
			}
		case <-ticker.C:
			if !running {
				start()
			}
		case round := <-completed:
			running = false
			a.publish(round)
			if pending {
				select {
				case <-a.manual:
				default:
				}
				pending = false
				start()
			}
		}
	}
}
func (a *App) fetchRound(ctx context.Context, cfg *config.Config, version uint64) roundResult {
	started := time.Now()
	var requests atomic.Uint64
	parent := providers.WithRequestCounter(ctx, &requests)
	round := a.engine.NewRound(parent, cfg.Monitors)
	fetch := a.fetch
	if fetch == nil {
		fetch = round.Fetch
	}
	results := make([]providers.MonitorResult, len(cfg.Monitors))
	keys := providers.MonitorKeys(cfg.Monitors)
	var wg sync.WaitGroup
	for i, m := range cfg.Monitors {
		if !m.Enabled {
			continue
		}
		wg.Go(func() {
			child, cancel := context.WithTimeout(parent, 15*time.Second)
			defer cancel()
			results[i] = providers.CompleteResult(m, keys[i], fetch(child, m))
		})
	}
	wg.Wait()
	return roundResult{version, cfg, results, time.Since(started), requests.Load()}
}
func visibleResults(results []providers.MonitorResult) []providers.MonitorResult {
	out := make([]providers.MonitorResult, 0, len(results))
	for _, r := range results {
		if r.Name != "" {
			out = append(out, r)
		}
	}
	return out
}
func (a *App) publish(round roundResult) {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	a.mu.Lock()
	if a.ctx.Err() != nil || round.generation != a.generation {
		a.mu.Unlock()
		return
	}
	a.lastResults = round.results
	a.lastPollDuration = round.duration
	a.lastPollRequests = round.requests
	var messages []notify.Message
	for i, r := range round.results {
		if r.Name == "" {
			continue
		}
		prev, exists := a.lastStatus[r.Key]
		before, after := prev, r.Status
		if before == status.Unknown {
			before = status.Normal
		}
		if after == status.Unknown {
			after = status.Normal
		}
		if exists && before != after && a.cfg.GlobalNotifyOn && round.cfg.Monitors[i].NotifyOn {
			messages = append(messages, notification(r, a.cfg.Language))
		}
		a.lastStatus[r.Key] = r.Status
	}
	lang := a.cfg.Language
	configError := a.configError
	a.mu.Unlock()
	for _, m := range messages {
		a.notifications.Enqueue(m)
	}
	a.render(withConfigError(visibleResults(round.results), configError), lang)
}
func notification(r providers.MonitorResult, lang string) notify.Message {
	label := r.Status.String()
	if strings.HasPrefix(strings.ToLower(lang), "zh") {
		switch r.Status {
		case status.Normal:
			label = "已恢复正常"
		case status.Degraded:
			label = "服务降级"
		case status.Outage:
			label = "服务宕机"
		case status.Unknown:
			label = "状态未知"
		}
	} else if r.Status == status.Normal {
		label = "Recovered (Normal)"
	}
	body := "Status: " + label
	if r.Message != "" {
		body += "\nDetail: " + r.Message
	}
	return notify.Message{Key: r.Key, Title: "IsLLMAlive: " + r.Name, Body: body}
}
func resolveStatusPage(m config.MonitorConfig, page string) string {
	if m.StatusPage != "" {
		return m.StatusPage
	}
	return page
}
func (a *App) ToggleNotify() {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	a.mu.Lock()
	next := cloneConfig(a.cfg)
	next.GlobalNotifyOn = !next.GlobalNotifyOn
	a.cfg = next
	a.mu.Unlock()
	if !next.GlobalNotifyOn {
		a.notifications.Clear()
	}
	if err := next.Save(); err != nil {
		a.mu.Lock()
		a.configError = err.Error()
		a.mu.Unlock()
	}
	a.settings(next)
}

// Diagnostics returns a text snapshot of the current config and latest poll results.
func (a *App) Diagnostics() string {
	a.mu.Lock()
	defer a.mu.Unlock()

	var b strings.Builder
	fmt.Fprintf(&b, "IsLLMAlive diagnostics\n")
	b.WriteString(diagnostics.RuntimeSummary())
	fmt.Fprintf(&b, "Configuration error: %s\n", a.configError)
	fmt.Fprintf(&b, "Last notification error: %s\n", a.notifications.LastError())
	fmt.Fprintf(&b, "Last poll duration: %s\nLast poll HTTP requests: %d\n", a.lastPollDuration, a.lastPollRequests)
	fmt.Fprintf(&b, "Generated: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(&b, "Language: %s\n", a.cfg.Language)
	fmt.Fprintf(&b, "Refresh interval minutes: %d\n", a.cfg.RefreshIntervalMinutes)
	fmt.Fprintf(&b, "Global notifications: %t\n", a.cfg.GlobalNotifyOn)
	fmt.Fprintf(&b, "\n")

	for i, monitor := range a.cfg.Monitors {
		fmt.Fprintf(&b, "Monitor %d\n", i+1)
		fmt.Fprintf(&b, "  config.name: %s\n", monitor.Name)
		fmt.Fprintf(&b, "  config.type: %s\n", monitor.Type)
		fmt.Fprintf(&b, "  config.enabled: %t\n", monitor.Enabled)
		fmt.Fprintf(&b, "  config.endpoint: %s\n", monitor.Endpoint)
		fmt.Fprintf(&b, "  config.component: %s\n", monitor.Component)
		fmt.Fprintf(&b, "  config.components: %s\n", strings.Join(monitor.Components, ", "))
		fmt.Fprintf(&b, "  config.status_page: %s\n", monitor.StatusPage)
		fmt.Fprintf(&b, "  config.notify_on: %t\n", monitor.NotifyOn)

		if i >= len(a.lastResults) || a.lastResults[i].Name == "" {
			fmt.Fprintf(&b, "  result: no latest result\n\n")
			continue
		}

		res := a.lastResults[i]
		fmt.Fprintf(&b, "  result.name: %s\n", res.Name)
		fmt.Fprintf(&b, "  result.type: %s\n", res.Type)
		fmt.Fprintf(&b, "  result.endpoint: %s\n", res.Endpoint)
		fmt.Fprintf(&b, "  result.component: %s\n", res.Component)
		fmt.Fprintf(&b, "  result.components: %s\n", strings.Join(res.Components, ", "))
		for _, component := range res.ComponentStatuses {
			fmt.Fprintf(&b, "  result.component_status: %s | %s | %s\n", component.Name, component.Status, component.Message)
		}
		fmt.Fprintf(&b, "  result.status: %s\n", res.Status)
		fmt.Fprintf(&b, "  result.message: %s\n", res.Message)
		fmt.Fprintf(&b, "  result.checked_at: %s\n", res.CheckedAt.Format(time.RFC3339))
		fmt.Fprintf(&b, "  result.status_page: %s\n", res.StatusPage)
		if res.Err != nil {
			fmt.Fprintf(&b, "  result.error: %v\n", res.Err)
		} else {
			fmt.Fprintf(&b, "  result.error: <nil>\n")
		}
		fmt.Fprintf(&b, "\n")
	}

	return b.String()
}

func (a *App) reconcileLocked(cfg *config.Config) {
	keys := providers.MonitorKeys(cfg.Monitors)
	previous := make(map[string]providers.MonitorResult, len(a.lastResults))
	for _, r := range a.lastResults {
		previous[r.Key] = r
	}
	next := make([]providers.MonitorResult, len(keys))
	statuses := make(map[string]status.Status, len(keys))
	for i, key := range keys {
		if !cfg.Monitors[i].Enabled {
			continue
		}
		if old, ok := previous[key]; ok {
			next[i] = providers.CompleteResult(cfg.Monitors[i], key, old)
		}
		if old, ok := a.lastStatus[key]; ok {
			statuses[key] = old
		}
	}
	a.lastResults = next
	a.lastStatus = statuses
}

func (a *App) WatchConfig(path string) error {
	stop, err := config.Watch(a.ctx, path, a.ReloadConfig, a.reportConfigError)
	if err != nil {
		a.mu.Lock()
		a.configError = err.Error()
		a.mu.Unlock()
		return err
	}
	a.mu.Lock()
	a.configPath = path
	if a.ctx.Err() != nil {
		a.mu.Unlock()
		stop()
		return a.ctx.Err()
	}
	a.closers = append(a.closers, stop)
	a.mu.Unlock()
	return nil
}

func withConfigError(results []providers.MonitorResult, message string) []providers.MonitorResult {
	if message != "" {
		results = append(results, providers.MonitorResult{Key: "configuration-error", Name: "Config Error", Status: status.Unknown, Message: message, CheckedAt: time.Now()})
	}
	return results
}
func (a *App) reportConfigError(err error) {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	a.mu.Lock()
	if a.ctx.Err() != nil {
		a.mu.Unlock()
		return
	}
	a.configError = err.Error()
	results := visibleResults(a.lastResults)
	lang := a.cfg.Language
	a.mu.Unlock()
	a.render(withConfigError(results, err.Error()), lang)
}
