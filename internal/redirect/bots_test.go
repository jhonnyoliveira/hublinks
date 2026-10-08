package redirect

import (
	"testing"
	"time"
)

func TestBots(t *testing.T) {
	if !IsCrawler("WhatsApp/2.23") || IsCrawler("Mozilla/5.0") {
		t.Fatal("detecção de crawler incorreta")
	}
	b := NewBotDetector(3, time.Second)
	now := time.Now()
	if b.Bot("ip", "aaaaaaa", now) || b.Bot("ip", "bbbbbbb", now) || !b.Bot("ip", "ccccccc", now) {
		t.Fatal("limite de automação incorreto")
	}
	if b.Bot("ip", "ddddddd", now.Add(2*time.Second)) {
		t.Fatal("janela expirada não foi limpa")
	}
}
