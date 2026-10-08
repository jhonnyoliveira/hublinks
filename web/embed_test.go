package web

import (
	"bytes"
	"strings"
	"testing"
)

func TestPublicPreviewUsesTemplateEscaping(t *testing.T) {
	var got bytes.Buffer
	err := Render(&got, "public", "public/preview", struct {
		Title, Marketplace, URL, DestinationURL string
		ImageURL                                *string
	}{Title: `<script>alert(1)</script>`, Marketplace: "Loja", URL: "https://hub.example/a", DestinationURL: "https://example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got.String(), "<script>alert") {
		t.Fatalf("título sem escape: %s", got.String())
	}
	if !strings.Contains(got.String(), "/privacidade") {
		t.Fatal("aviso de privacidade ausente")
	}
}
