package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type pingFunc func(context.Context) error

func (f pingFunc) Ping(c context.Context) error { return f(c) }
func TestHealth(t *testing.T) {
	for _, tt := range []struct {
		err    error
		status int
	}{{nil, 200}, {errors.New("no"), 503}} {
		r := httptest.NewRecorder()
		Health(pingFunc(func(context.Context) error { return tt.err })).ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		if r.Code != tt.status {
			t.Fatal(r.Code)
		}
	}
}
