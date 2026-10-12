package redirect

import (
	"bytes"
	"log/slog"
	"net/http"
	"strings"

	"github.com/hublinks/hublinks/internal/domain"
	webtmpl "github.com/hublinks/hublinks/web"
)

// preview responde a crawlers conhecidos (WhatsApp, Telegram…) com a página de
// prévia Open Graph: 200, sem Location e sem evento. Visitantes humanos nunca
// a recebem; a resposta varia pelo user-agent, então não pode ser reaproveitada.
func (h *Handler) preview(w http.ResponseWriter, r *http.Request, l domain.AffiliateLink) {
	data := struct {
		Title, Marketplace, URL, DestinationURL string
		ImageURL                                *string
	}{l.Title, l.Marketplace.Name, strings.TrimSuffix(h.BaseURL, "/") + r.URL.Path, l.DestinationURL, l.ImageURL}
	var buf bytes.Buffer
	if err := webtmpl.Render(&buf, "public", "public/preview", data); err != nil {
		slog.Error("falha ao renderizar a prévia", "error", err)
		http.Error(w, "erro ao renderizar página", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Vary", "User-Agent")
	_, _ = w.Write(buf.Bytes())
}
