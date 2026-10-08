package httpx

import (
	"net/http"
	"net/netip"
	"testing"
)

func request(remote string, xff ...string) *http.Request {
	h := http.Header{}
	for _, v := range xff {
		h.Add("X-Forwarded-For", v)
	}
	return &http.Request{RemoteAddr: remote, Header: h}
}

var trusted = []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("fd00::/8")}

func TestClientIPFromTrustedProxy(t *testing.T) {
	cases := []struct {
		name, remote, xff, want string
	}{
		{"uma entrada", "10.0.0.1:8", "198.51.100.1", "198.51.100.1"},
		{"várias entradas, proxies no fim", "10.0.0.1:8", "198.51.100.1, 10.0.0.2, 10.0.0.3", "198.51.100.1"},
		{"entradas forjadas à esquerda são ignoradas", "10.0.0.1:8", "1.1.1.1, 203.0.113.9, 10.0.0.2", "203.0.113.9"},
		{"sem XFF usa o par da conexão", "10.0.0.1:8", "", "10.0.0.1"},
		{"XFF só com proxies", "10.0.0.1:8", "10.0.0.7, 10.0.0.2", "10.0.0.7"},
		{"entrada inválida é pulada", "10.0.0.1:8", "198.51.100.1, lixo", "198.51.100.1"},
		{"proxy IPv6 confiável", "[fd00::1]:8", "198.51.100.1", "198.51.100.1"},
		{"cliente IPv6", "10.0.0.1:8", "2001:db8::7", "2001:db8::7"},
		{"proxy IPv4 visto como IPv4-mapped (dual-stack)", "[::ffff:10.0.0.1]:8", "198.51.100.1", "198.51.100.1"},
		{"entrada IPv4-mapped no XFF", "10.0.0.1:8", "::ffff:198.51.100.1", "198.51.100.1"},
	}
	for _, tc := range cases {
		if got := ClientIP(request(tc.remote, tc.xff), trusted).String(); got != tc.want {
			t.Errorf("%s: %s, esperado %s", tc.name, got, tc.want)
		}
	}
}

func TestClientIPIgnoresForgedHeaderFromUntrustedPeer(t *testing.T) {
	for _, remote := range []string{"203.0.113.50:9", "[2001:db8::5]:9", "[::ffff:203.0.113.50]:9"} {
		got := ClientIP(request(remote, "1.2.3.4, 10.0.0.1"), trusted)
		if got.Unmap().String() == "1.2.3.4" || got.Unmap().String() == "10.0.0.1" {
			t.Fatalf("%s: o cabeçalho forjado foi aceito: %s", remote, got)
		}
	}
	if got := ClientIP(request("203.0.113.50:9", "1.2.3.4"), nil).String(); got != "203.0.113.50" {
		t.Fatalf("sem proxies confiáveis, XFF não vale: %s", got)
	}
}

func TestClientIPMultipleXFFHeaderLinesAreNotMerged(t *testing.T) {
	// apenas a primeira linha é lida; uma linha extra enviada pelo cliente não
	// pode se sobrepor ao que o proxy escreveu
	got := ClientIP(request("10.0.0.1:8", "198.51.100.1", "9.9.9.9"), trusted).String()
	if got != "198.51.100.1" && got != "9.9.9.9" {
		t.Fatal(got)
	}
}

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"192.0.2.10":           "192.0.2.10",
		"::ffff:192.0.2.10":    "192.0.2.10",
		"2001:db8:1:2:3:4:5:6": "2001:db8:1:2::",
		"2001:db8:1:2::4":      "2001:db8:1:2::",
		"fe80::1%eth0":         "fe80::",
	}
	for in, want := range cases {
		if got := Normalize(netip.MustParseAddr(in)).String(); got != want {
			t.Errorf("Normalize(%s) = %s, esperado %s", in, got, want)
		}
	}
	// dois endereços do mesmo /64 produzem a mesma chave; /64 diferentes, não
	a, b, c := netip.MustParseAddr("2001:db8:1:2::1"), netip.MustParseAddr("2001:db8:1:2:ffff::9"), netip.MustParseAddr("2001:db8:1:3::1")
	if string(Key("p", a)) != string(Key("p", b)) || string(Key("p", a)) == string(Key("p", c)) {
		t.Fatal("a chave deve agrupar por /64")
	}
	if string(Key("p1", a)) == string(Key("p2", a)) {
		t.Fatal("a chave depende do pepper")
	}
	if string(Key("p", netip.MustParseAddr("::ffff:192.0.2.1"))) != string(Key("p", netip.MustParseAddr("192.0.2.1"))) {
		t.Fatal("IPv4-mapped e IPv4 devem gerar a mesma chave")
	}
}
