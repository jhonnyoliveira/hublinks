package redirect

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/domain"
)

type catalogStub struct {
	link       domain.AffiliateLink
	resolveErr error
	channels   map[string]uuid.UUID
}

func (s catalogStub) Resolve(context.Context, string) (domain.AffiliateLink, error) {
	return s.link, s.resolveErr
}
func (s catalogStub) Channel(_ context.Context, _ uuid.UUID, segment string) (uuid.UUID, error) {
	id, ok := s.channels[segment]
	if !ok {
		return uuid.Nil, domain.ErrNotFound
	}
	return id, nil
}

func TestHandlerRedirectsPublicURLsWithoutCookies(t *testing.T) {
	org := uuid.New()
	stub := catalogStub{link: domain.AffiliateLink{ID: uuid.New(), OrgID: org, Code: "abc1234", DestinationURL: "https://example.com/destino", Active: true, Marketplace: domain.Marketplace{ShortenPolicy: domain.PolicyShorten}}, channels: map[string]uuid.UUID{"wapp": uuid.New()}}
	h := &Handler{Catalog: stub, Cache: NewCache()}
	for _, path := range []string{"/abc1234", "/wapp/abc1234"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusFound || w.Header().Get("Location") != "https://example.com/destino" {
			t.Fatalf("%s: resposta = %d, location=%q", path, w.Code, w.Header().Get("Location"))
		}
		if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Set-Cookie") != "" {
			t.Fatalf("%s: cabeçalhos públicos incorretos: %#v", path, w.Header())
		}
	}
}
func TestHandlerReturnsNotFoundForInvalidAndUnknownURLs(t *testing.T) {
	h := &Handler{Catalog: catalogStub{resolveErr: domain.ErrNotFound}, Cache: NewCache()}
	for _, path := range []string{"/curto", "/abc1234", "/naoexiste/abc1234"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s: status %d", path, w.Code)
		}
	}
}
func TestHandlerRedirectsDirectLink(t *testing.T) {
	h := &Handler{Catalog: catalogStub{link: domain.AffiliateLink{ID: uuid.New(), OrgID: uuid.New(), Code: "abc1234", DestinationURL: "https://example.com", Active: true, Marketplace: domain.Marketplace{ShortenPolicy: domain.PolicyDirect}}}, Cache: NewCache()}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/abc1234", nil))
	if w.Code != http.StatusFound {
		t.Fatalf("status %d", w.Code)
	}
}
