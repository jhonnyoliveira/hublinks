package api_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/api"
	"github.com/hublinks/hublinks/internal/auth"
	"github.com/hublinks/hublinks/internal/service"
	"github.com/hublinks/hublinks/internal/store"
	"github.com/hublinks/hublinks/internal/store/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

// twoOrgs devolve o catálogo da API com sessões de duas organizações distintas.
func twoOrgs(t *testing.T) (api.Catalog, auth.Session, auth.Session, *pgxpool.Pool) {
	t.Helper()
	p := testdb.New(t)
	var sessions [2]auth.Session
	for i := range sessions {
		org := uuid.New()
		if _, err := p.Exec(context.Background(), "INSERT INTO organizations(id,name,slug,created_at) VALUES($1,$2,$3,$4)", org, "Org", org.String(), time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		sessions[i] = auth.Session{OrgID: org}
	}
	c := api.Catalog{Service: service.Catalog{Store: store.NewCatalog(p), BaseURL: "https://hub.example"}}
	return c, sessions[0], sessions[1], p
}

func item(t *testing.T, h http.HandlerFunc, s auth.Session, method, id string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var data []byte
	if body != nil {
		data = mustJSON(t, body)
	}
	r := httptest.NewRequest(method, "/", bytes.NewReader(data))
	r.SetPathValue("id", id)
	w := httptest.NewRecorder()
	h(w, auth.WithSession(r, s))
	return w
}

func errorEnvelope(t *testing.T, w *httptest.ResponseRecorder, status int, code string) map[string]any {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status %d, esperado %d: %s", w.Code, status, w.Body.String())
	}
	env, ok := decoded(t, w)["error"].(map[string]any)
	if !ok || env["code"] != code || env["message"] == "" {
		t.Fatalf("envelope de erro inválido (esperado %q): %s", code, w.Body.String())
	}
	return env
}

func createMarketplace(t *testing.T, c api.Catalog, s auth.Session, name string) string {
	t.Helper()
	w := call(t, c.Marketplace, s, http.MethodPost, "/", map[string]any{"name": name, "shorten_policy": "shorten"})
	if w.Code != http.StatusCreated {
		t.Fatalf("marketplace: %d %s", w.Code, w.Body.String())
	}
	return decoded(t, w)["id"].(string)
}

func createLink(t *testing.T, c api.Catalog, s auth.Session, title, marketplaceID string) string {
	t.Helper()
	w := call(t, c.Link, s, http.MethodPost, "/", map[string]any{"title": title, "destination_url": "https://example.com/" + title, "marketplace_id": marketplaceID})
	if w.Code != http.StatusCreated {
		t.Fatalf("link: %d %s", w.Code, w.Body.String())
	}
	return decoded(t, w)["id"].(string)
}

func TestCatalogAPIValidationErrorsUseEnvelopeWithFields(t *testing.T) {
	c, s, _, _ := twoOrgs(t)
	mp := createMarketplace(t, c, s, "Loja")
	cases := []struct {
		name  string
		h     http.HandlerFunc
		body  map[string]any
		field string
	}{
		{"marketplace sem nome", c.Marketplace, map[string]any{"name": "", "shorten_policy": "shorten"}, "name"},
		{"marketplace com política inválida", c.Marketplace, map[string]any{"name": "X", "shorten_policy": "outra"}, "shorten_policy"},
		{"canal com segmento reservado", c.Channel, map[string]any{"name": "API", "segment": "api"}, "segment"},
		{"canal com segmento em maiúsculas", c.Channel, map[string]any{"name": "WA", "segment": "WApp"}, "segment"},
		{"canal sem nome", c.Channel, map[string]any{"name": "", "segment": "ok"}, "name"},
		{"link com esquema ftp", c.Link, map[string]any{"title": "T", "destination_url": "ftp://example.com", "marketplace_id": mp}, "destination_url"},
		{"link sem título", c.Link, map[string]any{"title": "", "destination_url": "https://example.com", "marketplace_id": mp}, "title"},
		{"link com imagem inválida", c.Link, map[string]any{"title": "T", "destination_url": "https://example.com", "marketplace_id": mp, "image_url": "javascript:alert(1)"}, "image_url"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := call(t, tc.h, s, http.MethodPost, "/", tc.body)
			env := errorEnvelope(t, w, http.StatusUnprocessableEntity, "validation_failed")
			fields, _ := env["fields"].(map[string]any)
			if fields[tc.field] == nil {
				t.Fatalf("campo %q ausente em fields: %s", tc.field, w.Body.String())
			}
		})
	}
}

