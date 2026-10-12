package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/auth"
	"github.com/hublinks/hublinks/internal/config"
	"github.com/hublinks/hublinks/internal/server"
	"github.com/hublinks/hublinks/internal/store"
	"github.com/hublinks/hublinks/internal/store/testdb"
)

type app struct {
	h       http.Handler
	cookie  *http.Cookie
	csrf    string
	orgID   uuid.UUID
	t       *testing.T
	baseURL string
}

func newApp(t *testing.T) *app { return newAppWithEnv(t, "test") }

func newAppWithEnv(t *testing.T, appEnv string) *app {
	t.Helper()
	pool := testdb.New(t)
	ctx := context.Background()
	if err := store.Bootstrap(ctx, pool, "Org", "admin@example.com", "senha-de-teste-1234"); err != nil {
		t.Fatal(err)
	}
	var user, org uuid.UUID
	if err := pool.QueryRow(ctx, "SELECT user_id, org_id FROM memberships LIMIT 1").Scan(&user, &org); err != nil {
		t.Fatal(err)
	}
	raw, sess, err := auth.Manager{Pool: pool}.Create(ctx, user, org)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		BaseURL: "https://hub.example", Pepper: "pepper-de-teste-com-32-caracteres!", AppEnv: appEnv,
		ReportTZ: time.UTC, TrashRetentionDays: 30, PublicRateLimit: 1000,
		LoginMaxFailures: 5, LoginWindow: time.Minute, LoginLockout: time.Minute,
		EventQueueSize: 100, EventBatchSize: 10, EventFlushInterval: 50 * time.Millisecond,
		BotDistinctCodes: 5, BotWindow: time.Minute,
	}
	return &app{h: server.New(pool, cfg), cookie: &http.Cookie{Name: auth.CookieName, Value: raw}, csrf: sess.CSRF, orgID: org, t: t, baseURL: cfg.BaseURL}
}

