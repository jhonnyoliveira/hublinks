package server

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestServeFinishesInFlightRequestsBeforeReturning(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		_, _ = io.WriteString(w, "terminou")
	})}
	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- serve(ctx, ln, s, 5*time.Second) }()

	type result struct {
		body string
		err  error
	}
	got := make(chan result, 1)
	go func() {
		resp, err := http.Get("http://" + ln.Addr().String())
		if err != nil {
			got <- result{err: err}
			return
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		got <- result{body: string(b)}
	}()
	<-started
	cancel() // SIGTERM com uma requisição ainda em andamento
	select {
	case err := <-served:
		t.Fatalf("serve retornou (%v) antes de a requisição terminar", err)
	case <-time.After(300 * time.Millisecond):
	}
	close(release)
	if r := <-got; r.err != nil || r.body != "terminou" {
		t.Fatalf("a requisição em andamento deveria terminar: %+v", r)
	}
	if err := <-served; err != nil {
		t.Fatalf("serve: %v", err)
	}
}

func TestServeGivesUpAfterGracePeriod(t *testing.T) {
	started := make(chan struct{})
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	block := make(chan struct{})
	defer close(block)
	s := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-block })}
	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- serve(ctx, ln, s, 200*time.Millisecond) }()
	go func() { _, _ = http.Get("http://" + ln.Addr().String()) }()
	<-started
	cancel()
	select {
	case err := <-served:
		if err == nil {
			t.Fatal("esperava erro de Shutdown por prazo vencido")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("serve não respeitou o prazo de encerramento")
	}
}

func TestNewLoggerHonoursLogLevel(t *testing.T) {
	for level, wantDebug := range map[string]bool{"debug": true, "info": false, "warn": false, "error": false, "invalido": false} {
		l := newLogger(level)
		if got := l.Enabled(context.Background(), slog.LevelDebug); got != wantDebug {
			t.Errorf("LOG_LEVEL=%s: debug habilitado=%v", level, got)
		}
	}
	if newLogger("error").Enabled(context.Background(), slog.LevelWarn) {
		t.Error("com LOG_LEVEL=error, warn deve ficar desligado")
	}
}
