package notify

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Message struct{ Key, Title, Body string }
type Sender func(context.Context, string, string) error

// Queue retains at most one pending message per monitor. Only its worker executes commands.
type Queue struct {
	mu           sync.Mutex
	pending      map[string]Message
	order        []string
	wake         chan struct{}
	done         chan struct{}
	ctx          context.Context
	cancel       context.CancelFunc
	send         Sender
	activeCancel context.CancelFunc
	lastError    string
}

func NewQueue(parent context.Context, send Sender) *Queue {
	ctx, cancel := context.WithCancel(parent)
	q := &Queue{pending: make(map[string]Message), wake: make(chan struct{}, 1), done: make(chan struct{}), ctx: ctx, cancel: cancel, send: send}
	go q.run()
	return q
}
func (q *Queue) Enqueue(m Message) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.ctx.Err() != nil {
		return
	}
	if _, ok := q.pending[m.Key]; !ok {
		q.order = append(q.order, m.Key)
	}
	q.pending[m.Key] = m
	select {
	case q.wake <- struct{}{}:
	default:
	}
}
func (q *Queue) run() {
	defer close(q.done)
	for {
		select {
		case <-q.ctx.Done():
			return
		case <-q.wake:
		}
		for {
			q.mu.Lock()
			if len(q.order) == 0 || q.ctx.Err() != nil {
				q.mu.Unlock()
				break
			}
			key := q.order[0]
			q.order = q.order[1:]
			m := q.pending[key]
			delete(q.pending, key)
			ctx, cancel := context.WithTimeout(q.ctx, 5*time.Second)
			q.activeCancel = cancel
			q.mu.Unlock()
			err := q.send(ctx, m.Title, m.Body)
			cancel()
			if err != nil {
				q.mu.Lock()
				q.lastError = fmt.Sprintf("%s: %v", time.Now().Format(time.RFC3339), err)
				q.mu.Unlock()
			}
		}
	}
}
func (q *Queue) LastError() string { q.mu.Lock(); defer q.mu.Unlock(); return q.lastError }
func (q *Queue) Stop() {
	q.cancel()
	<-q.done
	q.mu.Lock()
	if q.activeCancel != nil {
		q.activeCancel()
	}
	clear(q.pending)
	q.order = nil
	q.mu.Unlock()
}

// Clear removes pending messages when configuration invalidates their context.
func (q *Queue) Clear() {
	q.mu.Lock()
	if q.activeCancel != nil {
		q.activeCancel()
	}
	clear(q.pending)
	q.order = nil
	q.mu.Unlock()
}