// do envia a requisição pelo mux completo. login=true anexa o cookie de sessão; hx
// marca a requisição como HTMX e envia o token CSRF no cabeçalho.
func (a *app) do(method, target string, login bool, form url.Values, hx bool) *httptest.ResponseRecorder {
	a.t.Helper()
	r := httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
	if form != nil {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if login {
		r.AddCookie(a.cookie)
		if method != http.MethodGet {
			r.Header.Set("X-CSRF-Token", a.csrf)
		}
	}
	if hx {
		r.Header.Set("HX-Request", "true")
	}
	w := httptest.NewRecorder()
	a.h.ServeHTTP(w, r)
	return w
}

func TestLiteralRoutesWinOverPublicCodePattern(t *testing.T) {
	a := newApp(t)
	// "healthz" tem sete caracteres [a-z] e casaria com o formato do código curto.
	w := a.do(http.MethodGet, "/healthz", false, nil, false)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"status":"ok"`) {
		t.Fatalf("/healthz: %d %s", w.Code, w.Body.String())
	}
	if w = a.do(http.MethodGet, "/privacidade", false, nil, false); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "rivacidade") {
		t.Fatalf("/privacidade: %d", w.Code)
	}
	for _, path := range []string{"/static/vendor/htmx.min.js", "/static/vendor/alpine.min.js", "/static/js/panel.js", "/static/css/base.css", "/static/fonts/inter-latin-wght-normal.woff2"} {
		if w = a.do(http.MethodGet, path, false, nil, false); w.Code != http.StatusOK || w.Body.Len() == 0 {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
	// a página de login precisa renderizar inteira (o layout lê .CSRF dos dados)
	w = a.do(http.MethodGet, "/admin/login", false, nil, false)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `id="email"`) || !strings.Contains(w.Body.String(), "</html>") || strings.Contains(w.Body.String(), "erro ao renderizar") {
		t.Fatalf("/admin/login: %d\n%s", w.Code, w.Body.String())
	}
}

func TestAdminAndAPIPrefixesNeverReachPublicRedirect(t *testing.T) {
	a := newApp(t)
	for _, path := range []string{"/admin", "/admin/marketplaces", "/admin/links/new", "/admin/lixeira", "/admin/desconhecido", "/admin/a/b/c"} {
		w := a.do(http.MethodGet, path, false, nil, false)
		if w.Code != http.StatusSeeOther || !strings.HasPrefix(w.Header().Get("Location"), "/admin/login") {
			t.Fatalf("%s sem sessão: %d %q", path, w.Code, w.Header().Get("Location"))
		}
	}
	for _, path := range []string{"/api/v1/marketplaces", "/api/v1/channels", "/api/v1/affiliate-links", "/api/v1/desconhecido", "/api/v1/stats/summary"} {
		w := a.do(http.MethodGet, path, false, nil, false)
		var body struct{ Error struct{ Code string } }
		if w.Code != http.StatusUnauthorized || json.Unmarshal(w.Body.Bytes(), &body) != nil || body.Error.Code != "unauthorized" {
			t.Fatalf("%s sem sessão: %d %s", path, w.Code, w.Body.String())
		}
	}
	// com sessão, caminho desconhecido é "não encontrado" no formato da área
	public404 := a.do(http.MethodGet, "/zzzzzzz", false, nil, false).Body.String()
	w := a.do(http.MethodGet, "/admin/desconhecido", true, nil, false)
	if w.Code != http.StatusNotFound || w.Body.String() == public404 {
		t.Fatalf("/admin/desconhecido deve ser o 404 do painel, não a página pública: %d", w.Code)
	}
	w = a.do(http.MethodGet, "/api/v1/desconhecido", true, nil, false)
	if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), `"not_found"`) {
		t.Fatalf("/api/v1/desconhecido: %d %s", w.Code, w.Body.String())
	}
}

func TestAdminWritesRequireCSRF(t *testing.T) {
	a := newApp(t)
	body := url.Values{"name": {"Loja"}, "shorten_policy": {"shorten"}}
	r := httptest.NewRequest(http.MethodPost, "/admin/marketplaces", strings.NewReader(body.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(a.cookie)
	w := httptest.NewRecorder()
	a.h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("sem token CSRF: %d", w.Code)
	}
	if w = a.do(http.MethodPost, "/admin/marketplaces", true, body, true); w.Code != http.StatusOK {
		t.Fatalf("com token: %d %s", w.Code, w.Body.String())
	}
	// origem de outro site é recusada mesmo com token válido
	r = httptest.NewRequest(http.MethodPost, "/admin/marketplaces", strings.NewReader(body.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("X-CSRF-Token", a.csrf)
	r.Header.Set("Origin", "https://evil.example")
	r.AddCookie(a.cookie)
	w = httptest.NewRecorder()
	a.h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("origem externa: %d", w.Code)
	}
}

var linkPath = regexp.MustCompile(`^/admin/links/([0-9a-f-]{36})\?ok=created$`)

// Fluxo da US1 pelo servidor completo: cadastrar marketplace, canal e link no
// painel, divulgar por canal e acessar as URLs públicas.
func TestUserStory1EndToEnd(t *testing.T) {
	a := newApp(t)
	var mpID string
	if w := a.do(http.MethodPost, "/admin/marketplaces", true, url.Values{"name": {"Loja"}, "shorten_policy": {"shorten"}}, true); w.Code != http.StatusOK {
		t.Fatalf("marketplace: %d", w.Code)
	}
	w := a.do(http.MethodGet, "/admin/links/new", true, nil, false)
	if m := regexp.MustCompile(`<option value="([0-9a-f-]{36})">Loja</option>`).FindStringSubmatch(w.Body.String()); m != nil {
		mpID = m[1]
	} else {
		t.Fatalf("marketplace ausente no formulário: %d", w.Code)
	}
	if w = a.do(http.MethodPost, "/admin/channels", true, url.Values{"name": {"WhatsApp"}, "segment": {"wapp"}}, true); w.Code != http.StatusOK {
		t.Fatalf("canal: %d", w.Code)
	}
	w = a.do(http.MethodPost, "/admin/links", true, url.Values{"title": {"Produto"}, "destination_url": {"https://example.com/produto"}, "marketplace_id": {mpID}, "active": {"true"}}, false)
	m := linkPath.FindStringSubmatch(w.Header().Get("Location"))
	if w.Code != http.StatusSeeOther || m == nil {
		t.Fatalf("link: %d %q", w.Code, w.Header().Get("Location"))
	}
	linkID := m[1]
	w = a.do(http.MethodGet, w.Header().Get("Location"), true, nil, false)
	codeRe := regexp.MustCompile(`https://hub\.example/([0-9a-z]{7})`)
	cm := codeRe.FindStringSubmatch(w.Body.String())
	if cm == nil || !strings.Contains(w.Body.String(), "https://hub.example/wapp/"+cm[1]) {
		t.Fatalf("URLs de divulgação ausentes:\n%s", w.Body.String())
	}
	code := cm[1]

	for _, path := range []string{"/" + code, "/wapp/" + code} {
		w = a.do(http.MethodGet, path, false, nil, false)
		if w.Code != http.StatusFound || w.Header().Get("Location") != "https://example.com/produto" || w.Header().Get("Set-Cookie") != "" || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s: %d %v", path, w.Code, w.Header())
		}
	}
	for _, path := range []string{"/outro/" + code, "/zzzzzzz", "/wapp/zzzzzzz", "/curto"} {
		if w = a.do(http.MethodGet, path, false, nil, false); w.Code != http.StatusNotFound {
			t.Fatalf("%s deveria ser 404: %d", path, w.Code)
		}
	}

	// inativo → 404; reativar → 302
	a.do(http.MethodPost, "/admin/links/"+linkID+"/toggle", true, url.Values{"active": {"false"}}, true)
	if w = a.do(http.MethodGet, "/"+code, false, nil, false); w.Code != http.StatusNotFound {
		t.Fatalf("link inativo: %d", w.Code)
	}
	a.do(http.MethodPost, "/admin/links/"+linkID+"/toggle", true, url.Values{"active": {"true"}}, true)
	if w = a.do(http.MethodGet, "/"+code, false, nil, false); w.Code != http.StatusFound {
		t.Fatalf("link reativado: %d", w.Code)
	}

	// lixeira → 404; restaurar → 302
	a.do(http.MethodPost, "/admin/links/"+linkID+"/delete", true, nil, true)
	if w = a.do(http.MethodGet, "/"+code, false, nil, false); w.Code != http.StatusNotFound {
		t.Fatalf("link na lixeira: %d", w.Code)
	}
	if w = a.do(http.MethodGet, "/admin/lixeira", true, nil, false); !strings.Contains(w.Body.String(), "Produto") {
		t.Fatalf("lixeira: %d", w.Code)
	}
	a.do(http.MethodPost, "/admin/links/"+linkID+"/restore", true, nil, true)
	if w = a.do(http.MethodGet, "/"+code, false, nil, false); w.Code != http.StatusFound {
		t.Fatalf("link restaurado: %d", w.Code)
	}

	// canal na lixeira → URL do canal 404, URL curta segue valendo
	chID := regexp.MustCompile(`/admin/channels/([0-9a-f-]{36})/edit`).FindStringSubmatch(a.do(http.MethodGet, "/admin/channels", true, nil, false).Body.String())
	if chID == nil {
		t.Fatal("canal não listado")
	}
	a.do(http.MethodPost, "/admin/channels/"+chID[1]+"/delete", true, nil, true)
	if w = a.do(http.MethodGet, "/wapp/"+code, false, nil, false); w.Code != http.StatusNotFound {
		t.Fatalf("canal na lixeira: %d", w.Code)
	}
	if w = a.do(http.MethodGet, "/"+code, false, nil, false); w.Code != http.StatusFound {
		t.Fatalf("URL curta: %d", w.Code)
	}
}

