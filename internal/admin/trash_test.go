package admin_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// ageTrash recua deleted_at de um item para simular o passar dos dias.
func (e env) ageTrash(t *testing.T, table string, id uuid.UUID, days int) {
	t.Helper()
	q := "UPDATE " + table + " SET deleted_at = now() - make_interval(days => $2) WHERE id=$1"
	if _, err := e.c.Service.Store.Pool.Exec(context.Background(), q, id, days); err != nil {
		t.Fatal(err)
	}
}

func TestTrashEmpty(t *testing.T) {
	e := newEnv(t)
	w := e.do(t, e.c.Trash, req{target: "/admin/lixeira"})
	mustContain(t, w, "Lixeira", "A lixeira está vazia.", "30 dias")
}

func TestTrashShowsDaysRemainingFromConfiguredRetention(t *testing.T) {
	e := newEnv(t)
	e.c.TrashRetentionDays = 10
	mp := e.newMarketplace(t, "Loja Velha")
	ch := e.newChannel(t, "Canal Novo", "novo")
	e.do(t, e.c.MarketplaceDelete, req{method: http.MethodPost, id: mp.String(), target: "/x", hx: true})
	e.do(t, e.c.ChannelDelete, req{method: http.MethodPost, id: ch.String(), target: "/x", hx: true})
	e.ageTrash(t, "marketplaces", mp, 4)

	w := e.do(t, e.c.Trash, req{target: "/admin/lixeira"})
	mustContain(t, w, "por 10 dias", "Loja Velha", "6 dias", "Canal Novo", "10 dias")
	body := w.Body.String()
	if strings.Index(body, "Canal Novo") > strings.Index(body, "Loja Velha") {
		t.Fatal("os itens mais recentes devem vir primeiro")
	}

	e.ageTrash(t, "channels", ch, 9)
	w = e.do(t, e.c.Trash, req{target: "/admin/lixeira"})
	mustContain(t, w, "1 dia")
	mustNotContain(t, w, "1 dias")

	e.ageTrash(t, "channels", ch, 10)
	w = e.do(t, e.c.Trash, req{target: "/admin/lixeira"})
	mustContain(t, w, "expira hoje")
}

func TestTrashRestoreMarketplaceViaHTMX(t *testing.T) {
	e := newEnv(t)
	mp := e.newMarketplace(t, "Loja")
	e.do(t, e.c.MarketplaceDelete, req{method: http.MethodPost, id: mp.String(), target: "/x", hx: true})
	w := e.do(t, e.c.Trash, req{target: "/admin/lixeira"})
	mustContain(t, w, `hx-post="/admin/marketplaces/`+mp.String()+`/restore"`, `hx-target="#trash-list"`)

	w = e.do(t, e.c.MarketplaceRestore, req{method: http.MethodPost, id: mp.String(), target: "/x", hx: true})
	if w.Code != http.StatusOK || !strings.Contains(w.Header().Get("HX-Trigger"), "Marketplace restaurado.") {
		t.Fatalf("%d %v", w.Code, w.Header())
	}
	mustContain(t, w, `id="trash-list"`, "A lixeira está vazia.")
	mustNotContain(t, w, "<html")
	// de volta à listagem normal
	mustContain(t, e.do(t, e.c.Marketplaces, req{target: "/admin/marketplaces"}), "Loja")
}

func TestTrashLinkWaitsForItsMarketplace(t *testing.T) {
	e := newEnv(t)
	mp := e.newMarketplace(t, "Loja Pai")
	link := e.fullLink(t, "Filho", mp, nil, true)
	e.do(t, e.c.LinkDelete, req{method: http.MethodPost, id: link.String(), target: "/x", hx: true, currentURL: "http://x/admin/links"})
	e.do(t, e.c.MarketplaceDelete, req{method: http.MethodPost, id: mp.String(), target: "/x", hx: true})

	w := e.do(t, e.c.Trash, req{target: "/admin/lixeira"})
	mustContain(t, w, "Restaure antes o marketplace “Loja Pai”.", `hx-post="/admin/marketplaces/`+mp.String()+`/restore"`)
	mustNotContain(t, w, `/admin/links/`+link.String()+`/restore`)

	// mesmo assim, forçar a restauração é recusado com aviso
	w = e.do(t, e.c.LinkRestore, req{method: http.MethodPost, id: link.String(), target: "/x", hx: true})
	if !strings.Contains(w.Header().Get("HX-Trigger"), `"kind":"error"`) || !strings.Contains(w.Header().Get("HX-Trigger"), "Restaure antes o marketplace") {
		t.Fatalf("aviso: %v", w.Header())
	}
	mustContain(t, w, "Filho")

	e.do(t, e.c.MarketplaceRestore, req{method: http.MethodPost, id: mp.String(), target: "/x", hx: true})
	w = e.do(t, e.c.Trash, req{target: "/admin/lixeira"})
	mustContain(t, w, `/admin/links/`+link.String()+`/restore`)
	w = e.do(t, e.c.LinkRestore, req{method: http.MethodPost, id: link.String(), target: "/x", hx: true})
	mustContain(t, w, "A lixeira está vazia.")
}

