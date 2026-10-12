package httpx

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hublinks/hublinks/internal/metrics"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/links/{id}", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("GET /boom", func(w http.ResponseWriter, r *http.Request) { panic("falha") })
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/abc1234", "/wapp/abc1234":
			http.Redirect(w, r, "https://example.com", http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	})
	return mux
}

func serve(h http.Handler, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	return w
}

func TestInstrumentCountsByMuxPatternAndStatus(t *testing.T) {
	m := metrics.New()
	h := Instrument(Recover(newMux(), slog.Default()), m)
	serve(h, http.MethodGet, "/abc1234")
	serve(h, http.MethodGet, "/wapp/abc1234")
	serve(h, http.MethodGet, "/zzzzzzz")
	serve(h, http.MethodGet, "/admin/links/1111")
	serve(h, http.MethodGet, "/admin/links/2222")
	serve(h, http.MethodGet, "/boom")
	serve(h, http.MethodPost, "/abc1234") // sem rota para POST

	count := func(route, status string) float64 {
		return testutil.ToFloat64(m.HTTPRequests.WithLabelValues(route, status))
	}
	for _, c := range []struct {
		route, status string
		want          float64
	}{
		{"GET /", "302", 2}, {"GET /", "404", 1},
		{"GET /admin/links/{id}", "200", 2}, // ids diferentes, uma só série
		{"GET /boom", "500", 1},             // o pânico recuperado também é contado
		{"unmatched", "405", 1},
	} {
		if got := count(c.route, c.status); got != c.want {
			t.Errorf("%s %s = %v, esperado %v", c.route, c.status, got, c.want)
		}
	}
	// nenhum código curto ou id vira rótulo
	if n := testutil.CollectAndCount(m.HTTPRequests); n != 5 {
		t.Errorf("séries = %d (esperado 5): a cardinalidade deve ser limitada às rotas", n)
	}
}

func TestInstrumentTimesOnlyPublicRedirects(t *testing.T) {
	m := metrics.New()
	h := Instrument(newMux(), m)
	serve(h, http.MethodGet, "/abc1234")
	serve(h, http.MethodGet, "/wapp/abc1234")
	serve(h, http.MethodGet, "/zzzzzzz")          // 404 não entra
	serve(h, http.MethodGet, "/admin/links/1111") // outra rota não entra
	if got := testutil.CollectAndCount(m.RedirectDuration); got != 1 {
		t.Fatalf("séries do histograma = %d", got)
	}
	reg := m.Registry
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var observed uint64
	for _, mf := range mfs {
		if mf.GetName() == "hublinks_redirect_duration_seconds" {
			observed = mf.GetMetric()[0].GetHistogram().GetSampleCount()
		}
	}
	if observed != 2 {
		t.Fatalf("redirecionamentos medidos = %d (esperado 2)", observed)
	}
}

func TestInstrumentWithoutMetricsIsTransparent(t *testing.T) {
	if w := serve(Instrument(newMux(), nil), http.MethodGet, "/abc1234"); w.Code != http.StatusFound {
		t.Fatalf("status %d", w.Code)
	}
}