func TestCrossOriginProtectionOnlyAppliesToAdminAndAPI(t *testing.T) {
	a := newApp(t)
	send := func(method, target string, hdr map[string]string, login bool) int {
		r := httptest.NewRequest(method, target, strings.NewReader("name=x&shorten_policy=shorten"))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for k, v := range hdr {
			r.Header.Set(k, v)
		}
		if login {
			r.AddCookie(a.cookie)
			r.Header.Set("X-CSRF-Token", a.csrf)
		}
		w := httptest.NewRecorder()
		a.h.ServeHTTP(w, r)
		return w.Code
	}
	// escrita no painel e na API vinda de outro site é recusada, mesmo com sessão e token
	for _, target := range []string{"/admin/marketplaces", "/admin/login", "/api/v1/marketplaces"} {
		for _, hdr := range []map[string]string{{"Sec-Fetch-Site": "cross-site"}, {"Origin": "https://evil.example"}} {
			if code := send(http.MethodPost, target, hdr, true); code != http.StatusForbidden {
				t.Errorf("POST %s com %v: %d (esperado 403)", target, hdr, code)
			}
		}
	}
	// mesma origem e clientes sem cabeçalhos de origem seguem
	if code := send(http.MethodPost, "/admin/marketplaces", map[string]string{"Sec-Fetch-Site": "same-origin"}, true); code != http.StatusSeeOther {
		t.Errorf("same-origin: %d", code)
	}
	// rotas públicas aceitam outros sites (redirecionamento, e o beacon da US3)
	if code := send(http.MethodGet, "/zzzzzzz", map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://outro.example"}, false); code != http.StatusNotFound {
		t.Errorf("GET público cross-site: %d", code)
	}
}

