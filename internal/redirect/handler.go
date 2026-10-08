package redirect

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/domain"
	"github.com/hublinks/hublinks/internal/events"
	"github.com/hublinks/hublinks/internal/httpx"
	webtmpl "github.com/hublinks/hublinks/web"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"time"
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

// EventRecorder enfileira o evento de clique de links rastreáveis, classificando
// bots; a gravação em lote é assíncrona (events.Queue).
type EventRecorder struct {
	Queue          *events.Queue
	Pepper         string
	TrustedProxies []netip.Prefix
	Bots           *BotDetector
}

func (e EventRecorder) Record(r *http.Request, l domain.AffiliateLink, code, channel string) {
	if !l.Trackable() || e.Queue == nil {
		return
	}
	ip := httpx.ClientIP(r, e.TrustedProxies)
	bot := false
	if e.Bots != nil {
		bot = e.Bots.Bot(string(httpx.Key(e.Pepper, ip)), code, time.Now())
	}
	e.Queue.Enqueue(events.Event{OccurredAt: time.Now().UTC(), OrgID: l.OrgID.String(), TargetID: l.ID.String(), ChannelID: channel, Code: code, VisitorID: events.VisitorID(e.Pepper, ip, r.UserAgent()), Referer: cut(r.Referer(), 1024), UserAgent: cut(r.UserAgent(), 512), IsBot: bot})
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
func (h *Handler) preview(w http.ResponseWriter, r *http.Request, l domain.AffiliateLink) {
	u := strings.TrimSuffix(h.BaseURL, "/") + r.URL.Path
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := struct {
		Title, Marketplace, URL, DestinationURL string
		ImageURL                                *string
	}{l.Title, l.Marketplace.Name, u, l.DestinationURL, l.ImageURL}
	if err := webtmpl.Render(w, "public", "public/preview", data); err != nil {
		http.Error(w, "erro ao renderizar página", http.StatusInternalServerError)
	}
}
func cut(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
