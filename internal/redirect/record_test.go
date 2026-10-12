package redirect

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/domain"
	"github.com/hublinks/hublinks/internal/events"
)

type captureWriter struct {
	mu     sync.Mutex
	events []events.Event
}

func (w *captureWriter) Write(_ context.Context, e []events.Event) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.events = append(w.events, e...)
	return nil
}
func (w *captureWriter) all() []events.Event {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]events.Event(nil), w.events...)
}

func recordHandler(policy domain.Policy, q *events.Queue) (*Handler, domain.AffiliateLink, uuid.UUID) {
	channel := uuid.New()
	link := domain.AffiliateLink{ID: uuid.New(), OrgID: uuid.New(), Code: "abc1234", DestinationURL: "https://example.com/d", Active: true, Marketplace: domain.Marketplace{ShortenPolicy: policy}}
	stub := catalogStub{link: link, channels: map[string]uuid.UUID{"wapp": channel}}
	rec := EventRecorder{Queue: q, Pepper: strings.Repeat("p", 32), Bots: NewBotDetector(5, time.Minute)}
	return &Handler{Catalog: stub, Cache: NewCache(), Recorder: rec}, link, channel
}

func TestRecorderEnqueuesRealAccess(t *testing.T) {
	w := &captureWriter{}
	q := events.NewQueue(10, 1, time.Hour, w, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go q.Run(ctx)
	h, link, channel := recordHandler(domain.PolicyShorten, q)
	r := httptest.NewRequest(http.MethodGet, "/wapp/abc1234", nil)
	r.RemoteAddr = "203.0.113.9:1234"
	r.Header.Set("Referer", strings.Repeat("r", 2000))
	r.Header.Set("User-Agent", strings.Repeat("u", 900))
	before := time.Now().UTC()
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, r)
	if rw.Code != http.StatusFound {
		t.Fatalf("status %d", rw.Code)
	}
	deadline := time.Now().Add(2 * time.Second)
	for len(w.all()) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	got := w.all()
	if len(got) != 1 {
		t.Fatalf("eventos: %d", len(got))
	}
	e := got[0]
	if e.OccurredAt.Before(before) || e.OccurredAt.Location() != time.UTC || e.ChannelID != channel.String() || e.TargetID != link.ID.String() || e.OrgID != link.OrgID.String() || e.Code != "abc1234" {
		t.Fatalf("evento = %+v", e)
	}
	if len(e.Referer) != 1024 || len(e.UserAgent) != 512 || len(e.VisitorID) != 32 || e.IsBot {
		t.Fatalf("referer=%d ua=%d visitor=%d bot=%v", len(e.Referer), len(e.UserAgent), len(e.VisitorID), e.IsBot)
	}
}

func TestRecorderSkipsDirectLink(t *testing.T) {
	q := events.NewQueue(10, 1, time.Hour, &captureWriter{}, nil)
	h, _, _ := recordHandler(domain.PolicyDirect, q)
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/abc1234", nil))
	if rw.Code != http.StatusFound {
		t.Fatalf("status %d", rw.Code)
	}
	if q.Len() != 0 {
		t.Fatal("link direct não deve enfileirar")
	}
}

func TestRecorderStoppedWorkerStillRedirects(t *testing.T) {
	q := events.NewQueue(1, 1, time.Hour, &captureWriter{}, nil) // sem worker, fila de 1
	h, _, _ := recordHandler(domain.PolicyShorten, q)
	for range 5 {
		rw := httptest.NewRecorder()
		h.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/abc1234", nil))
		if rw.Code != http.StatusFound {
			t.Fatalf("status %d", rw.Code)
		}
	}
	if q.Dropped() != 4 {
		t.Fatalf("descartados: %d", q.Dropped())
	}
}

func TestCutKeepsValidUTF8(t *testing.T) {
	got := cut("aé", 2) // "é" ocupa 2 bytes e não cabe inteiro
	if got != "a" {
		t.Fatalf("cut = %q", got)
	}
	if got := cut("a\x00b", 10); got != "ab" {
		t.Fatalf("cut = %q", got)
	}
}
