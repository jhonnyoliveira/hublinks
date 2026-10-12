package redirect

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/domain"
)

func previewHandler(image *string) (*Handler, *recorderSpy) {
	spy := &recorderSpy{}
	link := domain.AffiliateLink{ID: uuid.New(), OrgID: uuid.New(), Code: "abc1234", Title: "Fone <b>BT</b> & cia", DestinationURL: "https://loja.example/produto?a=1&b=2", ImageURL: image, Active: true,
		Marketplace: domain.Marketplace{Name: "Loja Exemplo", ShortenPolicy: domain.PolicyShorten}}
	stub := catalogStub{link: link, channels: map[string]uuid.UUID{"wapp": uuid.New()}}
	return &Handler{Catalog: stub, Cache: NewCache(), Recorder: spy, BaseURL: "https://hub.example/"}, spy
}

func crawl(h *Handler, path, ua string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.Header.Set("User-Agent", ua)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestPreviewWithImage(t *testing.T) {
	img := "https://cdn.loja.example/fone.jpg"
	h, spy := previewHandler(&img)
	for _, path := range []string{"/abc1234", "/wapp/abc1234"} {
		w := crawl(h, path, "WhatsApp/2.23")
		if w.Code != http.StatusOK || w.Header().Get("Location") != "" || spy.calls != 0 {
			t.Fatalf("%s: status=%d location=%q eventos=%d", path, w.Code, w.Header().Get("Location"), spy.calls)
		}
		body := w.Body.String()
		for _, want := range []string{
			`<meta property="og:title" content="Fone &lt;b&gt;BT&lt;/b&gt; &amp; cia">`,
			`<meta property="og:description" content="Disponível em Loja Exemplo">`,
			`<meta property="og:url" content="https://hub.example` + path + `">`, // absoluta, sem barra duplicada
			`<meta property="og:type" content="website">`,
			`<meta property="og:image" content="` + img + `">`,
			`<meta name="twitter:card" content="summary_large_image">`,
			`<meta name="robots" content="noindex">`,
			`href="/privacidade"`,
			`width="600" height="314"`,
			`href="https://loja.example/produto?a=1&amp;b=2"`,
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s: faltou %s\n%s", path, want, body)
			}
		}
		if strings.Contains(body, "<b>BT</b>") {
			t.Fatal("título deveria estar escapado")
		}
		if w.Header().Get("Set-Cookie") != "" || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Vary") != "User-Agent" {
			t.Fatalf("cabeçalhos: %v", w.Header())
		}
	}
}

func TestPreviewWithoutImage(t *testing.T) {
	h, spy := previewHandler(nil)
	w := crawl(h, "/abc1234", "TelegramBot (like TwitterBot)")
	body := w.Body.String()
	if w.Code != http.StatusOK || spy.calls != 0 {
		t.Fatalf("status=%d eventos=%d", w.Code, spy.calls)
	}
	if strings.Contains(body, "og:image") || strings.Contains(body, "<img") {
		t.Fatalf("sem imagem não deve haver og:image nem <img>:\n%s", body)
	}
	if !strings.Contains(body, `<meta name="twitter:card" content="summary">`) {
		t.Fatalf("twitter:card deveria ser summary:\n%s", body)
	}
}

func TestPreviewHasNoThirdPartyResources(t *testing.T) {
	img := "https://cdn.loja.example/fone.jpg"
	h, _ := previewHandler(&img)
	body := crawl(h, "/abc1234", "facebookexternalhit/1.1").Body.String()
	allowed := map[string]bool{img: true, "https://hub.example/abc1234": true, "https://loja.example/produto?a=1&amp;b=2": true}
	for _, u := range regexp.MustCompile(`https?://[^"'\s<>]+`).FindAllString(body, -1) {
		if !allowed[u] {
			t.Errorf("URL inesperada na prévia: %s", u)
		}
	}
	if strings.Contains(body, "<script src") || strings.Contains(body, `rel="stylesheet" href="http`) {
		t.Fatal("a prévia não deve carregar scripts nem estilos de terceiros")
	}
}

func TestHumanVisitorStillGetsRedirectNotPreview(t *testing.T) {
	h, spy := previewHandler(nil)
	w := crawl(h, "/abc1234", "Mozilla/5.0 (iPhone) Safari/604.1")
	if w.Code != http.StatusFound || spy.calls != 1 {
		t.Fatalf("status=%d eventos=%d", w.Code, spy.calls)
	}
}
