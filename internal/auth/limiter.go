package auth

import (
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/hublinks/hublinks/internal/httpx"
)

type attempt struct {
	count               int
	window, lockedUntil time.Time
}

// LoginLimiter bloqueia o login depois de max falhas dentro de window, por
// chave de IP e por e-mail normalizado (o que ocorrer primeiro). O bloqueio dura
// lockout e vale até para a senha correta. Os contadores ficam em memória.
type LoginLimiter struct {
	mu              sync.Mutex
	items           map[string]attempt
	pepper          string
	max             int
	window, lockout time.Duration
	trusted         []netip.Prefix
	now             func() time.Time
	lastSweep       time.Time
}

func NewLoginLimiter(pepper string, max int, window, lockout time.Duration) *LoginLimiter {
	return &LoginLimiter{items: map[string]attempt{}, pepper: pepper, max: max, window: window, lockout: lockout, now: time.Now}
}

// WithTrustedProxies faz o limitador ler o IP real em X-Forwarded-For quando a
// conexão vem de um proxy confiável (TRUSTED_PROXIES). Sem isso, atrás de um
// proxy todos os usuários compartilhariam a mesma chave de IP.
func (l *LoginLimiter) WithTrustedProxies(p []netip.Prefix) *LoginLimiter {
	l.trusted = p
	return l
}

func (l *LoginLimiter) keys(r *http.Request, email string) []string {
	return []string{"ip:" + string(httpx.Key(l.pepper, httpx.ClientIP(r, l.trusted))), "email:" + strings.ToLower(strings.TrimSpace(email))}
}

// current devolve a entrada de k já sem bloqueio vencido.
func (l *LoginLimiter) current(k string, now time.Time) attempt {
	a := l.items[k]
	if !a.lockedUntil.IsZero() && !a.lockedUntil.After(now) {
		a = attempt{}
		delete(l.items, k)
	}
	return a
}

// Blocked informa se o login está bloqueado e por quanto tempo ainda.
func (l *LoginLimiter) Blocked(r *http.Request, email string) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.sweep(now)
	var remaining time.Duration
	for _, k := range l.keys(r, email) {
		if a := l.current(k, now); a.lockedUntil.After(now) && a.lockedUntil.Sub(now) > remaining {
			remaining = a.lockedUntil.Sub(now)
		}
	}
	return remaining, remaining > 0
}

func (l *LoginLimiter) Fail(r *http.Request, email string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	for _, k := range l.keys(r, email) {
		a := l.current(k, now)
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

// Success zera o contador do e-mail (o do IP continua, para não anular o limite
// de quem alterna entre contas).
func (l *LoginLimiter) Success(r *http.Request, email string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.items, "email:"+strings.ToLower(strings.TrimSpace(email)))
}

// sweep descarta, no máximo uma vez por janela, as entradas sem bloqueio ativo e
// com a janela vencida; sem isso o mapa cresceria a cada IP e e-mail tentados.
func (l *LoginLimiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < l.window {
		return
	}
	l.lastSweep = now
	for k, a := range l.items {
		if a.lockedUntil.After(now) {
			continue
		}
		if now.Sub(a.window) > l.window {
			delete(l.items, k)
		}
	}
}

// Tracked devolve quantas chaves estão acompanhadas (para testes).
func (l *LoginLimiter) Tracked() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.items)
}
