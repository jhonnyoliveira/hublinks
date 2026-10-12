package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hublinks/hublinks/internal/config"
)

// seedLink cria marketplace, canal "wapp" e um link ativo; devolve o código curto.
func seedLink(t *testing.T, a *app) string {
	t.Helper()
	a.do(http.MethodPost, "/admin/marketplaces", true, url.Values{"name": {"Loja"}, "shorten_policy": {"shorten"}}, true)
	w := a.do(http.MethodGet, "/admin/links/new", true, nil, false)
	mp := regexp.MustCompile(`<option value="([0-9a-f-]{36})">Loja</option>`).FindStringSubmatch(w.Body.String())[1]
	a.do(http.MethodPost, "/admin/channels", true, url.Values{"name": {"WhatsApp"}, "segment": {"wapp"}}, true)
	w = a.do(http.MethodPost, "/admin/links", true, url.Values{"title": {"Produto"}, "destination_url": {"https://example.com/p"}, "marketplace_id": {mp}, "active": {"true"}}, false)
	w = a.do(http.MethodGet, w.Header().Get("Location"), true, nil, false)
	return regexp.MustCompile(`https://hub\.example/([0-9a-z]{7})`).FindStringSubmatch(w.Body.String())[1]
}

func publicGet(a *app, path, ua, ip string, withSession bool) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.RemoteAddr = ip + ":4000"
	if ua != "" {
		r.Header.Set("User-Agent", ua)
	}
	if withSession {
		r.AddCookie(a.cookie)
	}
	w := httptest.NewRecorder()
	a.h.ServeHTTP(w, r)
	return w
}

// Nenhuma resposta pública define cookie, com ou sem sessão de administrador, e
// toda página HTML pública traz o link para /privacidade.
func TestPublicResponsesNeverSetCookies(t *testing.T) {
	a := newAppCfg(t, "test", func(c *config.Config) { c.PublicRateLimit = 8 })
	code := seedLink(t, a)
	const browser = "Mozilla/5.0 (iPhone) Safari/604.1"
	cases := []struct {
		name, path, ua string
		status         int
		html           bool
	}{
		{"redirecionamento curto", "/" + code, browser, http.StatusFound, false},
		{"redirecionamento por canal", "/wapp/" + code, browser, http.StatusFound, false},
		{"prévia de crawler", "/" + code, "WhatsApp/2.23", http.StatusOK, true},
		{"404 de código", "/zzzzzzz", browser, http.StatusNotFound, true},
		{"404 de canal", "/nao/" + code, browser, http.StatusNotFound, true},
		{"privacidade", "/privacidade", browser, http.StatusOK, true},
		{"saúde", "/healthz", browser, http.StatusOK, false},
		{"estático", "/static/js/panel.js", browser, http.StatusOK, false},
	}
	for _, withSession := range []bool{false, true} {
		for i, c := range cases {
			// um IP por rodada e por caso, para não esbarrar no limite de taxa
			ip := "198.51.100." + string(rune('1'+i))
			if withSession {
				ip = "203.0.113." + string(rune('1'+i))
			}
			w := publicGet(a, c.path, c.ua, ip, withSession)
			if w.Code != c.status {
				t.Fatalf("%s (sessão=%v): status %d, esperado %d", c.name, withSession, w.Code, c.status)
			}
			if sc := w.Header().Values("Set-Cookie"); len(sc) != 0 {
				t.Errorf("%s (sessão=%v): Set-Cookie %v", c.name, withSession, sc)
			}
			if c.html && !strings.Contains(w.Body.String(), `href="/privacidade"`) {
				t.Errorf("%s (sessão=%v): sem link para /privacidade", c.name, withSession)
			}
		}
		// 429: esgota a cota de um IP e examina a recusa
		ip := "192.0.2.50"
		if withSession {
			ip = "192.0.2.51"
		}
		var last *httptest.ResponseRecorder
		for i := 0; i < 9; i++ {
			last = publicGet(a, "/"+code, browser, ip, withSession)
		}
		if last.Code != http.StatusTooManyRequests || last.Header().Get("Retry-After") == "" || len(last.Header().Values("Set-Cookie")) != 0 || last.Header().Get("Location") != "" {
			t.Fatalf("429 (sessão=%v): %d %v", withSession, last.Code, last.Header())
		}
	}
}

// O limite de taxa vale só para o redirecionamento público: saúde, estáticos e
// privacidade continuam respondendo, e o 429 não gera evento.
func TestRateLimitAppliesToRedirectsOnly(t *testing.T) {
	a := newAppCfg(t, "test", func(c *config.Config) { c.PublicRateLimit = 3 })
	code := seedLink(t, a)
	for i := 0; i < 3; i++ {
		if w := publicGet(a, "/"+code, "Mozilla/5.0", "198.51.100.8", false); w.Code != http.StatusFound {
			t.Fatalf("requisição %d: %d", i+1, w.Code)
		}
	}
	if w := publicGet(a, "/"+code, "Mozilla/5.0", "198.51.100.8", false); w.Code != http.StatusTooManyRequests {
		t.Fatalf("a 4ª deveria receber 429: %d", w.Code)
	}
	for _, path := range []string{"/healthz", "/privacidade", "/static/js/panel.js"} {
		for i := 0; i < 10; i++ {
			if w := publicGet(a, path, "Mozilla/5.0", "198.51.100.8", false); w.Code != http.StatusOK {
				t.Fatalf("%s não deveria sofrer o limite de taxa: %d", path, w.Code)
			}
		}
	}
	// só os 3 acessos aceitos viram eventos
	var raw map[string]any
	deadline := time.Now().Add(2 * time.Second)
	for {
		w := a.do(http.MethodGet, "/api/v1/stats/summary?period=7d", true, nil, false)
		_ = json.Unmarshal(w.Body.Bytes(), &raw)
		if raw["clicks"] == float64(3) || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if raw["clicks"] != float64(3) {
		t.Fatalf("o 429 não pode gerar evento: %v", raw)
	}
}
