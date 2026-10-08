package redirect

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/domain"
)

// maxEntries limita cada mapa: o cache negativo guarda códigos arbitrários
// vindos da internet e não pode crescer sem limite.
const maxEntries = 10000

type cached struct {
	link    domain.AffiliateLink
	err     error
	expires time.Time
}
type channelKey struct {
	org     uuid.UUID
	segment string
}
type cachedChannel struct {
	id      uuid.UUID
	err     error
	expires time.Time
}

// Cache guarda a resolução de código → link e (org, segmento) → canal.
// Somente "não encontrado" é cacheado negativamente; qualquer outro erro
// (por exemplo, falha do banco) não é guardado e volta ao chamador.
type Cache struct {
	mu            sync.RWMutex
	items         map[string]cached
	channels      map[channelKey]cachedChannel
	ttl, negative time.Duration
}

func NewCache() *Cache {
	return &Cache{items: map[string]cached{}, channels: map[channelKey]cachedChannel{}, ttl: 5 * time.Minute, negative: 30 * time.Second}
}

func (c *Cache) Resolve(ctx context.Context, code string, fn func(context.Context, string) (domain.AffiliateLink, error)) (domain.AffiliateLink, error) {
	c.mu.RLock()
	v, ok := c.items[code]
	c.mu.RUnlock()
	if ok && time.Now().Before(v.expires) {
		return v.link, v.err
	}
	l, err := fn(ctx, code)
	d := c.ttl
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			return l, err
		}
		d = c.negative
	}
	c.mu.Lock()
	if len(c.items) >= maxEntries {
		c.items = map[string]cached{}
	}
	c.items[code] = cached{l, err, time.Now().Add(d)}
	c.mu.Unlock()
	return l, err
}

func (c *Cache) Channel(ctx context.Context, org uuid.UUID, segment string, fn func(context.Context, uuid.UUID, string) (uuid.UUID, error)) (uuid.UUID, error) {
	key := channelKey{org, segment}
	c.mu.RLock()
	v, ok := c.channels[key]
	c.mu.RUnlock()
	if ok && time.Now().Before(v.expires) {
		return v.id, v.err
	}
	id, err := fn(ctx, org, segment)
	d := c.ttl
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			return id, err
		}
		d = c.negative
	}
	c.mu.Lock()
	if len(c.channels) >= maxEntries {
		c.channels = map[channelKey]cachedChannel{}
	}
	c.channels[key] = cachedChannel{id, err, time.Now().Add(d)}
	c.mu.Unlock()
	return id, err
}

func (c *Cache) Invalidate(code string) { c.mu.Lock(); delete(c.items, code); c.mu.Unlock() }

// InvalidateAll também descarta os canais: renomear, excluir ou restaurar um
// segmento muda quais URLs por canal são válidas.
func (c *Cache) InvalidateAll() {
	c.mu.Lock()
	c.items = map[string]cached{}
	c.channels = map[channelKey]cachedChannel{}
	c.mu.Unlock()
}