// TestUserStory2EndToEnd: acessos reais pelo mux completo viram eventos
// assíncronos e aparecem na API e no painel (2 cliques/1 único com o mesmo UA,
// novo único com outro UA, alcance por canal).
func TestUserStory2EndToEnd(t *testing.T) {
	a := newApp(t)
	a.do(http.MethodPost, "/admin/marketplaces", true, url.Values{"name": {"Loja"}, "shorten_policy": {"shorten"}}, true)
	w := a.do(http.MethodGet, "/admin/links/new", true, nil, false)
	mpID := regexp.MustCompile(`<option value="([0-9a-f-]{36})">Loja</option>`).FindStringSubmatch(w.Body.String())[1]
	a.do(http.MethodPost, "/admin/channels", true, url.Values{"name": {"WhatsApp"}, "segment": {"wapp"}}, true)
	w = a.do(http.MethodPost, "/admin/links", true, url.Values{"title": {"Produto"}, "destination_url": {"https://example.com/p"}, "marketplace_id": {mpID}, "active": {"true"}}, false)
	loc := w.Header().Get("Location")
	w = a.do(http.MethodGet, loc, true, nil, false)
	code := regexp.MustCompile(`https://hub\.example/([0-9a-z]{7})`).FindStringSubmatch(w.Body.String())[1]

	hit := func(path, ua string) {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Header.Set("User-Agent", ua)
		rec := httptest.NewRecorder()
		a.h.ServeHTTP(rec, r)
		if rec.Code != http.StatusFound {
			t.Fatalf("%s: %d", path, rec.Code)
		}
	}
	hit("/"+code, "Navegador A")
	hit("/"+code, "Navegador A")
	hit("/"+code, "Navegador B")
	hit("/wapp/"+code, "Navegador A")

	var raw map[string]any
	deadline := time.Now().Add(3 * time.Second)
	for {
		w = a.do(http.MethodGet, "/api/v1/stats/summary?period=7d", true, nil, false)
		if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
			t.Fatal(err)
		}
		if raw["clicks"] == float64(4) || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if raw["clicks"] != float64(4) || raw["unique"] != float64(2) || raw["active_links"] != float64(1) {
		t.Fatalf("summary = %v", raw)
	}
	w = a.do(http.MethodGet, "/api/v1/stats/channels?period=7d", true, nil, false)
	// sem canal: 3 cliques e 2 únicos de URL; wapp: 1 clique e 1 único de URL
	for _, want := range []string{`"segment":"wapp"`, `"clicks":3,"unique":2`, `"clicks":1,"unique":1`} {
		if !strings.Contains(w.Body.String(), want) {
			t.Fatalf("canais sem %s: %s", want, w.Body.String())
		}
	}
	w = a.do(http.MethodGet, "/admin", true, nil, false)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `<td class="num" data-label="Cliques">4</td>`) || !strings.Contains(w.Body.String(), "data-chart-data") {
		t.Fatalf("dashboard: %d\n%s", w.Code, w.Body.String())
	}
	// a listagem recebe os contadores (regressão: Stats precisa estar ligado ao Catalog)
	w = a.do(http.MethodGet, "/admin/links", true, nil, false)
	if !strings.Contains(w.Body.String(), `<td class="num" data-label="Cliques">4</td><td class="num hide-sm" data-label="Únicos">2</td>`) {
		t.Fatalf("listagem sem contadores:\n%s", w.Body.String())
	}
	w = a.do(http.MethodGet, loc, true, nil, false)
	if !strings.Contains(w.Body.String(), "Por canal") || !strings.Contains(w.Body.String(), "https://hub.example/wapp/"+code) {
		t.Fatalf("detalhe sem estatísticas:\n%s", w.Body.String())
	}
}

