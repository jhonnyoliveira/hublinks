package redirect

import (
	"strings"
	"sync"
	"time"
)

var crawlers = []string{"whatsapp", "telegrambot", "facebookexternalhit", "facebot", "twitterbot", "slackbot", "linkedinbot", "discordbot", "googlebot", "bingbot", "applebot", "pinterest", "redditbot", "skypeuripreview", "vkshare"}

func IsCrawler(ua string) bool {
	ua = strings.ToLower(ua)
	for _, s := range crawlers {
		if strings.Contains(ua, s) {
			return true
		}
	}
	return false
}

// BotDetector marca como automação o IP que acessa muitos códigos distintos
// em pouco tempo (janela deslizante em memória, por chave de IP já protegida
// por HMAC). Entradas expiradas são removidas aos poucos, a cada janela.
type BotDetector struct {
	mu        sync.Mutex
	seen      map[string]map[string]time.Time
	window    time.Duration
	limit     int
	lastSweep time.Time
}

func NewBotDetector(limit int, window time.Duration) *BotDetector {
	return &BotDetector{seen: map[string]map[string]time.Time{}, limit: limit, window: window}
}

// Bot registra o acesso de key ao código e informa se, com ele, a chave atingiu
// o limite de códigos distintos dentro da janela (o acesso e os seguintes da
// janela são robôs; os anteriores já foram gravados como reais).
func (b *BotDetector) Bot(key, code string, now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sweep(now)
	m := b.seen[key]
	if m == nil {
		m = map[string]time.Time{}
		b.seen[key] = m
	}
	for c, t := range m {
		if now.Sub(t) > b.window {
			delete(m, c)
		}
	}
	m[code] = now
	return len(m) >= b.limit
}

// sweep remove, no máximo uma vez por janela, as chaves sem acessos recentes;
// sem isso o mapa cresceria com cada IP já visto.
func (b *BotDetector) sweep(now time.Time) {
	if now.Sub(b.lastSweep) < b.window {
		return
	}
	b.lastSweep = now
	for key, m := range b.seen {
		for c, t := range m {
			if now.Sub(t) > b.window {
				delete(m, c)
			}
		}
		if len(m) == 0 {
			delete(b.seen, key)
		}
	}
}

// Tracked devolve quantas chaves estão sendo acompanhadas (para testes e métricas).
func (b *BotDetector) Tracked() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.seen)
}
