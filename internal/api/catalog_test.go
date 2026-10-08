package api_test

import (
	"bytes"
	"context"
	"encoding/json"
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
)

func apiCatalog(t *testing.T) (api.Catalog, auth.Session) {
	t.Helper()
	p := testdb.New(t)
	org := uuid.New()
	if _, err := p.Exec(context.Background(), "INSERT INTO organizations(id,name,slug,created_at) VALUES($1,$2,$3,$4)", org, "Org", org.String(), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	return api.Catalog{Service: service.Catalog{Store: store.NewCatalog(p), BaseURL: "https://hub.example"}}, auth.Session{OrgID: org}
}
func call(t *testing.T, h http.HandlerFunc, s auth.Session, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var data []byte
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	r := httptest.NewRequest(method, target, bytes.NewReader(data))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h(w, auth.WithSession(r, s))
	return w
}
func decoded(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.NewDecoder(w.Body).Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestCatalogAPIProducesTrackableURLsAndTrashListings(t *testing.T) {
	c, session := apiCatalog(t)
	w := call(t, c.Marketplace, session, http.MethodPost, "/api/v1/marketplaces", map[string]any{"name": "Loja", "shorten_policy": "shorten"})
	if w.Code != http.StatusCreated {
		t.Fatalf("marketplace: %d %s", w.Code, w.Body.String())
	}
	mpID := decoded(t, w)["id"].(string)
	w = call(t, c.Channel, session, http.MethodPost, "/api/v1/channels", map[string]any{"name": "WhatsApp", "segment": "wapp"})
	if w.Code != http.StatusCreated {
		t.Fatalf("canal: %d", w.Code)
	}
	w = call(t, c.Link, session, http.MethodPost, "/api/v1/affiliate-links", map[string]any{"title": "Produto", "destination_url": "https://example.com/p", "marketplace_id": mpID})
	if w.Code != http.StatusCreated {
		t.Fatalf("link: %d %s", w.Code, w.Body.String())
	}
	v := decoded(t, w)
	if v["trackable"] != true || v["effective_policy"] != "shorten" || len(v["urls"].([]any)) != 2 {
		t.Fatalf("URLs rastreáveis ausentes: %#v", v)
	}
	linkID := v["id"].(string)
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/affiliate-links/"+linkID, nil)
	r.SetPathValue("id", linkID)
	w = httptest.NewRecorder()
	c.LinkItem(w, auth.WithSession(r, session))
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	w = call(t, c.Link, session, http.MethodGet, "/api/v1/affiliate-links?trash=true", nil)
	if w.Code != http.StatusOK || decoded(t, w)["total"].(float64) != 1 {
		t.Fatalf("lixeira: %d %s", w.Code, w.Body.String())
	}
}

func TestCatalogAPIDirectLinkAndChannelDetails(t *testing.T) {
	c, session := apiCatalog(t)
	w := call(t, c.Marketplace, session, http.MethodPost, "/api/v1/marketplaces", map[string]any{"name": "Loja", "shorten_policy": "shorten"})
	mpID := decoded(t, w)["id"].(string)
	if w = call(t, c.Marketplace, session, http.MethodGet, "/api/v1/marketplaces", nil); w.Code != http.StatusOK || decoded(t, w)["total"].(float64) != 1 {
		t.Fatalf("lista de marketplaces: %d", w.Code)
	}
	w = call(t, c.Channel, session, http.MethodPost, "/api/v1/channels", map[string]any{"name": "WhatsApp", "segment": "wapp"})
	if w.Code != http.StatusCreated {
		t.Fatal(w.Code)
	}
	channelID := decoded(t, w)["id"].(string)
	w = call(t, c.Link, session, http.MethodPost, "/api/v1/affiliate-links", map[string]any{"title": "Direto", "destination_url": "https://example.com/original", "marketplace_id": mpID, "shorten_policy_override": "direct"})
	if w.Code != http.StatusCreated {
		t.Fatalf("link direto: %d %s", w.Code, w.Body.String())
	}
	direct := decoded(t, w)
	original := direct["urls"].([]any)[0].(map[string]any)
	if direct["trackable"] != false || direct["effective_policy"] != "direct" || len(direct["urls"].([]any)) != 1 || original["label"] != "original" || original["url"] != "https://example.com/original" {
		t.Fatalf("formato de link direto: %#v", direct)
	}
	r := httptest.NewRequest(http.MethodGet, "/api/v1/channels/"+channelID, nil)
	r.SetPathValue("id", channelID)
	w = httptest.NewRecorder()
	c.ChannelItem(w, auth.WithSession(r, session))
	if w.Code != http.StatusOK || decoded(t, w)["active_links_count"].(float64) != 1 {
		t.Fatalf("detalhe de canal: %d %s", w.Code, w.Body.String())
	}
	r = httptest.NewRequest(http.MethodDelete, "/api/v1/channels/"+channelID, nil)
	r.SetPathValue("id", channelID)
	w = httptest.NewRecorder()
	c.ChannelItem(w, auth.WithSession(r, session))
	if w.Code != http.StatusNoContent {
		t.Fatalf("exclusão do canal: %d", w.Code)
	}
}

func TestCatalogAPIMarketplaceConflictAndRestore(t *testing.T) {
	c, session := apiCatalog(t)
	w := call(t, c.Marketplace, session, http.MethodPost, "/api/v1/marketplaces", map[string]any{"name": "Loja", "shorten_policy": "shorten"})
	mpID := decoded(t, w)["id"].(string)
	w = call(t, c.Link, session, http.MethodPost, "/api/v1/affiliate-links", map[string]any{"title": "Produto", "destination_url": "https://example.com", "marketplace_id": mpID})
	if w.Code != http.StatusCreated {
		t.Fatal(w.Code)
	}
	linkID := decoded(t, w)["id"].(string)
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/marketplaces/"+mpID, nil)
	r.SetPathValue("id", mpID)
	w = httptest.NewRecorder()
	c.MarketplaceItem(w, auth.WithSession(r, session))
	if w.Code != http.StatusConflict || len(decoded(t, w)["links"].([]any)) != 1 {
		t.Fatalf("conflito de marketplace: %d %s", w.Code, w.Body.String())
	}
	r = httptest.NewRequest(http.MethodDelete, "/api/v1/affiliate-links/"+linkID, nil)
	r.SetPathValue("id", linkID)
	w = httptest.NewRecorder()
	c.LinkItem(w, auth.WithSession(r, session))
	if w.Code != http.StatusNoContent {
		t.Fatal(w.Code)
	}
	r = httptest.NewRequest(http.MethodPost, "/api/v1/affiliate-links/"+linkID+"/restore", nil)
	r.SetPathValue("id", linkID)
	w = httptest.NewRecorder()
	c.LinkRestore(w, auth.WithSession(r, session))
	if w.Code != http.StatusOK {
		t.Fatalf("restauração: %d %s", w.Code, w.Body.String())
	}
}

func TestCatalogAPIUpdatesAndRestoresEachResource(t *testing.T) {
	c, session := apiCatalog(t)
	w := call(t, c.Marketplace, session, http.MethodPost, "/api/v1/marketplaces", map[string]any{"name": "Loja", "shorten_policy": "shorten"})
	mpID := decoded(t, w)["id"].(string)
	patch := func(h http.HandlerFunc, id string, body any) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPatch, "/", bytes.NewReader(mustJSON(t, body)))
		r.SetPathValue("id", id)
		w := httptest.NewRecorder()
		h(w, auth.WithSession(r, session))
		return w
	}
	if w = patch(c.MarketplaceItem, mpID, map[string]any{"name": "Nova Loja", "shorten_policy": "direct"}); w.Code != http.StatusOK {
		t.Fatalf("patch marketplace: %d", w.Code)
	}
	r := httptest.NewRequest(http.MethodDelete, "/", nil)
	r.SetPathValue("id", mpID)
	w = httptest.NewRecorder()
	c.MarketplaceItem(w, auth.WithSession(r, session))
	if w.Code != http.StatusNoContent {
		t.Fatal(w.Code)
	}
	r = httptest.NewRequest(http.MethodPost, "/", nil)
	r.SetPathValue("id", mpID)
	w = httptest.NewRecorder()
	c.MarketplaceRestore(w, auth.WithSession(r, session))
	if w.Code != http.StatusOK {
		t.Fatal(w.Code)
	}
	w = call(t, c.Channel, session, http.MethodPost, "/api/v1/channels", map[string]any{"name": "WhatsApp", "segment": "wapp"})
	channelID := decoded(t, w)["id"].(string)
	if w = call(t, c.Channel, session, http.MethodGet, "/api/v1/channels", nil); w.Code != http.StatusOK || decoded(t, w)["total"].(float64) != 1 {
		t.Fatalf("lista de canais: %d", w.Code)
	}
	if w = patch(c.ChannelItem, channelID, map[string]any{"name": "WhatsApp novo", "segment": "zap"}); w.Code != http.StatusOK {
		t.Fatalf("patch canal: %d", w.Code)
	}
	r = httptest.NewRequest(http.MethodDelete, "/", nil)
	r.SetPathValue("id", channelID)
	w = httptest.NewRecorder()
	c.ChannelItem(w, auth.WithSession(r, session))
	if w.Code != http.StatusNoContent {
		t.Fatal(w.Code)
	}
	r = httptest.NewRequest(http.MethodPost, "/", nil)
	r.SetPathValue("id", channelID)
	w = httptest.NewRecorder()
	c.ChannelRestore(w, auth.WithSession(r, session))
	if w.Code != http.StatusOK {
		t.Fatal(w.Code)
	}
	w = call(t, c.Link, session, http.MethodPost, "/api/v1/affiliate-links", map[string]any{"title": "Produto", "destination_url": "https://example.com", "marketplace_id": mpID})
	linkID := decoded(t, w)["id"].(string)
	r = httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetPathValue("id", linkID)
	w = httptest.NewRecorder()
	c.LinkItem(w, auth.WithSession(r, session))
	if w.Code != http.StatusOK {
		t.Fatalf("detalhe do link: %d", w.Code)
	}
	if w = patch(c.LinkItem, linkID, map[string]any{"title": "Produto novo", "active": false}); w.Code != http.StatusOK {
		t.Fatalf("patch link: %d %s", w.Code, w.Body.String())
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
