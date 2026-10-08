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

type BotDetector struct {
	mu     sync.Mutex
	seen   map[string]map[string]time.Time
	window time.Duration
	limit  int
}

func NewBotDetector(limit int, window time.Duration) *BotDetector {
	return &BotDetector{seen: map[string]map[string]time.Time{}, limit: limit, window: window}
}
func (b *BotDetector) Bot(key, code string, now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
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
