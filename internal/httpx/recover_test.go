package httpx

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecoverTurnsPanicInto500AndLogsWithoutRequestData(t *testing.T) {
	var out bytes.Buffer
	h := Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("falha interna") }), slog.New(slog.NewJSONHandler(&out, nil)))
	r := httptest.NewRequest(http.MethodGet, "/x?token=segredo", nil)
	r.RemoteAddr = "203.0.113.9:1"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "erro interno") {
		t.Fatalf("%d %q", w.Code, w.Body.String())
	}
	logged := out.String()
	if !strings.Contains(logged, "panic_recovered") || !strings.Contains(logged, "falha interna") || !strings.Contains(logged, "stack") {
		t.Fatalf("log: %s", logged)
	}
	if strings.Contains(logged, "203.0.113.9") || strings.Contains(logged, "segredo") {
		t.Fatalf("o log não pode ter dados da requisição: %s", logged)
	}
}

func TestRecoverLetsNormalRequestsAndAbortPass(t *testing.T) {
	h := Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) }), slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusTeapot {
		t.Fatal(w.Code)
	}
	abort := Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) }), slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)))
	defer func() {
		if recover() != http.ErrAbortHandler {
			t.Fatal("http.ErrAbortHandler deve seguir adiante")
		}
	}()
	abort.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}
