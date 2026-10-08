package assets

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerServesEmbeddedDevelopmentCSS(t *testing.T) {
	r := httptest.NewRecorder()
	Handler().ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/static/css/app.dev.css", nil))
	if r.Code != http.StatusOK {
		t.Fatalf("status %d", r.Code)
	}
	if r.Header().Get("Cache-Control") != "" {
		t.Fatal("arquivo sem hash não pode ser imutável")
	}
}
