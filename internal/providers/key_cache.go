package providers

import (
	"context"
	"strings"
	"sync"
	"time"
)

type googleKeyCache struct {
	mu         sync.Mutex
	key        string
	expires    time.Time
	generation uint64
	loading    chan struct{}
}

func (c *googleKeyCache) invalidate() {
	c.mu.Lock()
	c.key = ""
	c.expires = time.Time{}
	c.generation++
	c.mu.Unlock()
}
func (c *googleKeyCache) get(ctx context.Context, load func() (string, error)) (string, error) {
	for {
		c.mu.Lock()
		if c.key != "" && time.Now().Before(c.expires) {
			key := c.key
			c.mu.Unlock()
			return key, nil
		}
		if loading := c.loading; loading != nil {
			c.mu.Unlock()
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-loading:
			}
			continue
		}
		done := make(chan struct{})
		c.loading = done
		generation := c.generation
		c.mu.Unlock()
		key, err := load()
		c.mu.Lock()
		if err == nil && generation == c.generation {
			c.key = strings.Clone(key)
			c.expires = time.Now().Add(24 * time.Hour)
		}
		c.loading = nil
		close(done)
		c.mu.Unlock()
		return key, err
	}
}
