package httpx

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLogDoesNotExposeClientAddress(t *testing.T) {
	var out bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&out, nil))
	h := Log(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }), logger)
	r := httptest.NewRequest(http.MethodGet, "/x?token=segredo", nil)
	r.RemoteAddr = "203.0.113.12:5555"
	r.Header.Set("X-Forwarded-For", "198.51.100.20")
	h.ServeHTTP(httptest.NewRecorder(), r)
	if bytes.Contains(out.Bytes(), []byte("203.0.113.12")) || bytes.Contains(out.Bytes(), []byte("198.51.100.20")) || bytes.Contains(out.Bytes(), []byte("segredo")) {
		t.Fatalf("logger expôs dado sensível: %s", out.String())
	}
}

func TestLogRecordsMethodRouteStatusAndDuration(t *testing.T) {
	var out bytes.Buffer
	h := Log(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/falha" {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte("ok")) // sem WriteHeader explícito: 200
	}), slog.New(slog.NewJSONHandler(&out, nil)))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/falha?a=b", nil))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ok", nil))
	lines := bytes.Split(bytes.TrimSpace(out.Bytes()), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("esperadas 2 linhas: %s", out.String())
	}
	want := []struct {
		method, route string
		status        float64
	}{{"POST", "/falha", 502}, {"GET", "/ok", 200}}
	for i, line := range lines {
		var rec map[string]any
		if err := json.Unmarshal(line, &rec); err != nil {
			t.Fatal(err)
		}
		if rec["method"] != want[i].method || rec["route"] != want[i].route || rec["status"] != want[i].status {
			t.Fatalf("linha %d: %v", i, rec)
		}
		if d, ok := rec["duration"].(string); !ok || d == "" {
			t.Fatalf("duração ausente: %v", rec)
		}
		if _, leaked := rec["query"]; leaked || bytes.Contains(line, []byte("a=b")) {
			t.Fatalf("query string vazou: %s", line)
		}
	}
}
