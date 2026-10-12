// Package metrics reúne o registro Prometheus e os contadores da fila de eventos.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics agrupa as métricas de R15. Cada instância usa o próprio registro,
// para que testes e servidores não compartilhem estado global.
type Metrics struct {
	Registry    *prometheus.Registry
	Enqueued    prometheus.Counter
	Dropped     prometheus.Counter
	Written     prometheus.Counter
	WriteErrors prometheus.Counter
	QueueLength prometheus.Gauge
	// Manutenção diária (consolidação, limpeza e lixeira).
	MaintenanceDuration    prometheus.Gauge
	MaintenanceFailures    prometheus.Counter
	MaintenanceLastSuccess prometheus.Gauge
	// HTTP: requisições por rota (padrão do mux, de cardinalidade limitada) e status,
	// e a latência dos redirecionamentos públicos.
	HTTPRequests     *prometheus.CounterVec
	RedirectDuration prometheus.Histogram
}

func New() *Metrics {
	m := &Metrics{
		Registry:    prometheus.NewRegistry(),
		Enqueued:    prometheus.NewCounter(prometheus.CounterOpts{Name: "hublinks_events_enqueued_total", Help: "Eventos aceitos na fila."}),
		Dropped:     prometheus.NewCounter(prometheus.CounterOpts{Name: "hublinks_events_dropped_total", Help: "Eventos descartados por fila cheia."}),
		Written:     prometheus.NewCounter(prometheus.CounterOpts{Name: "hublinks_events_written_total", Help: "Eventos gravados no banco."}),
		WriteErrors: prometheus.NewCounter(prometheus.CounterOpts{Name: "hublinks_events_write_errors_total", Help: "Lotes que falharam após todas as tentativas."}),
		QueueLength: prometheus.NewGauge(prometheus.GaugeOpts{Name: "hublinks_event_queue_length", Help: "Eventos aguardando gravação."}),

		MaintenanceDuration:    prometheus.NewGauge(prometheus.GaugeOpts{Name: "hublinks_maintenance_duration_seconds", Help: "Duração da última manutenção diária."}),
		MaintenanceFailures:    prometheus.NewCounter(prometheus.CounterOpts{Name: "hublinks_maintenance_failures_total", Help: "Manutenções diárias que falharam."}),
		MaintenanceLastSuccess: prometheus.NewGauge(prometheus.GaugeOpts{Name: "hublinks_maintenance_last_success_timestamp_seconds", Help: "Instante da última manutenção bem-sucedida."}),

		HTTPRequests: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "hublinks_http_requests_total", Help: "Requisições HTTP por rota (padrão do mux) e status."}, []string{"route", "status"}),
		RedirectDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "hublinks_redirect_duration_seconds", Help: "Tempo do redirecionamento público (resposta 302).",
			Buckets: []float64{.0005, .001, .0025, .005, .01, .025, .05, .1, .25, .5, 1},
		}),
	}
	m.Registry.MustRegister(m.Enqueued, m.Dropped, m.Written, m.WriteErrors, m.QueueLength, m.MaintenanceDuration, m.MaintenanceFailures, m.MaintenanceLastSuccess, m.HTTPRequests, m.RedirectDuration)
	return m
}

// Handler expõe as métricas no formato Prometheus.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{})
}
