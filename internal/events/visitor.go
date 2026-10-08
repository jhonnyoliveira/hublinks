package events

import (
	"crypto/hmac"
	"crypto/sha256"
	"github.com/hublinks/hublinks/internal/httpx"
	"net/netip"
)

func VisitorID(pepper string, ip netip.Addr, ua string) []byte {
	h := hmac.New(sha256.New, []byte(pepper))
	_, _ = h.Write([]byte(httpx.Normalize(ip).String() + "|" + ua))
	return h.Sum(nil)
}
