package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerExposesEveryDocumentedMetric(t *testing.T) {
	m := New()
	m.Enqueued.Inc()
	m.HTTPRequests.WithLabelValues("GET /", "302").Inc()
	m.RedirectDuration.Observe(0.001)
	w := httptest.NewRecorder()
	m.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := w.Body.String()
	for _, name := range []string{
		"hublinks_events_enqueued_total", "hublinks_events_dropped_total", "hublinks_events_written_total",
		"hublinks_events_write_errors_total", "hublinks_event_queue_length",
		"hublinks_http_requests_total", "hublinks_redirect_duration_seconds_bucket",
		"hublinks_maintenance_duration_seconds", "hublinks_maintenance_failures_total", "hublinks_maintenance_last_success_timestamp_seconds",
	} {
		if !strings.Contains(body, name) {
			t.Errorf("métrica %s ausente", name)
		}
	}
}

func TestInstancesDoNotShareState(t *testing.T) {
	a, b := New(), New()
	a.Dropped.Inc()
	w := httptest.NewRecorder()
	b.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(w.Body.String(), "hublinks_events_dropped_total 0") {
		t.Fatal("cada instância deve ter o próprio registro")
	}
}
