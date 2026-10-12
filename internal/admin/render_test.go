package admin_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/hublinks/hublinks/internal/admin"
)

// Estados vazios: toda listagem de uma instalação nova orienta o próximo passo.
func TestEmptyListingsGuideTheNextStep(t *testing.T) {
	e := newEnv(t)
	for _, c := range []struct {
		name   string
		h      http.HandlerFunc
		target string
		want   []string
	}{
		{"marketplaces", e.c.Marketplaces, "/admin/marketplaces", []string{`class="card empty"`, "Nenhum marketplace cadastrado. Crie o primeiro"}},
		{"canais", e.c.Channels, "/admin/channels", []string{`class="card empty"`, "Nenhum canal cadastrado. Crie canais como WhatsApp"}},
		{"links sem marketplace", e.c.Links, "/admin/links", []string{`class="card empty"`, "Crie um", "marketplace", "Nenhum link cadastrado"}},
		{"lixeira", e.c.Trash, "/admin/lixeira", []string{`class="card empty"`, "A lixeira está vazia."}},
		{"dashboard", e.dash.Show, "/admin", []string{"Comece por aqui", "Cadastrar marketplace", "Cadastrar canais", "Cadastrar link", `class="card empty"`}},
	} {
		w := e.do(t, c.h, req{target: c.target})
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status %d", c.name, w.Code)
		}
		mustContain(t, w, c.want...)
	}
}

// Depois de cadastrar o marketplace, a lista de links vazia aponta o próximo passo.
func TestEmptyLinksListAfterMarketplace(t *testing.T) {
	e := newEnv(t)
	e.newMarketplace(t, "Loja")
	w := e.do(t, e.c.Links, req{target: "/admin/links"})
	mustContain(t, w, "Nenhum link cadastrado. Cadastre o primeiro", `href="/admin/links/new"`)
}

// pages devolve páginas e fragmentos de formulário do painel para as verificações de acessibilidade.
func (e env) pages(t *testing.T) map[string]*httptest.ResponseRecorder {
	t.Helper()
	mp := e.newMarketplace(t, "Loja")
	link := e.fullLink(t, "Produto", mp, nil, true)
	e.newChannel(t, "WhatsApp", "wapp")
	e.click(t, link, nil, true, true, false)
	return map[string]*httptest.ResponseRecorder{
		"dashboard":      e.do(t, e.dash.Show, req{target: "/admin"}),
		"links":          e.do(t, e.c.Links, req{target: "/admin/links"}),
		"links/new":      e.do(t, e.c.LinkNew, req{target: "/admin/links/new"}),
		"links/show":     e.do(t, e.c.LinkShow, req{target: "/admin/links/" + link.String(), id: link.String()}),
		"links/edit":     e.do(t, e.c.LinkEdit, req{target: "/admin/links/" + link.String() + "/edit", id: link.String()}),
		"marketplaces":   e.do(t, e.c.Marketplaces, req{target: "/admin/marketplaces"}),
		"marketplace/nv": e.do(t, e.c.MarketplaceNew, req{target: "/admin/marketplaces/new", hx: true}),
		"channels":       e.do(t, e.c.Channels, req{target: "/admin/channels"}),
		"channel/nv":     e.do(t, e.c.ChannelNew, req{target: "/admin/channels/new", hx: true}),
		"lixeira":        e.do(t, e.c.Trash, req{target: "/admin/lixeira"}),
		"ui":             e.do(t, admin.UI, req{target: "/admin/ui"}),
	}
}

var (
	controlRe = regexp.MustCompile(`(?is)<(input|select|textarea)\b([^>]*)>`)
	attrRe    = func(name string) *regexp.Regexp { return regexp.MustCompile(`(?i)\b` + name + `="([^"]*)"`) }
	imgRe     = regexp.MustCompile(`(?is)<img\b([^>]*)>`)
)

