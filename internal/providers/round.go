package providers

import (
	"context"
	"fmt"
	"isllmalive/internal/config"
	"isllmalive/internal/status"
	"net/http"
	"slices"
	"sync"
	"time"
)

type snapshot interface {
	evaluate(config.MonitorConfig, MonitorResult) MonitorResult
}
type sourceEntry struct {
	done     chan struct{}
	snapshot snapshot
	err      error
}
type Engine struct {
	keys           googleKeyCache
	httpMu         sync.Mutex
	standard, utls *http.Client
}

func NewEngine() *Engine { return &Engine{} }

var defaultEngine = NewEngine()

func (e *Engine) Invalidate() { e.keys.invalidate(); e.CloseIdleConnections() }

type Round struct {
	ctx     context.Context
	engine  *Engine
	mu      sync.Mutex
	entries map[Source]*sourceEntry
	probes  map[Source][]string
}

func (e *Engine) NewRound(ctx context.Context, monitors []config.MonitorConfig) *Round {
	r := &Round{ctx: ctx, engine: e, entries: make(map[Source]*sourceEntry), probes: make(map[Source][]string)}
	for _, m := range monitors {
		if !m.Enabled && len(monitors) > 1 {
			continue
		}
		if m.Type != "deepseek" {
			continue
		}
		source := StateSource(m)
		names := m.ComponentNames()
		if len(names) == 0 {
			names = []string{"api", "web"}
		}
		for _, name := range names {
			target := normalizeDeepSeekComponent(name)
			if target == "" {
				continue
			}
			found := slices.Contains(r.probes[source], target)
			if !found {
				r.probes[source] = append(r.probes[source], target)
			}
		}
	}
	return r
}
func (r *Round) Fetch(ctx context.Context, m config.MonitorConfig) MonitorResult {
	base := CompleteResult(m, "", MonitorResult{Status: status.Unknown, CheckedAt: time.Now()})
	if ctx.Err() != nil {
		return incomplete(base, ctx.Err())
	}
	source := StateSource(m)
	r.mu.Lock()
	entry, exists := r.entries[source]
	if !exists {
		entry = &sourceEntry{done: make(chan struct{})}
		r.entries[source] = entry
	}
	r.mu.Unlock()
	if !exists {
		budget, cancel := context.WithTimeout(r.ctx, 15*time.Second)
		entry.snapshot, entry.err = r.engine.fetchSnapshot(budget, source, r.probes[source])
		cancel()
		close(entry.done)
	}
	select {
	case <-ctx.Done():
		return incomplete(base, ctx.Err())
	case <-entry.done:
		if entry.err != nil {
			return incomplete(base, entry.err)
		}
		if entry.snapshot == nil {
			return incomplete(base, fmt.Errorf("source returned no snapshot"))
		}
		return entry.snapshot.evaluate(m, base)
	}
}
func fetchStandalone(ctx context.Context, m config.MonitorConfig) MonitorResult {
	return defaultEngine.NewRound(ctx, []config.MonitorConfig{m}).Fetch(ctx, m)
}
