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
	if v["trackable"] != true || len(v["urls"].([]any)) != 2 {
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
