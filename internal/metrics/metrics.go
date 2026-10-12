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
}

func New() *Metrics {
	m := &Metrics{
		Registry:    prometheus.NewRegistry(),
		Enqueued:    prometheus.NewCounter(prometheus.CounterOpts{Name: "hublinks_events_enqueued_total", Help: "Eventos aceitos na fila."}),
		Dropped:     prometheus.NewCounter(prometheus.CounterOpts{Name: "hublinks_events_dropped_total", Help: "Eventos descartados por fila cheia."}),
		Written:     prometheus.NewCounter(prometheus.CounterOpts{Name: "hublinks_events_written_total", Help: "Eventos gravados no banco."}),
		WriteErrors: prometheus.NewCounter(prometheus.CounterOpts{Name: "hublinks_events_write_errors_total", Help: "Lotes que falharam após todas as tentativas."}),
		QueueLength: prometheus.NewGauge(prometheus.GaugeOpts{Name: "hublinks_event_queue_length", Help: "Eventos aguardando gravação."}),
	}
	m.Registry.MustRegister(m.Enqueued, m.Dropped, m.Written, m.WriteErrors, m.QueueLength)
	return m
}

// Handler expõe as métricas no formato Prometheus.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{})
}
