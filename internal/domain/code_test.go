package domain

import "testing"

func TestNewCode(t *testing.T) {
	seen := map[byte]bool{}
	for range 5000 {
		code, err := NewCode()
		if err != nil {
			t.Fatal(err)
		}
		if !CodeRE.MatchString(code) || code == "healthz" {
			t.Fatalf("código inválido: %q", code)
		}
		for _, b := range []byte(code) {
			seen[b] = true
		}
	}
	if len(seen) != 36 {
		t.Fatalf("distribuição insuficiente: %d", len(seen))
	}
}
