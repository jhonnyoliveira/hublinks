package domain

import "testing"

func TestValidation(t *testing.T) {
	for _, raw := range []string{"https://example.com/a", "http://example.com"} {
		if !URL(raw) {
			t.Fatalf("URL válida rejeitada: %s", raw)
		}
	}
	for _, raw := range []string{"ftp://example.com", "https:///sem-host", "javascript:alert(1)"} {
		if URL(raw) {
			t.Fatalf("URL inválida aceita: %s", raw)
		}
	}
	if !Segment("whats-app") || Segment("Admin") || Segment("admin") {
		t.Fatal("validação de segmento incorreta")
	}
	if !Text("x", 1, 1) || Text("", 1, 2) || Text("abc", 1, 2) {
		t.Fatal("validação de texto incorreta")
	}
}
