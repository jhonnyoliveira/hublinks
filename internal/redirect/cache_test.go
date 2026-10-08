package redirect

import (
	"context"
	"github.com/hublinks/hublinks/internal/domain"
	"testing"
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
