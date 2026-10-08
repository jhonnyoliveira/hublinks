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

func (e env) newMarketplace(t *testing.T, name string) uuid.UUID {
	t.Helper()
	m, err := e.c.Service.CreateMarketplace(context.Background(), e.session.OrgID, name, domain.PolicyShorten)
	if err != nil {
		t.Fatal(err)
	}
	return m.ID
}

func TestMarketplacesPageEscapesNamesAndOffersCreate(t *testing.T) {
	e := newEnv(t)
	e.newMarketplace(t, `<script>alert(1)</script>`)
	w := e.do(t, e.c.Marketplaces, req{target: "/admin/marketplaces"})
	if w.Code != http.StatusOK {
		t.Fatal(w.Code)
	}
	mustContain(t, w, "<html", "Novo marketplace", `hx-get="/admin/marketplaces/new"`, "&lt;script&gt;alert(1)&lt;/script&gt;", `hx-headers=`, `X-CSRF-Token`, "/static/vendor/htmx.min.js")
	mustNotContain(t, w, "<script>alert(1)")
}

func TestMarketplacesEmptyStates(t *testing.T) {
	e := newEnv(t)
	mustContain(t, e.do(t, e.c.Marketplaces, req{target: "/admin/marketplaces"}), "Nenhum marketplace cadastrado")
	e.newMarketplace(t, "Loja")
	w := e.do(t, e.c.Marketplaces, req{target: "/admin/marketplaces?q=zzz"})
	mustContain(t, w, "Nenhum marketplace encontrado para “zzz”")
}

func TestMarketplacesSearchReturnsOnlyTheListFragment(t *testing.T) {
	e := newEnv(t)
	e.newMarketplace(t, "Alfa")
	e.newMarketplace(t, "Beta")
	w := e.do(t, e.c.Marketplaces, req{target: "/admin/marketplaces?q=alf", hx: true, hxTarget: "marketplaces-list"})
	mustContain(t, w, `id="marketplaces-list"`, "Alfa")
	mustNotContain(t, w, "<html", "Beta", "Novo marketplace")
}

func TestMarketplacesPaginationKeepsQuery(t *testing.T) {
	e := newEnv(t)
	for i := 0; i < 25; i++ {
		e.newMarketplace(t, fmt.Sprintf("Loja %02d", i))
	}
	w := e.do(t, e.c.Marketplaces, req{target: "/admin/marketplaces?q=Loja"})
	mustContain(t, w, "Página 1 de 2", "25 itens", "Loja 00", "Loja 19", "page=2", "q=Loja")
	mustNotContain(t, w, "Loja 20", "Anterior")
	w = e.do(t, e.c.Marketplaces, req{target: "/admin/marketplaces?q=Loja&page=2"})
	mustContain(t, w, "Página 2 de 2", "Loja 20", "Loja 24", "Anterior")
	mustNotContain(t, w, "Loja 19", "Próxima")
}

func TestMarketplaceCreateViaHTMXRefreshesListClosesModalAndToasts(t *testing.T) {
	e := newEnv(t)
	w := e.do(t, e.c.MarketplaceCreate, req{method: http.MethodPost, target: "/admin/marketplaces", form: form("name", "Loja Nova", "shorten_policy", "direct"), hx: true})
	if w.Code != http.StatusOK || w.Header().Get("HX-Retarget") != "#marketplaces-list" || w.Header().Get("HX-Reswap") != "outerHTML" {
		t.Fatalf("status=%d headers=%v", w.Code, w.Header())
	}
	trig := w.Header().Get("HX-Trigger")
	if !strings.Contains(trig, `"modal-close":true`) || !strings.Contains(trig, "Marketplace criado.") {
		t.Fatalf("HX-Trigger = %s", trig)
	}
	mustContain(t, w, "Loja Nova", "Direto · não rastreável")
}

func TestMarketplaceCreateWithoutHTMXRedirects(t *testing.T) {
	e := newEnv(t)
	w := e.do(t, e.c.MarketplaceCreate, req{method: http.MethodPost, target: "/admin/marketplaces", form: form("name", "Loja", "shorten_policy", "shorten")})
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/admin/marketplaces" {
		t.Fatalf("%d %q", w.Code, w.Header().Get("Location"))
	}
}

func TestMarketplaceCreateShowsInlineErrorsAndKeepsInput(t *testing.T) {
	e := newEnv(t)
	w := e.do(t, e.c.MarketplaceCreate, req{method: http.MethodPost, target: "/admin/marketplaces", form: form("name", "", "shorten_policy", "outra"), hx: true})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d", w.Code)
	}
	mustContain(t, w, `id="mp-name-error"`, `role="alert"`, `aria-invalid="true"`, `aria-describedby="mp-name-error"`, "nome deve ter de 1 a 80 caracteres", "política inválida")
	mustNotContain(t, w, "<html", `id="marketplaces-list"`)

	e.newMarketplace(t, "Loja")
	w = e.do(t, e.c.MarketplaceCreate, req{method: http.MethodPost, target: "/admin/marketplaces", form: form("name", "loja", "shorten_policy", "shorten"), hx: true})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("duplicado: %d", w.Code)
	}
	mustContain(t, w, "Já existe um marketplace com este nome.", `value="loja"`)
}

