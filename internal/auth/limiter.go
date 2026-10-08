package auth

import (
	"github.com/hublinks/hublinks/internal/httpx"
	"net/http"
	"strings"
	"sync"
	"time"
)

type attempt struct {
	count               int
	window, lockedUntil time.Time
}
type LoginLimiter struct {
	mu              sync.Mutex
	items           map[string]attempt
	pepper          string
	max             int
	window, lockout time.Duration
	now             func() time.Time
}

func NewLoginLimiter(pepper string, max int, window, lockout time.Duration) *LoginLimiter {
	return &LoginLimiter{items: map[string]attempt{}, pepper: pepper, max: max, window: window, lockout: lockout, now: time.Now}
}
func (l *LoginLimiter) keys(r *http.Request, email string) []string {
	return []string{"ip:" + string(httpx.Key(l.pepper, httpx.ClientIP(r, nil))), "email:" + strings.ToLower(strings.TrimSpace(email))}
}
func (l *LoginLimiter) Blocked(r *http.Request, email string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	for _, k := range l.keys(r, email) {
		if l.items[k].lockedUntil.After(now) {
			return true
		}
	}
	return false
}
func (l *LoginLimiter) Fail(r *http.Request, email string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	for _, k := range l.keys(r, email) {
		a := l.items[k]
		if a.window.IsZero() || now.Sub(a.window) > l.window {
			a = attempt{window: now}
		}
		a.count++
		if a.count >= l.max {
			a.lockedUntil = now.Add(l.lockout)
		}
		l.items[k] = a
	}
}
func (l *LoginLimiter) Success(r *http.Request, email string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.items, "email:"+strings.ToLower(strings.TrimSpace(email)))
}
