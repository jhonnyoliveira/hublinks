package web

import (
	"bytes"
	"strings"
	"testing"
)

func page(t *testing.T, layout, view string, data any) string {
	t.Helper()
	var b bytes.Buffer
	if err := Render(&b, layout, view, data); err != nil {
		t.Fatalf("%s/%s: %v", layout, view, err)
	}
	return b.String()
}

func TestAdminLayoutForSignedInUser(t *testing.T) {
	out := page(t, "admin", "admin/dashboard", struct{ CSRF string }{"tok123"})
	has(t, out,
		`<html lang="pt-BR">`, `hx-headers=`, `X-CSRF-Token`, `tok123`, `class="skip-link" href="#conteudo"`, `id="conteudo"`,
		// menu lateral e recolhível no celular
		`id="sidebar"`, `aria-controls="sidebar"`, `:aria-expanded="menu"`, `aria-label="Abrir menu"`, `class="mobilebar"`,
		`<nav aria-label="Principal">`, "/admin/marketplaces", "/admin/channels", "/admin/links", "/admin/lixeira",
		// tema
		"data-theme-toggle", `aria-pressed="false"`, `localStorage.getItem("theme")`,
		// base: htmx, alpine, modal e avisos
		"/static/vendor/htmx.min.js", "/static/vendor/alpine.min.js", "/static/js/panel.js", `id="modal"`, `aria-live="polite"`,
		`"code":"422","swap":true`, `"code":"409","swap":true`,
		// logout protegido por CSRF
		`action="/admin/logout"`, `name="csrf_token" value="tok123"`)
	// o tema é resolvido antes da folha de estilo (sem flash)
	if strings.Index(out, `localStorage.getItem("theme")`) > strings.Index(out, `rel="stylesheet"`) {
		t.Error("theme_init deve vir antes da folha de estilo")
	}
	hasNot(t, out, "https://", "http://")
}

func TestAdminLayoutWithoutSessionHasNoNavigation(t *testing.T) {
	out := page(t, "admin", "admin/login", struct{ Next, Error, CSRF string }{"/admin", "E-mail ou senha inválidos", ""})
	has(t, out, `class="auth"`, "E-mail ou senha inválidos", `role="alert"`, `name="next" value="/admin"`, `id="email"`, `autocomplete="email"`, `id="password"`, `type="password"`, `autocomplete="current-password"`, "Entrar")
	hasNot(t, out, `id="sidebar"`, "hx-headers", "X-CSRF-Token", "/admin/lixeira")
}

func TestPublicPagesAlwaysCarryThePrivacyNotice(t *testing.T) {
	img := "https://example.com/i.png"
	cases := map[string]any{
		"public/404": nil,
		"public/preview": struct {
			Title, Marketplace, URL, DestinationURL string
			ImageURL                                *string
		}{"T", "M", "https://hub.example/x", "https://example.com", &img},
		"public/privacy": struct {
			EventsRetentionDays, BotEventsRetentionDays, TrashRetentionDays int
			PrivacyContact                                                  string
		}{395, 30, 30, "p@example.com"},
	}
	for view, data := range cases {
		out := page(t, "public", view, data)
		has(t, out, `id="privacy-notice"`, `href="/privacidade"`, `lang="pt-BR"`, "theme")
		hasNot(t, out, "htmx", "alpine", "/static/vendor/") // páginas públicas sem scripts de terceiros nem bibliotecas
	}
}

func TestComponentGuideRenders(t *testing.T) {
	data := map[string]any{
		"CSRF":  "tok",
		"Table": d("Caption", "C", "Columns", []string{"A"}, "Rows", [][]string{{"1"}}, "EmptyText", "-"),
		"Tabs": d("Label", "Abas", "Items", []struct {
			Href, Text string
			Current    bool
		}{{"#a", "A", true}}),
		"Periods": d("Items", []struct {
			Href, Text string
			Current    bool
		}{{"?p=7d", "7 dias", true}}),
	}
	out := page(t, "admin", "admin/ui", data)
	has(t, out, "Guia de componentes", "Botões e selos", "Campo", "Estados", "Tabela e cartão", "Abas e período")
}
