package redirect_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/domain"
	"github.com/hublinks/hublinks/internal/redirect"
	"github.com/hublinks/hublinks/internal/service"
	"github.com/hublinks/hublinks/internal/store"
	"github.com/hublinks/hublinks/internal/store/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

type env struct {
	svc  service.Catalog
	h    *redirect.Handler
	org  uuid.UUID
	pool *pgxpool.Pool
}

// newEnv liga o handler público ao catálogo real (PostgreSQL) e ao mesmo cache
// que o serviço invalida a cada escrita, como no servidor.
func newEnv(t *testing.T) env {
	t.Helper()
	pool := testdb.New(t)
	org := uuid.New()
	if _, err := pool.Exec(context.Background(), "INSERT INTO organizations(id,name,slug,created_at) VALUES($1,$2,$3,$4)", org, "Org", org.String(), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	cache := redirect.NewCache()
	st := store.NewCatalog(pool)
	return env{
		svc:  service.Catalog{Store: st, Cache: cache, BaseURL: "https://hub.example"},
		h:    &redirect.Handler{Catalog: st, Cache: cache},
		org:  org,
		pool: pool,
	}
}

func (e env) get(path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

func (e env) link(t *testing.T, policy domain.Policy, dest string) (mpID, linkID uuid.UUID, code string) {
	t.Helper()
	ctx := context.Background()
	mp, err := e.svc.CreateMarketplace(ctx, e.org, "Loja", policy)
	if err != nil {
		t.Fatal(err)
	}
	v, err := e.svc.CreateLink(ctx, domain.AffiliateLink{OrgID: e.org, MarketplaceID: mp.ID, Title: "Produto", DestinationURL: dest, Active: true})
	if err != nil {
		t.Fatal(err)
	}
	return mp.ID, v.ID, v.Code
}

func wantFound(t *testing.T, w *httptest.ResponseRecorder, location string) {
	t.Helper()
	if w.Code != http.StatusFound || w.Header().Get("Location") != location {
		t.Fatalf("esperado 302 para %q, veio %d (Location=%q)", location, w.Code, w.Header().Get("Location"))
	}
	if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Set-Cookie") != "" {
		t.Fatalf("cabeçalhos do redirecionamento incorretos: %#v", w.Header())
	}
}

func wantNotFound(t *testing.T, w *httptest.ResponseRecorder, what string) {
	t.Helper()
	if w.Code != http.StatusNotFound {
		t.Fatalf("%s: esperado 404, veio %d", what, w.Code)
	}
	if w.Header().Get("Set-Cookie") != "" || w.Header().Get("Location") != "" {
		t.Fatalf("%s: o 404 não pode ter Set-Cookie nem Location: %#v", what, w.Header())
	}
}

func TestRedirectLiveCatalogResolvesCodeAndChannel(t *testing.T) {
	e := newEnv(t)
	_, _, code := e.link(t, domain.PolicyShorten, "https://example.com/destino")
	if _, err := e.svc.CreateChannel(context.Background(), e.org, "WhatsApp", "wapp"); err != nil {
		t.Fatal(err)
	}
	wantFound(t, e.get("/"+code), "https://example.com/destino")
	wantFound(t, e.get("/wapp/"+code), "https://example.com/destino")
}

func TestRedirectDirectPolicyLinkAlsoRedirects302(t *testing.T) {
	e := newEnv(t)
	_, _, code := e.link(t, domain.PolicyDirect, "https://example.com/original")
	wantFound(t, e.get("/"+code), "https://example.com/original")
}

func TestRedirectNotFoundCases(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	mpID, linkID, code := e.link(t, domain.PolicyShorten, "https://example.com/a")
	ch, err := e.svc.CreateChannel(ctx, e.org, "WhatsApp", "wapp")
	if err != nil {
		t.Fatal(err)
	}

	for _, p := range []string{"/abc123", "/abc12345", "/ABC1234", "/abc-123", "/abc%20123", "/wapp/abc12"} {
		wantNotFound(t, e.get(p), "formato inválido "+p)
	}
	wantNotFound(t, e.get("/zzzzzzz"), "código inexistente")
	wantNotFound(t, e.get("/naoexiste/"+code), "segmento inexistente")

	// inativo
	off := false
	if err := e.svc.UpdateLink(ctx, e.org, linkID, nil, nil, nil, nil, nil, &off); err != nil {
		t.Fatal(err)
	}
	wantNotFound(t, e.get("/"+code), "link inativo")
	on := true
	if err := e.svc.UpdateLink(ctx, e.org, linkID, nil, nil, nil, nil, nil, &on); err != nil {
		t.Fatal(err)
	}
	wantFound(t, e.get("/"+code), "https://example.com/a")

	// segmento na lixeira
	if err := e.svc.DeleteChannel(ctx, e.org, ch.ID); err != nil {
		t.Fatal(err)
	}
	wantNotFound(t, e.get("/wapp/"+code), "segmento na lixeira")
	wantFound(t, e.get("/"+code), "https://example.com/a")

	// link na lixeira
	if err := e.svc.DeleteLink(ctx, e.org, linkID); err != nil {
		t.Fatal(err)
	}
	wantNotFound(t, e.get("/"+code), "link na lixeira")

	// restaurado volta a redirecionar
	if err := e.svc.RestoreLink(ctx, e.org, linkID); err != nil {
		t.Fatal(err)
	}
	wantFound(t, e.get("/"+code), "https://example.com/a")

	// excluído definitivamente (purged_at)
	if err := e.svc.DeleteLink(ctx, e.org, linkID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.pool.Exec(ctx, "UPDATE affiliate_links SET purged_at=now() WHERE id=$1", linkID); err != nil {
		t.Fatal(err)
	}
	e.h.Cache.InvalidateAll()
	wantNotFound(t, e.get("/"+code), "link excluído definitivamente")
	_ = mpID
}

func TestRedirectEditingDestinationInvalidatesCache(t *testing.T) {
	e := newEnv(t)
	_, linkID, code := e.link(t, domain.PolicyShorten, "https://example.com/antigo")
	wantFound(t, e.get("/"+code), "https://example.com/antigo") // aquece o cache

	novo := "https://example.com/novo"
	if err := e.svc.UpdateLink(context.Background(), e.org, linkID, nil, &novo, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	wantFound(t, e.get("/"+code), novo)
}

func TestRedirectPolicyChangeOnMarketplaceInvalidatesCache(t *testing.T) {
	e := newEnv(t)
	mpID, _, code := e.link(t, domain.PolicyShorten, "https://example.com/a")
	wantFound(t, e.get("/"+code), "https://example.com/a")
	direct := domain.PolicyDirect
	if err := e.svc.UpdateMarketplace(context.Background(), e.org, mpID, nil, &direct); err != nil {
		t.Fatal(err)
	}
	// continua redirecionando 302 após virar direct
	wantFound(t, e.get("/"+code), "https://example.com/a")
}
