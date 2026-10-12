package web_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hublinks/hublinks/internal/config"
	"github.com/hublinks/hublinks/internal/web"
)

func TestPrivacyPage(t *testing.T) {
	h := web.Privacy(config.Config{EventsRetentionDays: 395, BotEventsRetentionDays: 30, TrashRetentionDays: 21, PrivacyContact: "privacidade@exemplo.org"})
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest(http.MethodGet, "/privacidade", nil))
	body := w.Body.String()
	if w.Code != http.StatusOK || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("status=%d tipo=%q", w.Code, w.Header().Get("Content-Type"))
	}
	for _, want := range []string{
		"395 dias", "30 dias", "21 dias", // prazos configurados
		`href="mailto:privacidade@exemplo.org"`, "privacidade@exemplo.org",
		"identificador anônimo", "Não usamos cookies", "não é armazenado",
		"Totais diários", "enquanto o link existir",
		`id="privacy-notice"`, `href="/privacidade"`, // aviso do layout público
	} {
		if !strings.Contains(body, want) {
			t.Errorf("faltou %q", want)
		}
	}
	if w.Header().Get("Set-Cookie") != "" {
		t.Fatalf("a página pública não pode emitir cookies: %q", w.Header().Get("Set-Cookie"))
	}
	if strings.Contains(body, "consentimento") || strings.Contains(body, "Aceitar") {
		t.Fatal("o aviso não é um pedido de consentimento")
	}
}
