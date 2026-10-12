package httpx

import (
	"net/http"
	"strconv"
	"time"

	"github.com/hublinks/hublinks/internal/metrics"
)

// publicRoute é o padrão do mux das rotas públicas de redirecionamento ("/{code}"
// e "/{segment}/{code}" chegam todas aqui).
const publicRoute = "GET /"

// Instrument conta as requisições por rota e status e mede a latência dos
// redirecionamentos públicos. A rota é o padrão do mux (por exemplo
// "GET /admin/links/{id}"), nunca o caminho real: códigos curtos e ids não podem
// virar rótulos. Deve ficar por fora do Recover, para contar também os 500.
func Instrument(next http.Handler, m *metrics.Metrics) http.Handler {
	if m == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(sw, r)
		// O mux preenche r.Pattern no mesmo *http.Request que recebeu.
		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		m.HTTPRequests.WithLabelValues(route, strconv.Itoa(sw.status)).Inc()
		if route == publicRoute && sw.status == http.StatusFound {
			m.RedirectDuration.Observe(time.Since(start).Seconds())
		}
	})
}