func TestTrashRestoreChannelKeepsOtherItems(t *testing.T) {
	e := newEnv(t)
	ch := e.newChannel(t, "Canal", "canal")
	mp := e.newMarketplace(t, "Outra")
	e.do(t, e.c.ChannelDelete, req{method: http.MethodPost, id: ch.String(), target: "/x", hx: true})
	e.do(t, e.c.MarketplaceDelete, req{method: http.MethodPost, id: mp.String(), target: "/x", hx: true})
	w := e.do(t, e.c.ChannelRestore, req{method: http.MethodPost, id: ch.String(), target: "/x", hx: true})
	mustContain(t, w, "Outra")
	mustNotContain(t, w, "<code>/canal</code>", ">Canal<")
	if !strings.Contains(w.Header().Get("HX-Trigger"), "Canal restaurado.") {
		t.Fatal(w.Header().Get("HX-Trigger"))
	}
}

func TestTrashRestoreUnknownExpiredOrForeignItems(t *testing.T) {
	e, other := newEnv(t), newEnv(t)
	foreign := other.newMarketplace(t, "Alheio")
	other.do(t, other.c.MarketplaceDelete, req{method: http.MethodPost, id: foreign.String(), target: "/x", hx: true})
	purged := e.newMarketplace(t, "Vencido")
	e.do(t, e.c.MarketplaceDelete, req{method: http.MethodPost, id: purged.String(), target: "/x", hx: true})
	if _, err := e.c.Service.Store.Pool.Exec(context.Background(), "UPDATE marketplaces SET purged_at=now() WHERE id=$1", purged); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{uuid.NewString(), "x", foreign.String(), purged.String()} {
		w := e.do(t, e.c.MarketplaceRestore, req{method: http.MethodPost, id: id, target: "/x", hx: true})
		if w.Code != http.StatusOK || !strings.Contains(w.Header().Get("HX-Trigger"), "prazo de restauração vencido") {
			t.Fatalf("id %q: %d %v", id, w.Code, w.Header())
		}
		mustNotContain(t, w, "Alheio", "Vencido")
	}
	// a restauração da outra organização não foi afetada
	if _, n, _ := other.c.Service.Store.ListMarketplacesWithOptions(context.Background(), other.session.OrgID, trashOpts()); n != 1 {
		t.Fatalf("item da outra org deve seguir na lixeira: %d", n)
	}
}

func TestTrashRestoreWithoutHTMXRedirects(t *testing.T) {
	e := newEnv(t)
	mp := e.newMarketplace(t, "Loja")
	e.do(t, e.c.MarketplaceDelete, req{method: http.MethodPost, id: mp.String(), target: "/x", hx: true})
	w := e.do(t, e.c.MarketplaceRestore, req{method: http.MethodPost, id: mp.String(), target: "/x"})
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/admin/lixeira" {
		t.Fatalf("%d %q", w.Code, w.Header().Get("Location"))
	}
}

func TestTrashShowsOnlyOwnOrganization(t *testing.T) {
	e, other := newEnv(t), newEnv(t)
	other.do(t, other.c.MarketplaceDelete, req{method: http.MethodPost, id: other.newMarketplace(t, "Segredo").String(), target: "/x", hx: true})
	w := e.do(t, e.c.Trash, req{target: "/admin/lixeira"})
	mustContain(t, w, "A lixeira está vazia.")
	mustNotContain(t, w, "Segredo")
}