// Todo campo de formulário tem rótulo associado (label for, label envolvente ou aria-label).
func TestEveryControlHasALabelAndEveryImageHasAlt(t *testing.T) {
	e := newEnv(t)
	controls := 0
	defer func() {
		if controls < 10 {
			t.Errorf("apenas %d campos verificados: o teste deixou de enxergar os formulários", controls)
		}
	}()
	for name, w := range e.pages(t) {
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status %d", name, w.Code)
		}
		body := w.Body.String()
		for _, m := range controlRe.FindAllStringSubmatchIndex(body, -1) {
			tag, attrs := body[m[0]:m[1]], body[m[4]:m[5]]
			if t1 := attrRe("type").FindStringSubmatch(attrs); t1 != nil && t1[1] == "hidden" {
				continue
			}
			controls++
			if attrRe("aria-label").MatchString(attrs) || attrRe("aria-labelledby").MatchString(attrs) {
				continue
			}
			if id := attrRe("id").FindStringSubmatch(attrs); id != nil && strings.Contains(body, `for="`+id[1]+`"`) {
				continue
			}
			// envolvido por <label>…</label>
			before := body[:m[0]]
			if i := strings.LastIndex(strings.ToLower(before), "<label"); i >= 0 && !strings.Contains(strings.ToLower(before[i:]), "</label>") {
				continue
			}
			t.Errorf("%s: campo sem rótulo: %s", name, tag)
		}
		for _, m := range imgRe.FindAllStringSubmatch(body, -1) {
			if !attrRe("alt").MatchString(m[1]) {
				t.Errorf("%s: <img> sem alt: %s", name, m[0])
			}
		}
	}
}

// Páginas completas: idioma, botão de tema nomeado, "pular para o conteúdo" e botões de ícone nomeados.
func TestFullPagesDeclareLanguageThemeAndSkipLink(t *testing.T) {
	e := newEnv(t)
	for name, w := range e.pages(t) {
		body := w.Body.String()
		if !strings.Contains(body, "<html") {
			continue // fragmento HTMX
		}
		for _, want := range []string{`<html lang="pt-BR">`, `data-theme-toggle`, `aria-label="Alternar tema escuro"`, `class="skip-link" href="#conteudo"`, `id="conteudo"`, `<meta name="viewport"`} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: faltou %s", name, want)
			}
		}
		// todo <button> sem texto visível (apenas ícone) precisa de aria-label
		for _, m := range regexp.MustCompile(`(?is)<button\b([^>]*)>(.*?)</button>`).FindAllStringSubmatch(body, -1) {
			text := strings.TrimSpace(regexp.MustCompile(`(?is)<svg.*?</svg>|<[^>]+>`).ReplaceAllString(m[2], ""))
			if text == "" && !attrRe("aria-label").MatchString(m[1]) && !strings.Contains(m[1], "class=\"switch\"") {
				t.Errorf("%s: botão sem nome acessível: <button%s>", name, m[1])
			}
		}
	}
}

// O estado de cada interruptor é anunciado (role=switch com aria-checked) e nomeado.
func TestLinkSwitchIsNamed(t *testing.T) {
	e := newEnv(t)
	mp := e.newMarketplace(t, "Loja")
	e.fullLink(t, "Produto", mp, nil, true)
	w := e.do(t, e.c.Links, req{target: "/admin/links"})
	mustContain(t, w, `role="switch"`, `aria-checked="true"`, `aria-label="Ativo: Produto"`)
}

// Tabelas viram cartões no celular: cada célula (exceto a primeira) tem rótulo.
func TestTablesDeclareCardLabelsForMobile(t *testing.T) {
	e := newEnv(t)
	mp := e.newMarketplace(t, "Loja")
	e.fullLink(t, "Produto", mp, nil, true)
	e.newChannel(t, "WhatsApp", "wapp")
	for _, c := range []struct {
		name string
		h    http.HandlerFunc
		url  string
		want []string
	}{
		{"links", e.c.Links, "/admin/links", []string{`class="table-cards"`, `data-label="Marketplace"`, `data-label="Cliques"`, `data-label="Ativo"`}},
		{"marketplaces", e.c.Marketplaces, "/admin/marketplaces", []string{`class="table-cards"`, `data-label="Política"`}},
		{"canais", e.c.Channels, "/admin/channels", []string{`class="table-cards"`, `data-label="Segmento"`}},
	} {
		mustContain(t, e.do(t, c.h, req{target: c.url}), c.want...)
	}
}

// O formulário de link protege contra perda de alterações.
func TestLinkFormGuardsUnsavedChanges(t *testing.T) {
	e := newEnv(t)
	e.newMarketplace(t, "Loja")
	w := e.do(t, e.c.LinkNew, req{target: "/admin/links/new"})
	mustContain(t, w, `data-unsaved="false"`, `/static/js/unsaved.js`)
}