func TestCatalogAPIInvalidJSONAndUnknownIDs(t *testing.T) {
	c, s, _, _ := twoOrgs(t)
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("{")))
	w := httptest.NewRecorder()
	c.Marketplace(w, auth.WithSession(r, s))
	errorEnvelope(t, w, http.StatusBadRequest, "invalid_json")

	for name, h := range map[string]http.HandlerFunc{"marketplace": c.MarketplaceItem, "canal": c.ChannelItem, "link": c.LinkItem} {
		for _, id := range []string{"nao-e-uuid", uuid.NewString()} {
			for _, m := range []string{http.MethodGet, http.MethodPatch, http.MethodDelete} {
				errorEnvelope(t, item(t, h, s, m, id, map[string]any{}), http.StatusNotFound, "not_found")
			}
		}
		_ = name
	}
	for _, h := range []http.HandlerFunc{c.MarketplaceRestore, c.ChannelRestore, c.LinkRestore} {
		errorEnvelope(t, item(t, h, s, http.MethodPost, uuid.NewString(), nil), http.StatusNotFound, "not_found")
	}
}

func TestCatalogAPIOtherOrganizationGets404(t *testing.T) {
	c, a, b, _ := twoOrgs(t)
	mp := createMarketplace(t, c, a, "Loja")
	link := createLink(t, c, a, "p", mp)
	w := call(t, c.Channel, a, http.MethodPost, "/", map[string]any{"name": "WhatsApp", "segment": "wapp"})
	ch := decoded(t, w)["id"].(string)

	for _, tc := range []struct {
		h  http.HandlerFunc
		id string
	}{{c.MarketplaceItem, mp}, {c.ChannelItem, ch}, {c.LinkItem, link}} {
		for _, m := range []string{http.MethodGet, http.MethodPatch, http.MethodDelete} {
			errorEnvelope(t, item(t, tc.h, b, m, tc.id, map[string]any{"name": "x"}), http.StatusNotFound, "not_found")
		}
	}
	for _, m := range []string{"Marketplace", "Channel", "Link"} {
		h := map[string]http.HandlerFunc{"Marketplace": c.Marketplace, "Channel": c.Channel, "Link": c.Link}[m]
		w := call(t, h, b, http.MethodGet, "/", nil)
		if w.Code != http.StatusOK || decoded(t, w)["total"].(float64) != 0 {
			t.Fatalf("%s: a org B não deve listar itens da org A: %s", m, w.Body.String())
		}
	}
	// o item da org A permanece intacto
	if w := item(t, c.LinkItem, a, http.MethodGet, link, nil); w.Code != http.StatusOK {
		t.Fatalf("link da org A: %d", w.Code)
	}
	// um link de B não aceita marketplace de A
	w = call(t, c.Link, b, http.MethodPost, "/", map[string]any{"title": "t", "destination_url": "https://example.com", "marketplace_id": mp})
	if w.Code != http.StatusNotFound && w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("marketplace de outra org aceito: %d %s", w.Code, w.Body.String())
	}
}

func TestCatalogAPIDuplicateSegmentConflictsEvenInTrash(t *testing.T) {
	c, s, _, _ := twoOrgs(t)
	w := call(t, c.Channel, s, http.MethodPost, "/", map[string]any{"name": "WhatsApp", "segment": "wapp"})
	id := decoded(t, w)["id"].(string)
	errorEnvelope(t, call(t, c.Channel, s, http.MethodPost, "/", map[string]any{"name": "Outro", "segment": "wapp"}), http.StatusConflict, "conflict")

	if w = item(t, c.ChannelItem, s, http.MethodDelete, id, nil); w.Code != http.StatusNoContent {
		t.Fatal(w.Code)
	}
	errorEnvelope(t, call(t, c.Channel, s, http.MethodPost, "/", map[string]any{"name": "Outro", "segment": "wapp"}), http.StatusConflict, "conflict")

	// PATCH para segmento ocupado também conflita
	other := decoded(t, call(t, c.Channel, s, http.MethodPost, "/", map[string]any{"name": "Telegram", "segment": "tg"}))["id"].(string)
	errorEnvelope(t, item(t, c.ChannelItem, s, http.MethodPatch, other, map[string]any{"segment": "wapp"}), http.StatusConflict, "conflict")
}

