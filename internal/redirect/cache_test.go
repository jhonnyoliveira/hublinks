package redirect

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/domain"
)

func TestCacheAvoidsDuplicateLookup(t *testing.T) {
	c := NewCache()
	calls := 0
	lookup := func(context.Context, string) (domain.AffiliateLink, error) {
		calls++
		return domain.AffiliateLink{Code: "abc1234"}, nil
	}
	_, _ = c.Resolve(context.Background(), "abc1234", lookup)
	_, _ = c.Resolve(context.Background(), "abc1234", lookup)
	if calls != 1 {
		t.Fatal(calls)
	}
	c.Invalidate("abc1234")
	_, _ = c.Resolve(context.Background(), "abc1234", lookup)
	if calls != 2 {
		t.Fatal(calls)
	}
}

func TestCacheDoesNotStoreInfrastructureErrors(t *testing.T) {
	c := NewCache()
	calls := 0
	boom := errors.New("banco indisponível")
	lookup := func(context.Context, string) (domain.AffiliateLink, error) {
		calls++
		if calls == 1 {
			return domain.AffiliateLink{}, boom
		}
		return domain.AffiliateLink{Code: "abc1234"}, nil
	}
	if _, err := c.Resolve(context.Background(), "abc1234", lookup); !errors.Is(err, boom) {
		t.Fatalf("erro esperado, veio %v", err)
	}
	if l, err := c.Resolve(context.Background(), "abc1234", lookup); err != nil || l.Code != "abc1234" {
		t.Fatalf("a falha não pode ficar em cache: %v", err)
	}
}

func TestCacheNegativeNotFoundAndChannelCache(t *testing.T) {
	c := NewCache()
	calls := 0
	missing := func(context.Context, string) (domain.AffiliateLink, error) {
		calls++
		return domain.AffiliateLink{}, domain.ErrNotFound
	}
	for range 3 {
		_, _ = c.Resolve(context.Background(), "zzzzzzz", missing)
	}
	if calls != 1 {
		t.Fatalf("cache negativo: %d consultas", calls)
	}

	org, ch := uuid.New(), uuid.New()
	chCalls := 0
	lookup := func(context.Context, uuid.UUID, string) (uuid.UUID, error) { chCalls++; return ch, nil }
	for range 3 {
		if id, err := c.Channel(context.Background(), org, "wapp", lookup); err != nil || id != ch {
			t.Fatal(id, err)
		}
	}
	if _, err := c.Channel(context.Background(), uuid.New(), "wapp", lookup); err != nil {
		t.Fatal(err)
	}
	if chCalls != 2 {
		t.Fatalf("canais devem ser isolados por organização: %d consultas", chCalls)
	}
	c.InvalidateAll()
	_, _ = c.Channel(context.Background(), org, "wapp", lookup)
	if chCalls != 3 {
		t.Fatalf("InvalidateAll deve limpar canais: %d consultas", chCalls)
	}
}

func TestCacheIsBounded(t *testing.T) {
	c := NewCache()
	missing := func(context.Context, string) (domain.AffiliateLink, error) {
		return domain.AffiliateLink{}, domain.ErrNotFound
	}
	for i := range maxEntries + 5 {
		_, _ = c.Resolve(context.Background(), fmt.Sprintf("c%d", i), missing)
	}
	if len(c.items) > maxEntries {
		t.Fatalf("cache passou do limite: %d", len(c.items))
	}
}

func TestCacheConcurrentAccess(t *testing.T) {
	c := NewCache()
	lookup := func(context.Context, string) (domain.AffiliateLink, error) { return domain.AffiliateLink{}, nil }
	chLookup := func(context.Context, uuid.UUID, string) (uuid.UUID, error) { return uuid.Nil, nil }
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 200 {
				_, _ = c.Resolve(context.Background(), "abc1234", lookup)
				_, _ = c.Channel(context.Background(), uuid.Nil, "wapp", chLookup)
				if i%2 == 0 {
					c.Invalidate("abc1234")
				} else {
					c.InvalidateAll()
				}
			}
		}()
	}
	wg.Wait()
}
