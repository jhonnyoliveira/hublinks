package httpx

import (
	"log/slog"
	"net/http"
	"time"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(n int) { s.status = n; s.ResponseWriter.WriteHeader(n) }
func Log(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w, status: 200}
		start := time.Now()
		next.ServeHTTP(sw, r)
		logger.Info("http_request", "method", r.Method, "route", r.URL.Path, "status", sw.status, "duration", time.Since(start).String())
	})
}
