package httpx

import (
	"net/http"
	"net/netip"
	"testing"
)

func TestClientIP(t *testing.T) {
	r := &http.Request{RemoteAddr: "10.0.0.1:8", Header: http.Header{"X-Forwarded-For": []string{"198.51.100.1, 10.0.0.2"}}}
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	if got := ClientIP(r, trusted).String(); got != "198.51.100.1" {
		t.Fatal(got)
	}
	if got := Normalize(netip.MustParseAddr("2001:db8:1:2::4")).String(); got != "2001:db8:1:2::" {
		t.Fatal(got)
	}
}
