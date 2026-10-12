package redirect

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestIsCrawlerKnownAgentsCaseInsensitive(t *testing.T) {
	for _, ua := range []string{
		"WhatsApp/2.23.20.0 A", "TelegramBot (like TwitterBot)", "facebookexternalhit/1.1", "Facebot",
		"Twitterbot/1.0", "Slackbot-LinkExpanding 1.0", "LinkedInBot/1.0", "Mozilla/5.0 (compatible; Discordbot/2.0)",
		"Mozilla/5.0 (compatible; Googlebot/2.1)", "Mozilla/5.0 (compatible; bingbot/2.0)", "Applebot/0.1",
		"Pinterest/0.2", "redditbot/1.0", "SkypeUriPreview Preview/0.5", "vkShare; +http://vk.com/dev/Share",
		"whatsapp/2.23", "TELEGRAMBOT", "FACEBOOKEXTERNALHIT",
	} {
		if !IsCrawler(ua) {
			t.Errorf("%q deveria ser crawler", ua)
		}
	}
	for _, ua := range []string{
		"", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120 Safari/537.36",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) Safari/604.1", "curl/8.4.0",
	} {
		if IsCrawler(ua) {
			t.Errorf("%q não deveria ser crawler", ua)
		}
	}
}

func TestBotDetectorFifthDistinctCodeAndFollowingAreBots(t *testing.T) {
	b := NewBotDetector(5, 10*time.Second)
	now := time.Now()
	codes := []string{"aaaaaaa", "bbbbbbb", "ccccccc", "ddddddd"}
	for i, c := range codes {
		if b.Bot("ip", c, now.Add(time.Duration(i)*time.Second)) {
			t.Fatalf("%s (%dº código) não deveria ser robô", c, i+1)
		}
	}
	for i, c := range []string{"eeeeeee", "fffffff"} {
		if !b.Bot("ip", c, now.Add(time.Duration(4+i)*time.Second)) {
			t.Fatalf("%s deveria ser robô (5º código em diante na janela)", c)
		}
	}
	// o acesso a um código já visto, dentro da janela, também é robô
	if !b.Bot("ip", "aaaaaaa", now.Add(6*time.Second)) {
		t.Fatal("seguinte da janela deveria ser robô")
	}
	// outro IP não é afetado
	if b.Bot("outro", "aaaaaaa", now) {
		t.Fatal("outro IP não deveria ser robô")
	}
}

func TestBotDetectorRepeatedCodeDoesNotCountAsDistinct(t *testing.T) {
	b := NewBotDetector(3, time.Minute)
	now := time.Now()
	for i := range 50 {
		if b.Bot("ip", "aaaaaaa", now.Add(time.Duration(i)*time.Millisecond)) {
			t.Fatal("o mesmo código repetido não é automação")
		}
	}
	if b.Bot("ip", "bbbbbbb", now) || !b.Bot("ip", "ccccccc", now) {
		t.Fatal("o 3º código distinto deveria ser o robô")
	}
}

func TestBotDetectorWindowExpiresAndResetsCount(t *testing.T) {
	b := NewBotDetector(3, 10*time.Second)
	now := time.Now()
	b.Bot("ip", "aaaaaaa", now)
	b.Bot("ip", "bbbbbbb", now)
	if !b.Bot("ip", "ccccccc", now) {
		t.Fatal("3º código deveria ser robô")
	}
	later := now.Add(11 * time.Second)
	if b.Bot("ip", "ddddddd", later) || b.Bot("ip", "eeeeeee", later) {
		t.Fatal("após a janela a contagem recomeça")
	}
	if !b.Bot("ip", "fffffff", later) {
		t.Fatal("a nova janela também detecta")
	}
}

func TestBotDetectorSweepsExpiredKeys(t *testing.T) {
	b := NewBotDetector(5, time.Second)
	now := time.Now()
	for i := range 100 {
		b.Bot(fmt.Sprintf("ip-%d", i), "aaaaaaa", now)
	}
	if b.Tracked() != 100 {
		t.Fatalf("acompanhados: %d", b.Tracked())
	}
	b.Bot("novo", "aaaaaaa", now.Add(3*time.Second)) // dispara a limpeza
	if b.Tracked() != 1 {
		t.Fatalf("chaves expiradas deveriam sair do mapa: %d", b.Tracked())
	}
}

func TestBotDetectorUsesConfiguredLimits(t *testing.T) {
	for _, limit := range []int{2, 7} {
		b := NewBotDetector(limit, time.Minute)
		now := time.Now()
		var first int
		for i := range 10 {
			if b.Bot("ip", fmt.Sprintf("code%03d", i), now) {
				first = i + 1
				break
			}
		}
		if first != limit {
			t.Fatalf("limite %d: primeiro robô no acesso %d", limit, first)
		}
	}
}

func TestBotDetectorConcurrent(t *testing.T) {
	b := NewBotDetector(5, time.Second)
	var wg sync.WaitGroup
	for g := range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 200 {
				b.Bot(fmt.Sprintf("ip-%d", g%4), fmt.Sprintf("c%d", i%9), time.Now())
			}
		}()
	}
	wg.Wait()
}
