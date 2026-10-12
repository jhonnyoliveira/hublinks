package server_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hublinks/hublinks/internal/events"
	"github.com/hublinks/hublinks/internal/metrics"
	"github.com/hublinks/hublinks/internal/server"
)

type nopWriter struct{}

func (nopWriter) Write(context.Context, []events.Event) error { return nil }

func TestMetricsEndpointExposesQueueCounters(t *testing.T) {
	m := metrics.New()
	q := events.NewQueue(1, 1, time.Hour, nopWriter{}, m)
	q.Enqueue(events.Event{})
	q.Enqueue(events.Event{}) // fila cheia: descartado
	srv := httptest.NewServer(server.MetricsHandler(m))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	text := string(body)
	for _, want := range []string{"hublinks_events_enqueued_total 1", "hublinks_events_dropped_total 1", "hublinks_events_written_total 0", "hublinks_events_write_errors_total 0", "hublinks_event_queue_length 1"} {
		if !strings.Contains(text, want) {
			t.Fatalf("faltou %q:\n%s", want, text)
		}
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	// só /metrics é servido nesta porta
	r2, _ := http.Get(srv.URL + "/admin")
	r2.Body.Close()
	if r2.StatusCode != http.StatusNotFound {
		t.Fatalf("/admin na porta de métricas: %d", r2.StatusCode)
	}
}
