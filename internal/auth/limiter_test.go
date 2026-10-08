package auth

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoginLimiterLocksAfterFailures(t *testing.T) {
	l := NewLoginLimiter("12345678901234567890123456789012", 3, time.Minute, 2*time.Minute)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return now }
	r := httptest.NewRequest("POST", "/admin/login", nil)
	r.RemoteAddr = "198.51.100.8:1234"
	for range 3 {
		l.Fail(r, "admin@example.test")
	}
	if !l.Blocked(r, "admin@example.test") {
		t.Fatal("deveria estar bloqueado")
	}
	now = now.Add(3 * time.Minute)
	if l.Blocked(r, "admin@example.test") {
		t.Fatal("bloqueio deveria expirar")
	}
}
