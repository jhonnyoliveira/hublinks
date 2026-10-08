package redirect

import (
	"context"
	"github.com/hublinks/hublinks/internal/domain"
	"sync"
	"time"
)

type Resolver interface {
	Resolve(context.Context, string) (domain.AffiliateLink, error)
	Channel(context.Context, [16]byte, string) ([16]byte, error)
}
type cached struct {
	link    domain.AffiliateLink
	err     error
	expires time.Time
}
type Cache struct {
	mu            sync.RWMutex
	items         map[string]cached
	ttl, negative time.Duration
}

func NewCache() *Cache {
	return &Cache{items: map[string]cached{}, ttl: 5 * time.Minute, negative: 30 * time.Second}
}
func (c *Cache) Resolve(ctx context.Context, code string, fn func(context.Context, string) (domain.AffiliateLink, error)) (domain.AffiliateLink, error) {
	c.mu.RLock()
	v, ok := c.items[code]
	c.mu.RUnlock()
	if ok && time.Now().Before(v.expires) {
		return v.link, v.err
	}
	l, e := fn(ctx, code)
	d := c.ttl
	if e != nil {
		d = c.negative
	}
	c.mu.Lock()
	c.items[code] = cached{l, e, time.Now().Add(d)}
	c.mu.Unlock()
	return l, e
}
func (c *Cache) Invalidate(code string) { c.mu.Lock(); delete(c.items, code); c.mu.Unlock() }
func (c *Cache) InvalidateAll()         { c.mu.Lock(); c.items = map[string]cached{}; c.mu.Unlock() }
