package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hublinks/hublinks/internal/auth"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
}

func csrfCall(t *testing.T, method, path, origin, token string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, "https://hub.example"+path, nil)
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	if token != "" {
		r.Header.Set("X-CSRF-Token", token)
	}
	w := httptest.NewRecorder()
	auth.RequireCSRF(okHandler()).ServeHTTP(w, auth.WithSession(r, auth.Session{CSRF: "tok"}))
	return w
}

func envelopeCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var v struct {
		Error struct{ Code, Message string }
	}
	if err := json.NewDecoder(w.Body).Decode(&v); err != nil {
		t.Fatalf("corpo não é JSON: %v", err)
	}
	if w.Header().Get("Content-Type") != "application/json" || v.Error.Message == "" {
		t.Fatalf("envelope inválido: %#v %#v", w.Header(), v)
	}
	return v.Error.Code
}

func TestRequireCSRFOnAPIUsesJSONEnvelope(t *testing.T) {
	w := csrfCall(t, http.MethodPost, "/api/v1/channels", "", "")
	if w.Code != http.StatusForbidden || envelopeCode(t, w) != "forbidden" {
		t.Fatalf("sem token: %d %s", w.Code, w.Body.String())
	}
	w = csrfCall(t, http.MethodPost, "/api/v1/channels", "", "errado")
	if w.Code != http.StatusForbidden {
		t.Fatalf("token errado: %d", w.Code)
	}
}

func TestRequireCSRFAcceptsTokenAndSameOrigin(t *testing.T) {
	for name, origin := range map[string]string{"sem Origin": "", "mesma origem": "https://hub.example"} {
		if w := csrfCall(t, http.MethodPatch, "/api/v1/channels/1", origin, "tok"); w.Code != http.StatusNoContent {
			t.Fatalf("%s: %d", name, w.Code)
		}
	}
	if w := csrfCall(t, http.MethodGet, "/api/v1/channels", "https://outro.example", ""); w.Code != http.StatusNoContent {
		t.Fatalf("GET não exige token nem origem: %d", w.Code)
	}
}

func TestRequireCSRFRejectsCrossOriginEvenWithValidToken(t *testing.T) {
	for _, origin := range []string{"https://evil.example", "https://hub.example.evil.com", "null", "::::"} {
		w := csrfCall(t, http.MethodDelete, "/api/v1/channels/1", origin, "tok")
		if w.Code != http.StatusForbidden || envelopeCode(t, w) != "forbidden" {
			t.Fatalf("origem %q aceita: %d", origin, w.Code)
		}
	}
}

func TestRequireCSRFOnPanelKeepsPlainText(t *testing.T) {
	w := csrfCall(t, http.MethodPost, "/admin/links", "", "")
	if w.Code != http.StatusForbidden || w.Header().Get("Content-Type") == "application/json" {
		t.Fatalf("painel: %d %q", w.Code, w.Header().Get("Content-Type"))
	}
}

func TestRequireSessionWithoutCookie(t *testing.T) {
	var m auth.Manager
	api := httptest.NewRecorder()
	m.Require(okHandler(), true).ServeHTTP(api, httptest.NewRequest(http.MethodGet, "/api/v1/channels", nil))
	if api.Code != http.StatusUnauthorized || envelopeCode(t, api) != "unauthorized" {
		t.Fatalf("API sem sessão: %d %s", api.Code, api.Body.String())
	}
	panel := httptest.NewRecorder()
	m.Require(okHandler(), false).ServeHTTP(panel, httptest.NewRequest(http.MethodGet, "/admin/links", nil))
	if panel.Code != http.StatusSeeOther || panel.Header().Get("Location") != "/admin/login?next=/admin/links" {
		t.Fatalf("painel sem sessão deve redirecionar: %d %q", panel.Code, panel.Header().Get("Location"))
	}
}
