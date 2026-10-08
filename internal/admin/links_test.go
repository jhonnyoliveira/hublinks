package admin_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/domain"
)

func (e env) fullLink(t *testing.T, title string, mp uuid.UUID, policy *domain.Policy, active bool) uuid.UUID {
	t.Helper()
	l, err := e.c.Service.CreateLink(context.Background(), domain.AffiliateLink{OrgID: e.session.OrgID, MarketplaceID: mp, Title: title, DestinationURL: "https://example.com/produto", ShortenPolicyOverride: policy, Active: active})
	if err != nil {
		t.Fatal(err)
	}
	return l.ID
}

func TestLinksIndexNeedsMarketplaceFirst(t *testing.T) {
	e := newEnv(t)
	w := e.do(t, e.c.Links, req{target: "/admin/links"})
	mustContain(t, w, "Crie um", "marketplace", "Nenhum link cadastrado")
	mustNotContain(t, w, `href="/admin/links/new"`)
	if w = e.do(t, e.c.LinkNew, req{target: "/admin/links/new"}); w.Code != http.StatusSeeOther {
		t.Fatalf("sem marketplace, o formulário redireciona: %d", w.Code)
	}
}

func TestLinksIndexSearchFiltersAndPagination(t *testing.T) {
	e := newEnv(t)
	a, b := e.newMarketplace(t, "Alfa"), e.newMarketplace(t, "Beta")
	for i := 0; i < 22; i++ {
		e.fullLink(t, fmt.Sprintf("Camisa %02d", i), a, nil, true)
	}
	calca := e.fullLink(t, "Calça <b>jeans</b>", b, nil, false)

	w := e.do(t, e.c.Links, req{target: "/admin/links"})
	mustContain(t, w, "<html", "Novo link", "Página 1 de 2", "23 itens", `id="links-list"`)
	mustContain(t, w, `<option value="`+a.String()+`">Alfa</option>`)

	// fragmento da lista por HTMX
	w = e.do(t, e.c.Links, req{target: "/admin/links?q=jeans", hx: true, hxTarget: "links-list"})
	mustContain(t, w, `id="link-`+calca.String()+`"`, "Calça &lt;b&gt;jeans&lt;/b&gt;", "Inativo")
	mustNotContain(t, w, "<html", "Camisa", "<b>jeans")

	w = e.do(t, e.c.Links, req{target: "/admin/links?marketplace_id=" + b.String()})
	mustContain(t, w, "Calça", `value="`+b.String()+`" selected`)
	mustNotContain(t, w, "Camisa 00")

	w = e.do(t, e.c.Links, req{target: "/admin/links?active=false"})
	mustContain(t, w, "Calça", `value="false" selected`)
	mustNotContain(t, w, "Camisa 00")

	w = e.do(t, e.c.Links, req{target: "/admin/links?q=nada"})
	mustContain(t, w, "Nenhum link encontrado com estes filtros.")

	w = e.do(t, e.c.Links, req{target: "/admin/links?page=2"})
	mustContain(t, w, "Página 2 de 2", "Anterior")
}

func TestLinksRowShowsPolicyAndSwitchState(t *testing.T) {
	e := newEnv(t)
	mp := e.newMarketplace(t, "Loja")
	direct := domain.PolicyDirect
	id := e.fullLink(t, "Direto", mp, &direct, true)
	w := e.do(t, e.c.Links, req{target: "/admin/links"})
	mustContain(t, w, "Direto · não rastreável", `role="switch"`, `aria-checked="true"`, `hx-post="/admin/links/`+id.String()+`/toggle"`, `hx-vals='{"active":"false"}'`)
}

