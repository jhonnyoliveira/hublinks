package web

import (
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// tokens extrai as variáveis --nome: #hex de um bloco CSS que começa em selector.
func tokens(t *testing.T, css, selector string) map[string]string {
	t.Helper()
	i := strings.Index(css, selector+" {")
	if i < 0 {
		t.Fatalf("bloco %q não encontrado", selector)
	}
	rest := css[i+len(selector)+2:]
	end := strings.Index(rest, "}")
	out := map[string]string{}
	for _, m := range regexp.MustCompile(`--([a-z-]+):\s*(#[0-9a-fA-F]{6})`).FindAllStringSubmatch(rest[:end], -1) {
		out[m[1]] = strings.ToLower(m[2])
	}
	return out
}

func channel(v float64) float64 {
	v /= 255
	if v <= 0.03928 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

func luminance(hex string) float64 {
	n, _ := strconv.ParseUint(strings.TrimPrefix(hex, "#"), 16, 32)
	return 0.2126*channel(float64(n>>16&0xff)) + 0.7152*channel(float64(n>>8&0xff)) + 0.0722*channel(float64(n&0xff))
}

func contrast(a, b string) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func TestDesignTokensMeetWCAGAA(t *testing.T) {
	raw, err := Files.ReadFile("static/css/base.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(raw)
	themes := map[string]map[string]string{"claro": tokens(t, css, ":root"), "escuro": tokens(t, css, ".dark")}
	// texto: 4,5:1; contorno de foco (componente não textual): 3:1
	text := [][2]string{{"ink", "bg"}, {"ink", "surface"}, {"muted", "surface"}, {"muted", "bg"}, {"brand", "surface"}, {"brand", "bg"},
		{"brand-ink", "brand"}, {"danger", "surface"}, {"ok", "surface"}, {"ink", "warn-bg"}, {"danger", "warn-bg"}}
	for name, tk := range themes {
		for _, p := range text {
			fg, bg := tk[p[0]], tk[p[1]]
			if fg == "" || bg == "" {
				t.Fatalf("%s: token ausente em %v", name, p)
			}
			if c := contrast(fg, bg); c < 4.5 {
				t.Errorf("%s: %s sobre %s tem %.2f:1 (mínimo 4,5:1)", name, p[0], p[1], c)
			}
		}
		for _, bg := range []string{"bg", "surface"} {
			if c := contrast(tk["focus"], tk[bg]); c < 3 {
				t.Errorf("%s: o anel de foco sobre %s tem %.2f:1 (mínimo 3:1)", name, bg, c)
			}
		}
	}
}

// O modo escuro existe em dois lugares (classe .dark e preferência do sistema sem
// JavaScript); eles não podem divergir.
func TestDarkFallbackMatchesDarkClass(t *testing.T) {
	raw, _ := Files.ReadFile("static/css/base.css")
	css := string(raw)
	class := tokens(t, css, ".dark")
	media := tokens(t, css, ":root:not(.light):not(.dark)")
	if len(class) == 0 || len(class) != len(media) {
		t.Fatalf("tokens: .dark=%d media=%d", len(class), len(media))
	}
	for k, v := range class {
		if media[k] != v {
			t.Errorf("--%s: .dark=%s, fallback=%s", k, v, media[k])
		}
	}
}

func TestDesignDeclaresRequiredTokenGroups(t *testing.T) {
	raw, _ := Files.ReadFile("static/css/base.css")
	css := string(raw)
	for _, want := range []string{"--space-1", "--radius:", "--shadow:", "--font-body", `font-family: "Inter"`, "font-display: swap", "/static/fonts/inter-latin-wght-normal.woff2", "prefers-reduced-motion"} {
		if !strings.Contains(css, want) {
			t.Errorf("faltou %q no CSS base", want)
		}
	}
}

// web/assets/app.css (entrada do Tailwind) não é embutido; confere o arquivo no disco.
func TestTailwindEntryExposesTokensAndDarkVariant(t *testing.T) {
	b, err := os.ReadFile("assets/app.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`@import "tailwindcss"`, "@theme inline", "@custom-variant dark", ".dark", "static/css/base.css", "--color-brand", "--color-surface", "--font-sans"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("web/assets/app.css sem %q", want)
		}
	}
}
