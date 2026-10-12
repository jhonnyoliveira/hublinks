package httpx

import (
	"math"
	"net/http"
	"net/netip"
	"strconv"
	"sync"
	"time"
)

// RateLimiter limita as rotas públicas a perMinute requisições por minuto para
// cada chave de IP, com uma janela deslizante aproximada (contador da janela
// atual somado ao da anterior, ponderado pelo tempo decorrido). Até perMinute
// requisições seguidas passam; a seguinte recebe 429. Requisições recusadas não
// consomem cota, então quem espera o Retry-After volta a ser atendido.
type RateLimiter struct {
	mu        sync.Mutex
	items     map[string]*rateEntry
	perMinute int
	pepper    string
	trusted   []netip.Prefix
	now       func() time.Time
	lastSweep time.Time
}

type rateEntry struct {
	start     time.Time // início da janela atual
	cur, prev int
	seen      time.Time
}

func NewRateLimiter(pepper string, perMinute int) *RateLimiter {
	return &RateLimiter{items: map[string]*rateEntry{}, pepper: pepper, perMinute: perMinute, now: time.Now}
}

// WithTrustedProxies lê o IP real em X-Forwarded-For quando a conexão vem de um
// proxy confiável (TRUSTED_PROXIES).
func (l *RateLimiter) WithTrustedProxies(p []netip.Prefix) *RateLimiter {
	l.trusted = p
	return l
}

// Allow informa se a requisição cabe na cota; se não, devolve em quanto tempo tentar de novo.
func (l *RateLimiter) Allow(r *http.Request) (bool, time.Duration) {
	key := string(Key(l.pepper, ClientIP(r, l.trusted)))
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.sweep(now)
	e := l.items[key]
	if e == nil {
		e = &rateEntry{start: now}
		l.items[key] = e
	}
	e.seen = now
	switch elapsed := now.Sub(e.start); {
	case elapsed >= 2*time.Minute:
		e.start, e.cur, e.prev = now, 0, 0
	case elapsed >= time.Minute:
		e.start, e.prev, e.cur = e.start.Add(time.Minute), e.cur, 0
	}
	frac := float64(now.Sub(e.start)) / float64(time.Minute)
	if float64(e.prev)*(1-frac)+float64(e.cur)+1 > float64(l.perMinute) {
		return false, retryAfter(e, now, l.perMinute)
	}
	e.cur++
	return true, 0
}

// retryAfter estima quando a cota volta a abrir: o peso da janela anterior cai
// linearmente até o fim da janela atual.
func retryAfter(e *rateEntry, now time.Time, limit int) time.Duration {
	end := e.start.Add(time.Minute)
	if e.prev > 0 {
		if over := float64(e.prev)*(1-float64(now.Sub(e.start))/float64(time.Minute)) + float64(e.cur) + 1 - float64(limit); over > 0 {
			// cada segundo libera prev/60 requisições
			wait := time.Duration(over / float64(e.prev) * float64(time.Minute))
			if now.Add(wait).Before(end) {
				return wait
			}
		}
	}
	return end.Sub(now)
}

// sweep remove, no máximo uma vez por minuto, as chaves ociosas há mais de duas janelas.
func (l *RateLimiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < time.Minute {
		return
	}
	l.lastSweep = now
	for k, e := range l.items {
		if now.Sub(e.seen) > 2*time.Minute {
			delete(l.items, k)
		}
	}
}

// Tracked devolve quantas chaves estão acompanhadas (para testes).
func (l *RateLimiter) Tracked() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.items)
}

func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ok, wait := l.Allow(r); !ok {
			secs := int(math.Ceil(wait.Seconds()))
			if secs < 1 {
				secs = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(secs))
			w.Header().Set("Cache-Control", "no-store")
			http.Error(w, "muitas requisições", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