// TestUserStory3EndToEnd: o crawler recebe a prévia e não gera evento; a mesma
// URL redireciona para um navegador; o 404 e a página de privacidade trazem o aviso.
func TestUserStory3EndToEnd(t *testing.T) {
	a := newApp(t)
	a.do(http.MethodPost, "/admin/marketplaces", true, url.Values{"name": {"Loja"}, "shorten_policy": {"shorten"}}, true)
	w := a.do(http.MethodGet, "/admin/links/new", true, nil, false)
	mpID := regexp.MustCompile(`<option value="([0-9a-f-]{36})">Loja</option>`).FindStringSubmatch(w.Body.String())[1]
	w = a.do(http.MethodPost, "/admin/links", true, url.Values{"title": {"Produto"}, "destination_url": {"https://example.com/p"}, "marketplace_id": {mpID}, "active": {"true"}}, false)
	w = a.do(http.MethodGet, w.Header().Get("Location"), true, nil, false)
	code := regexp.MustCompile(`https://hub\.example/([0-9a-z]{7})`).FindStringSubmatch(w.Body.String())[1]

	get := func(path, ua string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Header.Set("User-Agent", ua)
		rec := httptest.NewRecorder()
		a.h.ServeHTTP(rec, r)
		return rec
	}
	for _, ua := range []string{"WhatsApp/2.23", "TelegramBot (like TwitterBot)", "facebookexternalhit/1.1"} {
		rec := get("/"+code, ua)
		body := rec.Body.String()
		if rec.Code != http.StatusOK || rec.Header().Get("Location") != "" || !strings.Contains(body, `og:title" content="Produto"`) || !strings.Contains(body, `href="/privacidade"`) || rec.Header().Get("Set-Cookie") != "" {
			t.Fatalf("%s: %d\n%s", ua, rec.Code, body)
		}
	}
	if rec := get("/"+code, "Mozilla/5.0 (iPhone) Safari/604.1"); rec.Code != http.StatusFound {
		t.Fatalf("navegador: %d", rec.Code)
	}
	time.Sleep(300 * time.Millisecond) // lote de 50 ms
	w = a.do(http.MethodGet, "/api/v1/stats/summary?period=7d", true, nil, false)
	if !strings.Contains(w.Body.String(), `"clicks":1,`) {
		t.Fatalf("só o acesso do navegador conta: %s", w.Body.String())
	}
	for _, path := range []string{"/zzzzzzz", "/privacidade"} {
		rec := get(path, "Mozilla/5.0")
		if !strings.Contains(rec.Body.String(), `id="privacy-notice"`) || rec.Header().Get("Set-Cookie") != "" {
			t.Fatalf("%s sem aviso de privacidade ou com cookie", path)
		}
	}
}

// /admin/ui (guia de componentes) só existe em APP_ENV=development.
func TestComponentGuideOnlyInDevelopment(t *testing.T) {
	for env, want := range map[string]int{"test": http.StatusNotFound, "production": http.StatusNotFound, "development": http.StatusOK} {
		a := newAppWithEnv(t, env)
		if w := a.do(http.MethodGet, "/admin/ui", true, nil, false); w.Code != want {
			t.Errorf("APP_ENV=%s: /admin/ui = %d, esperado %d", env, w.Code, want)
		}
		if w := a.do(http.MethodGet, "/admin/ui", false, nil, false); w.Code != http.StatusSeeOther {
			t.Errorf("APP_ENV=%s: sem sessão = %d, esperado redirecionamento ao login", env, w.Code)
		}
	}
}
