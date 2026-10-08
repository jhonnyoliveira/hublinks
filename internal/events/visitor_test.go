package events

import (
	"net/netip"
	"testing"
)

func TestVisitorID(t *testing.T) {
	pepper := "12345678901234567890123456789012"
	a := VisitorID(pepper, netip.MustParseAddr("2001:db8::1"), "ua")
	b := VisitorID(pepper, netip.MustParseAddr("2001:db8::2"), "ua")
	if len(a) != 32 || string(a) != string(b) {
		t.Fatal("IPv6 /64 deveria coincidir")
	}
	if string(a) == string(VisitorID(pepper, netip.MustParseAddr("2001:db8::2"), "outro")) {
		t.Fatal("UA deveria alterar o hash")
	}
}
