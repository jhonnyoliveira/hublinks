package redirect

import (
	"context"
	"errors"
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

func TestHandlerReturns500WhenCatalogFails(t *testing.T) {
	h := &Handler{Catalog: catalogStub{resolveErr: errors.New("banco indisponível")}, Cache: NewCache()}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/abc1234", nil))
	if w.Code != http.StatusInternalServerError || w.Header().Get("Set-Cookie") != "" || w.Header().Get("Location") != "" {
		t.Fatalf("falha de infraestrutura deve ser 500 sem cookie nem Location: %d %#v", w.Code, w.Header())
	}
}

type recorderSpy struct {
	calls   int
	code    string
	channel string
}

func (s *recorderSpy) Record(_ *http.Request, _ domain.AffiliateLink, code, channel string) {
	s.calls++
	s.code, s.channel = code, channel
}

func trackedHandler(spy *recorderSpy) (*Handler, uuid.UUID) {
	ch := uuid.New()
	stub := catalogStub{link: domain.AffiliateLink{ID: uuid.New(), OrgID: uuid.New(), Code: "abc1234", DestinationURL: "https://example.com/d", Active: true, Marketplace: domain.Marketplace{ShortenPolicy: domain.PolicyShorten}}, channels: map[string]uuid.UUID{"wapp": ch}}
	return &Handler{Catalog: stub, Cache: NewCache(), Recorder: spy, BaseURL: "https://hub.example"}, ch
}

func TestHandlerCallsRecorderOnlyForRealRedirects(t *testing.T) {
	spy := &recorderSpy{}
	h, ch := trackedHandler(spy)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/wapp/abc1234", nil))
	if w.Code != http.StatusFound || spy.calls != 1 || spy.code != "abc1234" || spy.channel != ch.String() {
		t.Fatalf("GET: status=%d chamadas=%d code=%q canal=%q", w.Code, spy.calls, spy.code, spy.channel)
	}
	if w.Header().Get("Referrer-Policy") != "no-referrer-when-downgrade" {
		t.Fatalf("Referrer-Policy: %q", w.Header().Get("Referrer-Policy"))
	}

	// código curto sem canal: channelID vazio
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/abc1234", nil))
	if spy.calls != 2 || spy.channel != "" {
		t.Fatalf("sem canal: chamadas=%d canal=%q", spy.calls, spy.channel)
	}

	// HEAD resolve como GET, mas não registra visita
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodHead, "/abc1234", nil))
	if w.Code != http.StatusFound || spy.calls != 2 {
		t.Fatalf("HEAD: status=%d chamadas=%d", w.Code, spy.calls)
	}

	// crawler recebe prévia (200) e não registra visita
	r := httptest.NewRequest(http.MethodGet, "/abc1234", nil)
	r.Header.Set("User-Agent", "facebookexternalhit/1.1")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK || spy.calls != 2 || w.Header().Get("Set-Cookie") != "" {
		t.Fatalf("crawler: status=%d chamadas=%d", w.Code, spy.calls)
	}

	// 404 e métodos não suportados não registram
	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/naoexiste/abc1234", nil),
		httptest.NewRequest(http.MethodGet, "/curto", nil),
		httptest.NewRequest(http.MethodPost, "/abc1234", nil),
	} {
		w = httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound || spy.calls != 2 {
			t.Fatalf("%s %s: status=%d chamadas=%d", req.Method, req.URL.Path, w.Code, spy.calls)
		}
	}
}

func TestHandlerWithoutRecorderStillRedirects(t *testing.T) {
	h, _ := trackedHandler(&recorderSpy{})
	h.Recorder = nil
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/abc1234", nil))
	if w.Code != http.StatusFound {
		t.Fatalf("status %d", w.Code)
	}
}