func TestLinkToggleIsExplicitAndReturnsRow(t *testing.T) {
	e := newEnv(t)
	mp := e.newMarketplace(t, "Loja")
	id := e.fullLink(t, "Produto", mp, nil, true)

	w := e.do(t, e.c.LinkToggle, req{method: http.MethodPost, id: id.String(), target: "/x", form: form("active", "false"), hx: true})
	if w.Code != http.StatusOK {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	mustContain(t, w, `<tr id="link-`+id.String()+`">`, `aria-checked="false"`, "Inativo")
	mustNotContain(t, w, "<html")
	if !strings.Contains(w.Header().Get("HX-Trigger"), "responde") {
		t.Fatalf("toast de desativação: %s", w.Header().Get("HX-Trigger"))
	}
	// repetir a mesma ação é idempotente (não alterna de volta)
	w = e.do(t, e.c.LinkToggle, req{method: http.MethodPost, id: id.String(), target: "/x", form: form("active", "false"), hx: true})
	mustContain(t, w, `aria-checked="false"`)

	w = e.do(t, e.c.LinkToggle, req{method: http.MethodPost, id: id.String(), target: "/x", form: form("active", "true"), hx: true})
	mustContain(t, w, `aria-checked="true"`)
	mustNotContain(t, w, "Inativo")
	if !strings.Contains(w.Header().Get("HX-Trigger"), "Link ativado.") {
		t.Fatal(w.Header().Get("HX-Trigger"))
	}
}

func TestInactiveLinkAnswers404ThroughResolver(t *testing.T) {
	e := newEnv(t)
	mp := e.newMarketplace(t, "Loja")
	id := e.fullLink(t, "Produto", mp, nil, true)
	link, _ := e.c.Service.Store.GetLink(context.Background(), e.session.OrgID, id, false)
	if _, err := e.c.Service.Store.Resolve(context.Background(), link.Code); err != nil {
		t.Fatalf("link ativo deve resolver: %v", err)
	}
	e.do(t, e.c.LinkToggle, req{method: http.MethodPost, id: id.String(), target: "/x", form: form("active", "false"), hx: true})
	if _, err := e.c.Service.Store.Resolve(context.Background(), link.Code); err == nil {
		t.Fatal("link inativo não pode resolver (FR-011)")
	}
}

func TestLinkNewFormHasSwitchAndPolicyOptions(t *testing.T) {
	e := newEnv(t)
	mp := e.newMarketplace(t, "Loja")
	w := e.do(t, e.c.LinkNew, req{target: "/admin/links/new?marketplace_id=" + mp.String()})
	mustContain(t, w, "Novo link", `type="checkbox" role="switch" name="active" value="true" checked`, "Usar a política do marketplace", "Direto (não rastreável)", `value="`+mp.String()+`" selected`, `name="csrf_token" value="csrf-token"`, `action="/admin/links"`)
}

func TestLinkCreateRedirectsToDetailAndShowsToast(t *testing.T) {
	e := newEnv(t)
	mp := e.newMarketplace(t, "Loja")
	e.newChannel(t, "WhatsApp", "wapp")
	w := e.do(t, e.c.LinkCreate, req{method: http.MethodPost, target: "/admin/links", form: form("title", "Produto", "destination_url", "https://example.com/p", "marketplace_id", mp.String(), "active", "true")})
	loc := w.Header().Get("Location")
	if w.Code != http.StatusSeeOther || !strings.HasPrefix(loc, "/admin/links/") || !strings.HasSuffix(loc, "?ok=created") {
		t.Fatalf("%d %q", w.Code, loc)
	}
	id := strings.TrimSuffix(strings.TrimPrefix(loc, "/admin/links/"), "?ok=created")
	w = e.do(t, e.c.LinkShow, req{id: id, target: loc})
	mustContain(t, w, "Produto", "Link criado.", "URLs para divulgar", "https://hub.example/wapp/", "WhatsApp", "Curta", `data-copy="https://hub.example/`, `x-data="copy"`, "Copiar", "Código curto")
	mustNotContain(t, w, "Não rastreável")
}

func TestLinkCreateInlineErrorsKeepInput(t *testing.T) {
	e := newEnv(t)
	mp := e.newMarketplace(t, "Loja")
	w := e.do(t, e.c.LinkCreate, req{method: http.MethodPost, target: "/admin/links", form: form("title", "", "destination_url", "ftp://example.com", "image_url", "javascript:alert(1)", "marketplace_id", mp.String(), "shorten_policy_override", "direct")})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("%d", w.Code)
	}
	mustContain(t, w, `id="title-error"`, "título deve ter de 1 a 200 caracteres", `id="destination_url-error"`, "URL deve usar http ou https", `id="image_url-error"`,
		`value="ftp://example.com"`, `<option value="direct" selected>`, `value="`+mp.String()+`" selected`, `aria-invalid="true"`)

	w = e.do(t, e.c.LinkCreate, req{method: http.MethodPost, target: "/admin/links", form: form("title", "T", "destination_url", "https://example.com", "marketplace_id", "")})
	mustContain(t, w, "Selecione um marketplace.")
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatal(w.Code)
	}
	other := newEnv(t).newMarketplace(t, "De outra org")
	w = e.do(t, e.c.LinkCreate, req{method: http.MethodPost, target: "/admin/links", form: form("title", "T", "destination_url", "https://example.com", "marketplace_id", other.String())})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("marketplace de outra org: %d", w.Code)
	}
	mustContain(t, w, "Selecione um marketplace")
	mustNotContain(t, w, "De outra org")
}

