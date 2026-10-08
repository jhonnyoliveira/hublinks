package auth_test

import (
	"context"
	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/auth"
	"github.com/hublinks/hublinks/internal/store/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionLifecycle(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	user, _ := uuid.NewV7()
	org, _ := uuid.NewV7()
	now := time.Now().UTC()
	_, err := p.Exec(ctx, "INSERT INTO organizations(id,name,slug,created_at) VALUES($1,'Org','org',$2)", org, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Exec(ctx, "INSERT INTO users(id,email,password_hash,created_at) VALUES($1,'admin@example.test','hash',$2)", user, now)
	if err != nil {
		t.Fatal(err)
	}
	m := auth.Manager{Pool: p}
	raw, s, err := m.Create(ctx, user, org)
	if err != nil {
		t.Fatal(err)
	}
	if raw == "" || s.CSRF == "" {
		t.Fatal("sessão sem tokens")
	}
	got, err := m.Lookup(ctx, raw)
	if err != nil || got.UserID != user || got.OrgID != org {
		t.Fatalf("lookup: %#v, %v", got, err)
	}
	if err := m.Destroy(ctx, raw); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Lookup(ctx, raw); err == nil {
		t.Fatal("sessão removida ainda encontrada")
	}
}

func seedSession(t *testing.T, p *pgxpool.Pool) (auth.Manager, uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	user, _ := uuid.NewV7()
	org, _ := uuid.NewV7()
	now := time.Now().UTC()
	if _, err := p.Exec(ctx, "INSERT INTO organizations(id,name,slug,created_at) VALUES($1,'Org','org',$2)", org, now); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, "INSERT INTO users(id,email,password_hash,created_at) VALUES($1,'a@example.test','hash',$2)", user, now); err != nil {
		t.Fatal(err)
	}
	return auth.Manager{Pool: p}, user, org
}

func TestEachLoginGetsANewTokenAndOnlyTheHashIsStored(t *testing.T) {
	p := testdb.New(t)
	m, user, org := seedSession(t, p)
	ctx := context.Background()
	a, sa, _ := m.Create(ctx, user, org)
	b, sb, _ := m.Create(ctx, user, org)
	if a == b || sa.CSRF == sb.CSRF {
		t.Fatal("cada login deve gerar token e CSRF novos")
	}
	if len(a) < 43 { // 32 bytes em base64 sem padding
		t.Fatalf("token curto: %d", len(a))
	}
	var plain int
	if err := p.QueryRow(ctx, "SELECT count(*) FROM sessions WHERE token_hash = convert_to($1,'UTF8') OR token_hash = convert_to($2,'UTF8')", a, b).Scan(&plain); err != nil || plain != 0 {
		t.Fatalf("o token em claro não pode ser gravado: %d %v", plain, err)
	}
	var hashLen int
	_ = p.QueryRow(ctx, "SELECT length(token_hash) FROM sessions LIMIT 1").Scan(&hashLen)
	if hashLen != 32 {
		t.Fatalf("esperado SHA-256 (32 bytes): %d", hashLen)
	}
}

func TestSessionCookieAttributes(t *testing.T) {
	w := httptest.NewRecorder()
	auth.SetCookie(w, "tok", time.Now().Add(time.Hour))
	c := w.Result().Cookies()[0]
	if c.Name != "hl_session" || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
		t.Fatalf("cookie: %+v", c)
	}
	w = httptest.NewRecorder()
	auth.ClearCookie(w)
	if c = w.Result().Cookies()[0]; c.MaxAge >= 0 || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie de logout: %+v", c)
	}
}

func TestSessionExpiresAfterSevenDaysWithoutUse(t *testing.T) {
	p := testdb.New(t)
	m, user, org := seedSession(t, p)
	ctx := context.Background()

	// sem uso há 8 dias: expirou
	old, _, _ := m.Create(ctx, user, org)
	if _, err := p.Exec(ctx, "UPDATE sessions SET last_seen_at=now()-interval '8 days', expires_at=now()-interval '1 day'"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Lookup(ctx, old); err == nil {
		t.Fatal("sessão sem uso há 8 dias deveria ter expirado")
	}

	// criada há 6 dias e usada agora: a validade desliza para 7 dias a partir do uso
	active, _, _ := m.Create(ctx, user, org)
	if _, err := p.Exec(ctx, "UPDATE sessions SET created_at=now()-interval '6 days', last_seen_at=now()-interval '1 hour', expires_at=now()+interval '1 day' WHERE token_hash=sha256(convert_to($1,'UTF8'))", active); err != nil {
		t.Fatal(err)
	}
	got, err := m.Lookup(ctx, active)
	if err != nil || !got.Renewed {
		t.Fatalf("sessão em uso deve renovar: %+v %v", got, err)
	}
	if left := time.Until(got.ExpiresAt); left < 6*24*time.Hour+23*time.Hour {
		t.Fatalf("a validade deveria voltar a ~7 dias, restam %s", left)
	}
	var stored time.Time
	_ = p.QueryRow(ctx, "SELECT expires_at FROM sessions WHERE token_hash=sha256(convert_to($1,'UTF8'))", active).Scan(&stored)
	if time.Until(stored) < 6*24*time.Hour+23*time.Hour {
		t.Fatalf("a renovação deve ser gravada: %s", stored)
	}

	// uso muito recente: não grava de novo
	again, err := m.Lookup(ctx, active)
	if err != nil || again.Renewed {
		t.Fatalf("não deve renovar a cada requisição: %+v %v", again, err)
	}
}

func TestRequireSessionInjectsSessionAndRenewsCookie(t *testing.T) {
	p := testdb.New(t)
	m, user, org := seedSession(t, p)
	ctx := context.Background()
	raw, _, _ := m.Create(ctx, user, org)
	if _, err := p.Exec(ctx, "UPDATE sessions SET last_seen_at=now()-interval '1 hour'"); err != nil {
		t.Fatal(err)
	}
	var seen auth.Session
	h := m.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, _ = auth.FromContext(r.Context())
	}), false)
	r := httptest.NewRequest(http.MethodGet, "/admin", nil)
	r.AddCookie(&http.Cookie{Name: auth.CookieName, Value: raw})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if seen.UserID != user || seen.OrgID != org || seen.CSRF == "" {
		t.Fatalf("contexto: %+v", seen)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Value != raw || !cookies[0].HttpOnly || !cookies[0].Secure {
		t.Fatalf("o cookie deveria ser renovado junto: %+v", cookies)
	}

	for name, cookie := range map[string]*http.Cookie{"cookie inválido": {Name: auth.CookieName, Value: "nao-existe"}, "sem cookie": nil} {
		r := httptest.NewRequest(http.MethodGet, "/admin/links", nil)
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/admin/login?next=/admin/links" {
			t.Fatalf("%s: %d %q", name, w.Code, w.Header().Get("Location"))
		}
	}
}
