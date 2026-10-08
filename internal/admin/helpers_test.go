package admin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/admin"
	"github.com/hublinks/hublinks/internal/auth"
	"github.com/hublinks/hublinks/internal/service"
	"github.com/hublinks/hublinks/internal/store"
	"github.com/hublinks/hublinks/internal/store/testdb"
)

type env struct {
	c       admin.Catalog
	session auth.Session
}

func newEnv(t *testing.T) env {
	t.Helper()
	p := testdb.New(t)
	org := uuid.New()
	if _, err := p.Exec(context.Background(), "INSERT INTO organizations(id,name,slug,created_at) VALUES($1,$2,$3,$4)", org, "Org", org.String(), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	svc := service.Catalog{Store: store.NewCatalog(p), BaseURL: "https://hub.example"}
	return env{c: admin.Catalog{Service: svc, TrashRetentionDays: 30}, session: auth.Session{OrgID: org, CSRF: "csrf-token"}}
}

// req descreve uma requisição ao painel.
type req struct {
	method, target string
	form           url.Values
	id             string
	hx             bool   // cabeçalho HX-Request
	hxTarget       string // HX-Target
	currentURL     string // HX-Current-URL
}

func (e env) do(t *testing.T, h http.HandlerFunc, q req) *httptest.ResponseRecorder {
	t.Helper()
	method := q.method
	if method == "" {
		method = http.MethodGet
	}
	r := httptest.NewRequest(method, q.target, strings.NewReader(q.form.Encode()))
	if q.form != nil {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if q.id != "" {
		r.SetPathValue("id", q.id)
	}
	if q.hx {
		r.Header.Set("HX-Request", "true")
	}
	if q.hxTarget != "" {
		r.Header.Set("HX-Target", q.hxTarget)
	}
	if q.currentURL != "" {
		r.Header.Set("HX-Current-URL", q.currentURL)
	}
	w := httptest.NewRecorder()
	h(w, auth.WithSession(r, e.session))
	return w
}

func mustContain(t *testing.T, w *httptest.ResponseRecorder, parts ...string) {
	t.Helper()
	body := w.Body.String()
	for _, p := range parts {
		if !strings.Contains(body, p) {
			t.Fatalf("resposta não contém %q (status %d):\n%s", p, w.Code, body)
		}
	}
}

func mustNotContain(t *testing.T, w *httptest.ResponseRecorder, parts ...string) {
	t.Helper()
	body := w.Body.String()
	for _, p := range parts {
		if strings.Contains(body, p) {
			t.Fatalf("resposta não deveria conter %q:\n%s", p, body)
		}
	}
}

func form(kv ...string) url.Values {
	v := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		v.Set(kv[i], kv[i+1])
	}
	return v
}

func trashOpts() store.ListOptions { return store.ListOptions{Trash: true} }
