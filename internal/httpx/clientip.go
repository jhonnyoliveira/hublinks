package httpx

import (
	"crypto/hmac"
	"crypto/sha256"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

func ClientIP(r *http.Request, trusted []netip.Prefix) netip.Addr {
	remote, _ := netip.ParseAddrPort(r.RemoteAddr)
	ip := remote.Addr()
	if !contains(trusted, ip) {
		return ip
	}
	chain := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(chain) - 1; i >= 0; i-- {
		candidate, err := netip.ParseAddr(strings.TrimSpace(chain[i]))
		if err != nil {
			continue
		}
		if !contains(trusted, candidate) {
			return candidate
		}
		ip = candidate
	}
	return ip
}
func contains(prefixes []netip.Prefix, ip netip.Addr) bool {
	for _, p := range prefixes {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}
func Normalize(ip netip.Addr) netip.Addr {
	ip = ip.Unmap()
	if ip.Is6() {
		return netip.PrefixFrom(ip, 64).Masked().Addr()
	}
	return ip
}
func Key(pepper string, ip netip.Addr) []byte {
	h := hmac.New(sha256.New, []byte(pepper))
	_, _ = h.Write([]byte("rl|" + Normalize(ip).String()))
	return h.Sum(nil)
}
func RemoteAddr(ip string) string { return net.JoinHostPort(ip, "0") }
