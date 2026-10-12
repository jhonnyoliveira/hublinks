package httpx

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

func newLimiter(perMinute int) (*RateLimiter, *fakeClock) {
	c := &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	l := NewRateLimiter("12345678901234567890123456789012", perMinute)
	l.now = c.now
	return l, c
}

func get(l http.Handler, ip string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, "/abc1234", nil)
	r.RemoteAddr = ip + ":5000"
	w := httptest.NewRecorder()
	l.ServeHTTP(w, r)
	return w
}

func TestRateLimit300PerMinutePerIP(t *testing.T) {
	l, _ := newLimiter(300)
	var served atomic.Int32
	h := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { served.Add(1) }))
	for i := 1; i <= 300; i++ {
		if w := get(h, "198.51.100.8"); w.Code != http.StatusOK {
			t.Fatalf("requisição %d recusada: %d", i, w.Code)
		}
	}
	w := get(h, "198.51.100.8")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("a 301ª deveria receber 429: %d", w.Code)
	}
	secs, err := strconv.Atoi(w.Header().Get("Retry-After"))
	if err != nil || secs < 1 || secs > 60 {
		t.Fatalf("Retry-After = %q", w.Header().Get("Retry-After"))
	}
	if w.Header().Get("Set-Cookie") != "" || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Location") != "" {
		t.Fatalf("cabeçalhos do 429: %v", w.Header())
	}
	// o excedente não chega ao handler (não redireciona nem gera evento)
	for i := 0; i < 50; i++ {
		get(h, "198.51.100.8")
	}
	if served.Load() != 300 {
		t.Fatalf("o handler atendeu %d requisições (esperado 300)", served.Load())
	}
	// outro IP não é afetado
	if w := get(h, "198.51.100.9"); w.Code != http.StatusOK {
		t.Fatalf("outro IP: %d", w.Code)
	}
}

func TestRateLimitRecoversAfterTheWindow(t *testing.T) {
	l, c := newLimiter(10)
	h := l.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	for i := 0; i < 10; i++ {
		get(h, "198.51.100.8")
	}
	if get(h, "198.51.100.8").Code != http.StatusTooManyRequests {
		t.Fatal("deveria estar limitado")
	}
	c.t = c.t.Add(30 * time.Second)
	if get(h, "198.51.100.8").Code != http.StatusTooManyRequests {
		t.Fatal("meia janela depois, as 10 requisições ainda pesam metade na janela deslizante")
	}
	c.t = c.t.Add(31 * time.Second)
	// 61 s depois a janela anterior ainda pesa quase por inteiro (aproximação conservadora);
	// o Retry-After diz quanto falta e, esperando esse tempo, a cota reabre.
	w := get(h, "198.51.100.8")
	if w.Code != http.StatusTooManyRequests {
		t.Fatal("logo após a virada da janela o peso da anterior ainda bloqueia")
	}
	wait, _ := strconv.Atoi(w.Header().Get("Retry-After"))
	if wait < 1 || wait > 15 {
		t.Fatalf("Retry-After deveria ser curto aqui: %d", wait)
	}
	c.t = c.t.Add(time.Duration(wait) * time.Second)
	if get(h, "198.51.100.8").Code != http.StatusOK {
		t.Fatalf("esperando o Retry-After (%ds) a cota deveria reabrir", wait)
	}
	c.t = c.t.Add(3 * time.Minute)
	for i := 0; i < 10; i++ {
		if get(h, "198.51.100.8").Code != http.StatusOK {
			t.Fatal("depois de ociosidade longa a cota é cheia")
		}
	}
}

func TestRateLimitRetryAfterShrinksAsTimePasses(t *testing.T) {
	l, c := newLimiter(5)
	h := l.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	for i := 0; i < 5; i++ {
		get(h, "198.51.100.8")
	}
	first, _ := strconv.Atoi(get(h, "198.51.100.8").Header().Get("Retry-After"))
	c.t = c.t.Add(40 * time.Second)
	later, _ := strconv.Atoi(get(h, "198.51.100.8").Header().Get("Retry-After"))
	if later >= first {
		t.Fatalf("Retry-After deveria diminuir: %d depois de %d", later, first)
	}
}

func TestRateLimitUsesTrustedProxyClientIP(t *testing.T) {
	proxy := netip.MustParsePrefix("10.0.0.0/8")
	via := func(h http.Handler, client string) int {
		r := httptest.NewRequest(http.MethodGet, "/abc1234", nil)
		r.RemoteAddr = "10.0.0.5:5000"
		r.Header.Set("X-Forwarded-For", client)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	// sem proxy confiável, o segundo cliente cai no mesmo balde do proxy
	l, _ := newLimiter(2)
	h := l.Middleware(next)
	via(h, "198.51.100.1")
	via(h, "198.51.100.1")
	if via(h, "198.51.100.2") != http.StatusTooManyRequests {
		t.Fatal("sem proxy confiável, a chave é o IP do proxy")
	}
	// confiando no proxy, cada cliente tem a sua cota
	l, _ = newLimiter(2)
	h = l.WithTrustedProxies([]netip.Prefix{proxy}).Middleware(next)
	via(h, "198.51.100.1")
	via(h, "198.51.100.1")
	if via(h, "198.51.100.1") != http.StatusTooManyRequests || via(h, "198.51.100.2") != http.StatusOK {
		t.Fatal("cada cliente atrás do proxy confiável deveria ter a própria cota")
	}
}

func TestRateLimitSweepsIdleKeys(t *testing.T) {
	l, c := newLimiter(10)
	h := l.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	for i := 0; i < 100; i++ {
		get(h, "198.51.100."+strconv.Itoa(i))
	}
	if l.Tracked() != 100 {
		t.Fatalf("acompanhados: %d", l.Tracked())
	}
	c.t = c.t.Add(5 * time.Minute)
	get(h, "203.0.113.1")
	if l.Tracked() != 1 {
		t.Fatalf("chaves ociosas deveriam sair do mapa: %d", l.Tracked())
	}
}

func TestRateLimitIsSafeForConcurrentUse(t *testing.T) {
	l := NewRateLimiter("12345678901234567890123456789012", 100)
	h := l.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	var wg sync.WaitGroup
	var ok atomic.Int32
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				if get(h, "198.51.100.8").Code == http.StatusOK {
					ok.Add(1)
				}
			}
		}()
	}
	wg.Wait()
	if ok.Load() != 100 {
		t.Fatalf("exatamente 100 requisições deveriam passar: %d", ok.Load())
	}
}
