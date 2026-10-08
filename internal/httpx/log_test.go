package httpx

import (
	"bytes"
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