func TestLinkEditUpdateAndClearPolicy(t *testing.T) {
	e := newEnv(t)
	mp := e.newMarketplace(t, "Loja")
	direct := domain.PolicyDirect
	id := e.fullLink(t, "Antigo", mp, &direct, true)

	w := e.do(t, e.c.LinkEdit, req{id: id.String(), target: "/x"})
	mustContain(t, w, "Editar link", `value="Antigo"`, `<option value="direct" selected>`, `action="/admin/links/`+id.String()+`"`)

	w = e.do(t, e.c.LinkUpdate, req{method: http.MethodPost, id: id.String(), target: "/x", form: form("title", "Novo", "destination_url", "https://example.com/novo", "marketplace_id", mp.String(), "shorten_policy_override", "")})
	if w.Code != http.StatusSeeOther || !strings.HasSuffix(w.Header().Get("Location"), "?ok=updated") {
		t.Fatalf("%d %q", w.Code, w.Header().Get("Location"))
	}
	got, _ := e.c.Service.Store.GetLink(context.Background(), e.session.OrgID, id, false)
	// active não veio marcado: desativa; política própria removida
	if got.Title != "Novo" || got.DestinationURL != "https://example.com/novo" || got.ShortenPolicyOverride != nil || got.Active {
		t.Fatalf("estado após edição: %+v", got)
	}
	w = e.do(t, e.c.LinkShow, req{id: id.String(), target: "/x?ok=updated"})
	mustContain(t, w, "Link atualizado.", "Este link está inativo", "Encurtado")

	w = e.do(t, e.c.LinkUpdate, req{method: http.MethodPost, id: id.String(), target: "/x", form: form("title", "", "destination_url", "x", "marketplace_id", mp.String())})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatal(w.Code)
	}
	mustContain(t, w, "Editar link", "título deve ter de 1 a 200 caracteres")
}

func TestLinkShowDirectPolicyHidesTrackingURLs(t *testing.T) {
	e := newEnv(t)
	mp := e.newMarketplace(t, "Loja")
	e.newChannel(t, "WhatsApp", "wapp")
	direct := domain.PolicyDirect
	id := e.fullLink(t, "Direto", mp, &direct, true)
	w := e.do(t, e.c.LinkShow, req{id: id.String(), target: "/x"})
	mustContain(t, w, "Não rastreável", "Original", `data-copy="https://example.com/produto"`, "definida neste link")
	mustNotContain(t, w, "hub.example", "Código curto", "WhatsApp")
}

func TestLinkShowEscapesAndIsScopedToOrg(t *testing.T) {
	e, other := newEnv(t), newEnv(t)
	mp := e.newMarketplace(t, "Loja")
	id := e.fullLink(t, `<script>alert(1)</script>`, mp, nil, true)
	w := e.do(t, e.c.LinkShow, req{id: id.String(), target: "/x"})
	mustContain(t, w, "&lt;script&gt;alert(1)&lt;/script&gt;")
	mustNotContain(t, w, "<script>alert(1)")

	for _, h := range []http.HandlerFunc{other.c.LinkShow, other.c.LinkEdit} {
		if w = other.do(t, h, req{id: id.String(), target: "/x"}); w.Code != http.StatusNotFound {
			t.Fatalf("link de outra org: %d", w.Code)
		}
	}
	if w = e.do(t, e.c.LinkShow, req{id: "nao-uuid", target: "/x"}); w.Code != http.StatusNotFound {
		t.Fatalf("id malformado: %d", w.Code)
	}
}

func TestLinkDeleteFromListRefreshesList(t *testing.T) {
	e := newEnv(t)
	mp := e.newMarketplace(t, "Loja")
	id := e.fullLink(t, "Apagar", mp, nil, true)
	e.fullLink(t, "Ficar", mp, nil, true)
	w := e.do(t, e.c.LinkConfirmDelete, req{id: id.String(), target: "/x", hx: true})
	mustContain(t, w, "Excluir link", "Enviar “Apagar” para a lixeira?", "30 dias")

	w = e.do(t, e.c.LinkDelete, req{method: http.MethodPost, id: id.String(), target: "/x", hx: true, currentURL: "http://localhost/admin/links"})
	if w.Code != http.StatusOK || w.Header().Get("HX-Retarget") != "#links-list" || !strings.Contains(w.Header().Get("HX-Trigger"), "modal-close") {
		t.Fatalf("%d %v", w.Code, w.Header())
	}
	mustContain(t, w, "Ficar")
	mustNotContain(t, w, "Apagar")
}

func TestLinkDeleteFromDetailRedirectsToList(t *testing.T) {
	e := newEnv(t)
	mp := e.newMarketplace(t, "Loja")
	id := e.fullLink(t, "Apagar", mp, nil, true)
	w := e.do(t, e.c.LinkDelete, req{method: http.MethodPost, id: id.String(), target: "/x", hx: true, currentURL: "http://localhost/admin/links/" + id.String()})
	if w.Header().Get("HX-Redirect") != "/admin/links" {
		t.Fatalf("%d %v", w.Code, w.Header())
	}
	// excluir de novo (outra aba): volta para a lista em vez de mostrar erro
	w = e.do(t, e.c.LinkDelete, req{method: http.MethodPost, id: id.String(), target: "/x", hx: true, currentURL: "http://localhost/admin/links/" + id.String()})
	if w.Header().Get("HX-Redirect") != "/admin/links" {
		t.Fatalf("segunda exclusão: %v", w.Header())
	}
	w = e.do(t, e.c.LinkDelete, req{method: http.MethodPost, id: uuid.NewString(), target: "/x", hx: true, currentURL: "http://localhost/admin/links"})
	if !strings.Contains(w.Header().Get("HX-Trigger"), "Link não encontrado") {
		t.Fatalf("na lista, avisa e atualiza: %v", w.Header())
	}
}
