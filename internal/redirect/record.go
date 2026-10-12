package redirect

import (
	"net/http"
	"net/netip"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hublinks/hublinks/internal/domain"
	"github.com/hublinks/hublinks/internal/events"
	"github.com/hublinks/hublinks/internal/httpx"
)

// EventRecorder enfileira o evento de clique de links rastreáveis, classificando
// bots; a gravação em lote é assíncrona (events.Queue) e nunca bloqueia.
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
	now := time.Now().UTC()
	ip := httpx.ClientIP(r, e.TrustedProxies)
	bot := false
	if e.Bots != nil {
		bot = e.Bots.Bot(string(httpx.Key(e.Pepper, ip)), code, now)
	}
	e.Queue.Enqueue(events.Event{OccurredAt: now, OrgID: l.OrgID.String(), TargetID: l.ID.String(), ChannelID: channel, Code: code, VisitorID: events.VisitorID(e.Pepper, ip, r.UserAgent()), Referer: cut(r.Referer(), 1024), UserAgent: cut(r.UserAgent(), 512), IsBot: bot})
}

// cut limita s a n bytes sem partir um caractere UTF-8 e remove NULs: o
// PostgreSQL rejeita ambos em text, o que derrubaria o lote inteiro.
func cut(s string, n int) string {
	s = strings.ReplaceAll(s, "\x00", "")
	if len(s) > n {
		s = s[:n]
		for len(s) > 0 && !utf8.ValidString(s) {
			s = s[:len(s)-1]
		}
	}
	return strings.ToValidUTF8(s, "")
}
