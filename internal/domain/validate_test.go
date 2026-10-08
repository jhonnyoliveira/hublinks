package domain

import (
	"strings"
	"testing"
)

func TestValidation(t *testing.T) {
	for _, raw := range []string{"https://example.com/a", "http://example.com", "https://example.com/" + strings.Repeat("a", 2027)} {
		if !URL(raw) {
			t.Fatalf("URL válida rejeitada: %s", raw)
		}
	}
	for _, raw := range []string{"ftp://example.com", "https:///sem-host", "javascript:alert(1)", "https://example.com/" + strings.Repeat("a", 2029)} {
		if URL(raw) {
			t.Fatalf("URL inválida aceita: %s", raw)
		}
	}
	if !Segment("whats-app") || Segment("Admin") || Segment("admin") || Segment(strings.Repeat("a", 21)) {
		t.Fatal("validação de segmento incorreta")
	}
	for _, word := range []string{"api", "p", "static", "healthz", "beacon", "privacidade"} {
		if Segment(word) {
			t.Fatalf("segmento reservado aceito: %s", word)
		}
	}
	if !Text("x", 1, 1) || Text("", 1, 2) || Text("abc", 1, 2) || !Text(strings.Repeat("x", 200), 1, 200) || Text(strings.Repeat("x", 201), 1, 200) || !Text(strings.Repeat("x", 80), 1, 80) || !Text(strings.Repeat("x", 60), 1, 60) {
		t.Fatal("validação de texto incorreta")
	}
	if !PolicyValid(PolicyShorten) || !PolicyValid(PolicyDirect) || PolicyValid("other") {
		t.Fatal("política inválida")
	}
}
