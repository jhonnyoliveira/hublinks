package admin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/hublinks/hublinks/internal/admin"
	"github.com/hublinks/hublinks/internal/auth"
	"github.com/hublinks/hublinks/internal/store"
	"github.com/hublinks/hublinks/internal/store/testdb"
)

const adminPassword = "senha-do-admin-1234"

func newLogin(t *testing.T, maxFailures int) (admin.Login, auth.Manager) {
	t.Helper()
	p := testdb.New(t)
	if err := store.Bootstrap(context.Background(), p, "Org", "Admin@Example.com", adminPassword); err != nil {
		t.Fatal(err)
	}
	m := auth.Manager{Pool: p}
	return admin.Login{Pool: p, Sessions: m, Limiter: auth.NewLoginLimiter("pepper-de-teste-com-32-caracteres!", maxFailures, time.Minute, time.Minute)}, m
}

func postLogin(l admin.Login, email, password, next string) *httptest.ResponseRecorder {
	form := url.Values{"email": {email}, "password": {password}, "next": {next}}
	r := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.RemoteAddr = "203.0.113.7:4444"
	w := httptest.NewRecorder()
	l.ServeHTTP(w, r)
	return w
}

func TestLoginSuccessCreatesSessionWithSecureCookie(t *testing.T) {
	l, m := newLogin(t, 5)
	w := postLogin(l, "  ADMIN@example.com ", adminPassword, "")
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/admin" {
		t.Fatalf("%d %q", w.Code, w.Header().Get("Location"))
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies: %v", cookies)
	}
	c := cookies[0]
	if c.Name != "hl_session" || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.Expires.IsZero() {
		t.Fatalf("atributos do cookie: %+v", c)
	}
	if s, err := m.Lookup(context.Background(), c.Value); err != nil || s.CSRF == "" {
		t.Fatalf("a sessão do cookie deve existir: %v", err)
	}
}

func TestLoginFailureIsGenericAndCostsTheSameForUnknownEmail(t *testing.T) {
	l, _ := newLogin(t, 50)
	timed := func(email, pass string) (string, time.Duration) {
		start := time.Now()
		w := postLogin(l, email, pass, "")
		if w.Code != http.StatusOK || len(w.Result().Cookies()) != 0 {
			t.Fatalf("falha não pode criar sessão: %d %v", w.Code, w.Result().Cookies())
		}
		return w.Body.String(), time.Since(start)
	}
	wrongPass, dWrong := timed("admin@example.com", "senha-errada-123456")
	unknown, dUnknown := timed("naoexiste@example.com", "senha-errada-123456")
	for _, body := range []string{wrongPass, unknown} {
		if !strings.Contains(body, "E-mail ou senha inválidos") {
			t.Fatalf("mensagem genérica ausente:\n%s", body)
		}
	}
	if strings.Contains(unknown, "naoexiste") || strings.Contains(wrongPass, "não encontrado") {
		t.Fatal("a mensagem não pode revelar se o e-mail existe")
	}
	// e-mail inexistente também executa o argon2: o tempo não pode ser ordens de grandeza menor
	if dUnknown < dWrong/4 {
		t.Fatalf("e-mail inexistente respondeu rápido demais (%s vs %s): vaza quais e-mails existem", dUnknown, dWrong)
	}
}

func TestLoginNextOnlyAllowsAdminPaths(t *testing.T) {
	l, _ := newLogin(t, 50)
	cases := map[string]string{
		"/admin/links?page=2":     "/admin/links?page=2",
		"/admin":                  "/admin",
		"https://evil.example":    "/admin",
		"//evil.example":          "/admin",
		"/adminx":                 "/admin",
		"/admin\\evil.example":    "/admin",
		"/admin/\r\nSet-Cookie:x": "/admin",
		"javascript:alert(1)":     "/admin",
		"":                        "/admin",
		"/outra-area":             "/admin",
	}
	for next, want := range cases {
		w := postLogin(l, "admin@example.com", adminPassword, next)
		if got := w.Header().Get("Location"); w.Code != http.StatusSeeOther || got != want {
			t.Errorf("next=%q: Location=%q (status %d), esperado %q", next, got, w.Code, want)
		}
	}
}

func TestLoginLocksAfterTooManyFailures(t *testing.T) {
	l, _ := newLogin(t, 3)
	for i := 0; i < 3; i++ {
		postLogin(l, "admin@example.com", "errada-errada-123", "")
	}
	w := postLogin(l, "admin@example.com", adminPassword, "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Muitas tentativas") || len(w.Result().Cookies()) != 0 {
		t.Fatalf("bloqueado deveria recusar até a senha correta: %d\n%s", w.Code, w.Body.String())
	}
}

func TestLoginFormKeepsAndEscapesNext(t *testing.T) {
	l, _ := newLogin(t, 5)
	r := httptest.NewRequest(http.MethodGet, "/admin/login?next="+url.QueryEscape(`/admin/x"><script>alert(1)</script>`), nil)
	w := httptest.NewRecorder()
	l.ServeHTTP(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `name="next"`) || strings.Contains(w.Body.String(), "<script>alert(1)") {
		t.Fatalf("%d\n%s", w.Code, w.Body.String())
	}
	if r := httptest.NewRequest(http.MethodPut, "/admin/login", nil); true {
		w = httptest.NewRecorder()
		l.ServeHTTP(w, r)
		if w.Code != http.StatusNotFound {
			t.Fatalf("método não suportado: %d", w.Code)
		}
	}
}

func TestLogoutInvalidatesSessionAndClearsCookie(t *testing.T) {
	l, m := newLogin(t, 5)
	login := postLogin(l, "admin@example.com", adminPassword, "")
	raw := login.Result().Cookies()[0].Value
	r := httptest.NewRequest(http.MethodPost, "/admin/logout", nil)
	r.AddCookie(&http.Cookie{Name: auth.CookieName, Value: raw})
	w := httptest.NewRecorder()
	admin.Logout(m).ServeHTTP(w, r)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/admin/login" {
		t.Fatalf("%d %q", w.Code, w.Header().Get("Location"))
	}
	if cs := w.Result().Cookies(); len(cs) != 1 || cs[0].MaxAge >= 0 {
		t.Fatalf("o cookie deve ser apagado: %v", cs)
	}
	if _, err := m.Lookup(context.Background(), raw); err == nil {
		t.Fatal("a sessão deve ter sido invalidada no servidor")
	}
}
