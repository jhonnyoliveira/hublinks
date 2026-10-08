package httpx

import (
	"net/http"
	"sync"
	"time"
)

type rateEntry struct {
	window time.Time
	count  int
	seen   time.Time
}
type RateLimiter struct {
	mu        sync.Mutex
	items     map[string]*rateEntry
	perMinute int
	pepper    string
}

func NewRateLimiter(pepper string, perMinute int) *RateLimiter {
	return &RateLimiter{items: map[string]*rateEntry{}, pepper: pepper, perMinute: perMinute}
}
func (l *RateLimiter) Allow(r *http.Request) bool {
	key := string(Key(l.pepper, ClientIP(r, nil)))
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	e := l.items[key]
	if e == nil || now.Sub(e.window) >= time.Minute {
		e = &rateEntry{window: now}
		l.items[key] = e
	}
	e.seen = now
	e.count++
	for k, v := range l.items {
		if now.Sub(v.seen) > 10*time.Minute {
			delete(l.items, k)
		}
	}
	return e.count <= l.perMinute
}
func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.Allow(r) {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "muitas requisições", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