func TestCatalogAPIMarketplaceDeleteConflictCarriesLinks(t *testing.T) {
	c, s, _, _ := twoOrgs(t)
	mp := createMarketplace(t, c, s, "Loja")
	link := createLink(t, c, s, "p", mp)
	w := item(t, c.MarketplaceItem, s, http.MethodDelete, mp, nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	body := decoded(t, w)
	if body["error"].(map[string]any)["code"] != "conflict" {
		t.Fatalf("envelope: %#v", body)
	}
	links := body["links"].([]any)
	first := links[0].(map[string]any)
	if len(links) != 1 || first["id"] != link || first["title"] != "p" {
		t.Fatalf("links do conflito: %#v", links)
	}
}

func TestCatalogAPIRestoreRules(t *testing.T) {
	c, s, _, p := twoOrgs(t)
	mp := createMarketplace(t, c, s, "Loja")
	link := createLink(t, c, s, "p", mp)

	// restaurar item que não está na lixeira → 404
	errorEnvelope(t, item(t, c.LinkRestore, s, http.MethodPost, link, nil), http.StatusNotFound, "not_found")

	// link e marketplace na lixeira: restaurar o link exige o marketplace ativo → 409
	item(t, c.LinkItem, s, http.MethodDelete, link, nil)
	if w := item(t, c.MarketplaceItem, s, http.MethodDelete, mp, nil); w.Code != http.StatusNoContent {
		t.Fatalf("marketplace sem links ativos deve ir à lixeira: %d %s", w.Code, w.Body.String())
	}
	errorEnvelope(t, item(t, c.LinkRestore, s, http.MethodPost, link, nil), http.StatusConflict, "conflict")

	// restaurar o marketplace devolve o recurso e libera o link
	w := item(t, c.MarketplaceRestore, s, http.MethodPost, mp, nil)
	if got := decoded(t, w); w.Code != http.StatusOK || got["id"] != mp || got["name"] != "Loja" || got["deleted_at"] != nil {
		t.Fatalf("restore de marketplace deve devolver o recurso: %d %s", w.Code, w.Body.String())
	}
	w = item(t, c.LinkRestore, s, http.MethodPost, link, nil)
	if got := decoded(t, w); w.Code != http.StatusOK || got["id"] != link || got["code"] == nil {
		t.Fatalf("restore de link deve devolver o recurso: %d %s", w.Code, w.Body.String())
	}

	// item definitivamente excluído (purged_at) não é restaurável
	item(t, c.LinkItem, s, http.MethodDelete, link, nil)
	if _, err := p.Exec(context.Background(), "UPDATE affiliate_links SET purged_at = now() WHERE id = $1", link); err != nil {
		t.Fatal(err)
	}
	errorEnvelope(t, item(t, c.LinkRestore, s, http.MethodPost, link, nil), http.StatusNotFound, "not_found")
}

func TestCatalogAPIPatchAndRestoreReturnResources(t *testing.T) {
	c, s, _, _ := twoOrgs(t)
	mp := createMarketplace(t, c, s, "Loja")
	createLink(t, c, s, "p", mp)
	w := call(t, c.Channel, s, http.MethodPost, "/", map[string]any{"name": "WhatsApp", "segment": "wapp"})
	created := decoded(t, w)
	if w.Code != http.StatusCreated || created["segment"] != "wapp" || created["active_links_count"] != float64(1) {
		t.Fatalf("POST de canal deve devolver o recurso com active_links_count: %s", w.Body.String())
	}
	id := created["id"].(string)

	w = item(t, c.ChannelItem, s, http.MethodPatch, id, map[string]any{"name": "Zap"})
	if got := decoded(t, w); w.Code != http.StatusOK || got["name"] != "Zap" || got["segment"] != "wapp" {
		t.Fatalf("PATCH de canal: %s", w.Body.String())
	}
	w = item(t, c.MarketplaceItem, s, http.MethodPatch, mp, map[string]any{"shorten_policy": "direct"})
	if got := decoded(t, w); w.Code != http.StatusOK || got["shorten_policy"] != "direct" || got["name"] != "Loja" {
		t.Fatalf("PATCH de marketplace: %s", w.Body.String())
	}

	item(t, c.ChannelItem, s, http.MethodDelete, id, nil)
	w = item(t, c.ChannelRestore, s, http.MethodPost, id, nil)
	if got := decoded(t, w); w.Code != http.StatusOK || got["id"] != id || got["deleted_at"] != nil {
		t.Fatalf("restore de canal: %s", w.Body.String())
	}
}

func TestCatalogAPIListsHonourTrashFiltersAndPaging(t *testing.T) {
	c, s, _, _ := twoOrgs(t)
	mpA := createMarketplace(t, c, s, "Alfa")
	mpB := createMarketplace(t, c, s, "Beta")
	l1 := createLink(t, c, s, "camisa", mpA)
	createLink(t, c, s, "calca", mpB)
	chID := decoded(t, call(t, c.Channel, s, http.MethodPost, "/", map[string]any{"name": "WhatsApp", "segment": "wapp"}))["id"].(string)

	total := func(h http.HandlerFunc, target string) (float64, map[string]any) {
		w := call(t, h, s, http.MethodGet, target, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", target, w.Code, w.Body.String())
		}
		body := decoded(t, w)
		return body["total"].(float64), body
	}
	if n, body := total(c.Link, "/?per_page=500&page=2"); n != 2 || body["per_page"] != float64(100) || body["page"] != float64(2) {
		t.Fatalf("paginação: %#v", body)
	}
	if n, _ := total(c.Link, "/?q=camisa"); n != 1 {
		t.Fatalf("filtro q: %v", n)
	}
	if n, _ := total(c.Link, "/?marketplace_id="+mpB); n != 1 {
		t.Fatalf("filtro marketplace_id: %v", n)
	}
	item(t, c.LinkItem, s, http.MethodPatch, l1, map[string]any{"active": false})
	if n, _ := total(c.Link, "/?active=false"); n != 1 {
		t.Fatalf("filtro active=false: %v", n)
	}

	item(t, c.LinkItem, s, http.MethodDelete, l1, nil)
	item(t, c.ChannelItem, s, http.MethodDelete, chID, nil)
	if n, _ := total(c.Link, "/"); n != 1 {
		t.Fatalf("o padrão exclui a lixeira: %v", n)
	}
	if n, _ := total(c.Link, "/?trash=true"); n != 1 {
		t.Fatalf("lixeira de links: %v", n)
	}
	if n, _ := total(c.Channel, "/"); n != 0 {
		t.Fatalf("canais fora da lixeira: %v", n)
	}
	if n, _ := total(c.Channel, "/?trash=true"); n != 1 {
		t.Fatalf("lixeira de canais: %v", n)
	}
	item(t, c.MarketplaceItem, s, http.MethodDelete, mpA, nil)
	if n, _ := total(c.Marketplace, "/?trash=true"); n != 1 {
		t.Fatalf("lixeira de marketplaces: %v", n)
	}
	// o detalhe de um item na lixeira só aparece com ?trash=true
	r := httptest.NewRequest(http.MethodGet, "/?trash=true", nil)
	r.SetPathValue("id", chID)
	w := httptest.NewRecorder()
	c.ChannelItem(w, auth.WithSession(r, s))
	if w.Code != http.StatusOK || decoded(t, w)["deleted_at"] == nil {
		t.Fatalf("detalhe de canal na lixeira: %d %s", w.Code, w.Body.String())
	}
	errorEnvelope(t, item(t, c.ChannelItem, s, http.MethodGet, chID, nil), http.StatusNotFound, "not_found")
}