func TestMarketplaceEditAndUpdate(t *testing.T) {
	e := newEnv(t)
	id := e.newMarketplace(t, "Antiga")
	w := e.do(t, e.c.MarketplaceEdit, req{id: id.String(), target: "/admin/marketplaces/x/edit", hx: true})
	mustContain(t, w, "Editar marketplace", `value="Antiga"`, `hx-post="/admin/marketplaces/`+id.String()+`"`)

	w = e.do(t, e.c.MarketplaceUpdate, req{method: http.MethodPost, id: id.String(), target: "/x", form: form("name", "Nova", "shorten_policy", "direct"), hx: true})
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	mustContain(t, w, "Nova", "Direto")
	if !strings.Contains(w.Header().Get("HX-Trigger"), "Marketplace atualizado.") {
		t.Fatal(w.Header().Get("HX-Trigger"))
	}

	w = e.do(t, e.c.MarketplaceUpdate, req{method: http.MethodPost, id: id.String(), target: "/x", form: form("name", "", "shorten_policy", "direct"), hx: true})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatal(w.Code)
	}
	mustContain(t, w, "Editar marketplace", "nome deve ter de 1 a 80 caracteres")
}

func TestMarketplaceActionsKeepSearchFromCurrentURL(t *testing.T) {
	e := newEnv(t)
	e.newMarketplace(t, "Alfa")
	e.newMarketplace(t, "Beta")
	w := e.do(t, e.c.MarketplaceCreate, req{method: http.MethodPost, target: "/admin/marketplaces", form: form("name", "Alfa Dois", "shorten_policy", "shorten"), hx: true, currentURL: "http://localhost/admin/marketplaces?q=alfa"})
	mustContain(t, w, "Alfa Dois", "Alfa")
	mustNotContain(t, w, "Beta")
}

func TestMarketplaceDeleteFlow(t *testing.T) {
	e := newEnv(t)
	id := e.newMarketplace(t, "Loja")
	w := e.do(t, e.c.MarketplaceConfirmDelete, req{id: id.String(), target: "/x", hx: true})
	mustContain(t, w, "Excluir marketplace", "Enviar “Loja” para a lixeira?", "30 dias", `hx-post="/admin/marketplaces/`+id.String()+`/delete"`)

	w = e.do(t, e.c.MarketplaceDelete, req{method: http.MethodPost, id: id.String(), target: "/x", hx: true})
	if w.Code != http.StatusOK || !strings.Contains(w.Header().Get("HX-Trigger"), "lixeira") {
		t.Fatalf("%d %s", w.Code, w.Header().Get("HX-Trigger"))
	}
	mustNotContain(t, w, "Loja")
	if _, n, _ := e.c.Service.Store.ListMarketplacesWithOptions(context.Background(), e.session.OrgID, trashOpts()); n != 1 {
		t.Fatalf("itens na lixeira: %d", n)
	}
}

func TestMarketplaceDeleteBlockedListsLinks(t *testing.T) {
	e := newEnv(t)
	id := e.newMarketplace(t, "Loja")
	link, err := e.c.Service.CreateLink(context.Background(), domain.AffiliateLink{OrgID: e.session.OrgID, MarketplaceID: id, Title: "Produto <b>X</b>", DestinationURL: "https://example.com", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	w := e.do(t, e.c.MarketplaceDelete, req{method: http.MethodPost, id: id.String(), target: "/x", hx: true})
	if w.Code != http.StatusConflict {
		t.Fatalf("status %d", w.Code)
	}
	mustContain(t, w, "ainda tem 1 link", "Produto &lt;b&gt;X&lt;/b&gt;", `href="/admin/links/`+link.ID.String()+`"`, "Fechar")
	mustNotContain(t, w, `type="submit"`, "Produto <b>")
	if w.Header().Get("HX-Trigger") != "" {
		t.Fatalf("bloqueio não deve fechar o modal nem avisar sucesso: %s", w.Header().Get("HX-Trigger"))
	}
}

func TestMarketplaceUnknownOrForeignIDRefreshesListWithErrorToast(t *testing.T) {
	e := newEnv(t)
	other := newEnv(t)
	foreign := other.newMarketplace(t, "De outra org")
	for _, id := range []string{uuid.NewString(), "nao-e-uuid", foreign.String()} {
		w := e.do(t, e.c.MarketplaceEdit, req{id: id, target: "/x", hx: true})
		if w.Code != http.StatusOK || !strings.Contains(w.Header().Get("HX-Trigger"), "não encontrado") || w.Header().Get("HX-Retarget") != "#marketplaces-list" {
			t.Fatalf("id %q: %d %v", id, w.Code, w.Header())
		}
		mustNotContain(t, w, "De outra org")
	}
}

func TestMarketplacesHTMXSearchUsesItsOwnQueryNotThePageURL(t *testing.T) {
	e := newEnv(t)
	e.newMarketplace(t, "Alfa")
	e.newMarketplace(t, "Beta")
	// o navegador envia HX-Current-URL da página atual (sem q) junto da busca
	w := e.do(t, e.c.Marketplaces, req{target: "/admin/marketplaces?q=beta", hx: true, hxTarget: "marketplaces-list", currentURL: "http://localhost/admin/marketplaces"})
	mustContain(t, w, "Beta")
	mustNotContain(t, w, "Alfa")
}
