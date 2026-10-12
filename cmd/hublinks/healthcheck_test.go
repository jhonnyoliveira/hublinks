package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthURLUsesLoopbackForWildcardAddresses(t *testing.T) {
	for addr, want := range map[string]string{
		":8080":          "http://127.0.0.1:8080/healthz",
		"0.0.0.0:9000":   "http://127.0.0.1:9000/healthz",
		"[::]:8081":      "http://127.0.0.1:8081/healthz",
		"10.1.2.3:8080":  "http://10.1.2.3:8080/healthz",
		"localhost:8080": "http://localhost:8080/healthz",
		"endereço-ruim":  "http://127.0.0.1:8080/healthz",
	} {
		if got := healthURL(addr); got != want {
			t.Errorf("%q: %q, esperado %q", addr, got, want)
		}
	}
}

func TestCheckHealth(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer ok.Close()
	if err := checkHealth(context.Background(), ok.URL); err != nil {
		t.Fatal(err)
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer bad.Close()
	if err := checkHealth(context.Background(), bad.URL); err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("503 deveria falhar: %v", err)
	}
	down := httptest.NewServer(http.NotFoundHandler())
	url := down.URL
	down.Close()
	if err := checkHealth(context.Background(), url); err == nil {
		t.Fatal("serviço fora do ar deveria falhar")
	}
}
