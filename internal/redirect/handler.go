package redirect

import (
	"github.com/hublinks/hublinks/internal/domain"
	"github.com/hublinks/hublinks/internal/events"
	"github.com/hublinks/hublinks/internal/httpx"
	"github.com/hublinks/hublinks/internal/store"
	webtmpl "github.com/hublinks/hublinks/web"
	"net/http"
	"net/netip"
	"strings"
	"time"
)

type Handler struct {
	Catalog         *store.Catalog
	Cache           *Cache
	Queue           *events.Queue
	Pepper, BaseURL string
	TrustedProxies  []netip.Prefix
	Bots            *BotDetector
}

// ServeHTTP only accepts the public one- and two-part URL forms.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
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
		h.notFound(w)
		return
	}
	channel := ""
	if len(parts) == 2 {
		cid, err := h.Catalog.Channel(r.Context(), l.OrgID, parts[0])
		if err != nil {
			h.notFound(w)
			return
		}
		channel = cid.String()
	}
	if IsCrawler(r.UserAgent()) {
		h.preview(w, r, l)
		return
	}
	if l.Trackable() && h.Queue != nil {
		ip := httpx.ClientIP(r, h.TrustedProxies)
		bot := false
		if h.Bots != nil {
			bot = h.Bots.Bot(string(httpx.Key(h.Pepper, ip)), code, time.Now())
		}
		h.Queue.Enqueue(events.Event{OccurredAt: time.Now().UTC(), OrgID: l.OrgID.String(), TargetID: l.ID.String(), ChannelID: channel, Code: code, VisitorID: events.VisitorID(h.Pepper, ip, r.UserAgent()), Referer: cut(r.Referer(), 1024), UserAgent: cut(r.UserAgent(), 512), IsBot: bot})
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer-when-downgrade")
	http.Redirect(w, r, l.DestinationURL, http.StatusFound)
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
