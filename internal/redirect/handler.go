package redirect

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/domain"
	webtmpl "github.com/hublinks/hublinks/web"
	"log/slog"
	"net/http"
	"strings"
)

type Handler struct {
	Catalog Catalog
	Cache   *Cache
	BaseURL string
	// Recorder registra a visita depois que o destino foi resolvido. Nil
	// equivale a não registrar nada (comportamento da US1).
	Recorder Recorder
}

// Recorder é o ponto de extensão de registro de visitas. Record não pode
// bloquear nem falhar o redirecionamento (FR-012).
type Recorder interface {
	Record(r *http.Request, link domain.AffiliateLink, code, channelID string)
}

type Catalog interface {
	Resolve(context.Context, string) (domain.AffiliateLink, error)
	Channel(context.Context, uuid.UUID, string) (uuid.UUID, error)
}

// ServeHTTP only accepts the public one- and two-part URL forms.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 1 || len(parts) > 2 || !domain.CodeRE.MatchString(parts[len(parts)-1]) {
		h.notFound(w)
		return
	}
	code := parts[len(parts)-1]
	l, err := h.Cache.Resolve(r.Context(), code, h.Catalog.Resolve)
	if err != nil {
		h.lookupFailed(w, err)
		return
	}
	channel := ""
	if len(parts) == 2 {
		cid, err := h.Cache.Channel(r.Context(), l.OrgID, parts[0], h.Catalog.Channel)
		if err != nil {
			h.lookupFailed(w, err)
			return
		}
		channel = cid.String()
	}
	if IsCrawler(r.UserAgent()) {
		h.preview(w, r, l)
		return
	}
	if h.Recorder != nil && r.Method == http.MethodGet {
		h.Recorder.Record(r, l, code, channel)
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer-when-downgrade")
	http.Redirect(w, r, l.DestinationURL, http.StatusFound)
}

// lookupFailed separa "não existe" (404) de falha de infraestrutura (500), para
// que uma queda do banco não pareça um link inexistente.
func (h *Handler) lookupFailed(w http.ResponseWriter, err error) {
	if errors.Is(err, domain.ErrNotFound) {
		h.notFound(w)
		return
	}
	slog.Error("falha ao resolver redirecionamento", "error", err)
	w.Header().Set("Cache-Control", "no-store")
	http.Error(w, "erro interno", http.StatusInternalServerError)
}
func (h *Handler) notFound(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	if err := webtmpl.Render(w, "public", "public/404", nil); err != nil {
		http.Error(w, "erro ao renderizar página", http.StatusInternalServerError)
	}
}
