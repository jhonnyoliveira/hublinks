package admin_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/domain"
)

func (e env) newChannel(t *testing.T, name, segment string) uuid.UUID {
	t.Helper()
	ch, err := e.c.Service.CreateChannel(context.Background(), e.session.OrgID, name, segment)
	if err != nil {
		t.Fatal(err)
	}
	return ch.ID
}

func (e env) newLink(t *testing.T, title string, mp uuid.UUID, active bool) domain.AffiliateLink {
	t.Helper()
	l, err := e.c.Service.CreateLink(context.Background(), domain.AffiliateLink{OrgID: e.session.OrgID, MarketplaceID: mp, Title: title, DestinationURL: "https://example.com/" + strings.ReplaceAll(title, " ", "-"), Active: active})
	if err != nil {
		t.Fatal(err)
	}
	return domain.AffiliateLink{ID: l.ID, Code: l.Code}
}

func TestChannelsPageListsSegmentsAndEscapes(t *testing.T) {
	e := newEnv(t)
	mustContain(t, e.do(t, e.c.Channels, req{target: "/admin/channels"}), "Nenhum canal cadastrado")
	e.newChannel(t, `<i>Zap</i>`, "wapp")
	w := e.do(t, e.c.Channels, req{target: "/admin/channels"})
	mustContain(t, w, "<html", "Novo canal", "<code>/wapp</code>", "&lt;i&gt;Zap&lt;/i&gt;")
	mustNotContain(t, w, "<i>Zap")

	w = e.do(t, e.c.Channels, req{target: "/admin/channels?q=zzz", hx: true, hxTarget: "channels-list"})
	mustContain(t, w, `id="channels-list"`, "Nenhum canal encontrado para “zzz”")
	mustNotContain(t, w, "<html")
}

func TestChannelCreateFlow(t *testing.T) {
	e := newEnv(t)
	w := e.do(t, e.c.ChannelNew, req{target: "/admin/channels/new", hx: true})
	mustContain(t, w, "Novo canal", `name="segment"`, `pattern="[a-z0-9][a-z0-9\-]{0,19}"`, "Minúsculas, números e hífen")

	w = e.do(t, e.c.ChannelCreate, req{method: http.MethodPost, target: "/admin/channels", form: form("name", "Telegram", "segment", "tg"), hx: true})
	if w.Code != http.StatusOK || w.Header().Get("HX-Retarget") != "#channels-list" || !strings.Contains(w.Header().Get("HX-Trigger"), "Canal criado.") {
		t.Fatalf("%d %v", w.Code, w.Header())
	}
	mustContain(t, w, "Telegram", "<code>/tg</code>")

	w = e.do(t, e.c.ChannelCreate, req{method: http.MethodPost, target: "/admin/channels", form: form("name", "Outro", "segment", "x"), hx: false})
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/admin/channels" {
		t.Fatalf("sem HTMX: %d", w.Code)
	}
}

func TestChannelCreateInlineErrors(t *testing.T) {
	e := newEnv(t)
	e.newChannel(t, "WhatsApp", "wapp")

	cases := []struct {
		name, segment string
		status        int
		want          []string
	}{
		{"", "ok", 422, []string{`id="ch-name-error"`, "nome deve ter de 1 a 60 caracteres"}},
		{"API", "api", 422, []string{`id="ch-segment-error"`, "segmento inválido ou reservado", `value="api"`}},
		{"Maiúsculo", "WApp", 422, []string{"segmento inválido ou reservado"}},
		{"Duplicado", "wapp", 422, []string{"Este segmento já está em uso (inclusive por um canal na lixeira).", `aria-invalid="true"`}},
	}
	for _, tc := range cases {
		w := e.do(t, e.c.ChannelCreate, req{method: http.MethodPost, target: "/admin/channels", form: form("name", tc.name, "segment", tc.segment), hx: true})
		if w.Code != tc.status {
			t.Fatalf("%q/%q: status %d", tc.name, tc.segment, w.Code)
		}
		mustContain(t, w, tc.want...)
		mustNotContain(t, w, "<html")
	}
}

func TestChannelTrashedSegmentStaysReserved(t *testing.T) {
	e := newEnv(t)
	id := e.newChannel(t, "WhatsApp", "wapp")
	e.do(t, e.c.ChannelDelete, req{method: http.MethodPost, id: id.String(), target: "/x", hx: true})
	w := e.do(t, e.c.ChannelCreate, req{method: http.MethodPost, target: "/admin/channels", form: form("name", "Novo", "segment", "wapp"), hx: true})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("segmento de canal na lixeira deveria seguir reservado: %d", w.Code)
	}
}

func TestChannelEditUpdateWarnsAboutSegmentChange(t *testing.T) {
	e := newEnv(t)
	id := e.newChannel(t, "WhatsApp", "wapp")
	w := e.do(t, e.c.ChannelEdit, req{id: id.String(), target: "/x", hx: true})
	mustContain(t, w, "Editar canal", `value="WhatsApp"`, `value="wapp"`, "Alterar o segmento invalida as URLs")

	w = e.do(t, e.c.ChannelUpdate, req{method: http.MethodPost, id: id.String(), target: "/x", form: form("name", "Zap", "segment", "zap"), hx: true})
	if w.Code != http.StatusOK {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	mustContain(t, w, "Zap", "<code>/zap</code>")
	mustNotContain(t, w, "<code>/wapp</code>")

	w = e.do(t, e.c.ChannelUpdate, req{method: http.MethodPost, id: id.String(), target: "/x", form: form("name", "Zap", "segment", "admin"), hx: true})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("segmento reservado na edição: %d", w.Code)
	}
}

func TestChannelDeleteConfirmationReportsAffectedLinks(t *testing.T) {
	e := newEnv(t)
	id := e.newChannel(t, "WhatsApp", "wapp")
	w := e.do(t, e.c.ChannelConfirmDelete, req{id: id.String(), target: "/x", hx: true})
	mustContain(t, w, "Excluir canal", "30 dias", "segmento continua reservado")
	mustNotContain(t, w, "terá a URL", "terão a URL")

	mp := e.newMarketplace(t, "Loja")
	e.newLink(t, "um", mp, true)
	w = e.do(t, e.c.ChannelConfirmDelete, req{id: id.String(), target: "/x", hx: true})
	mustContain(t, w, "1 link terá a URL deste canal invalidada")

	e.newLink(t, "dois", mp, true)
	e.newLink(t, "inativo", mp, false)
	w = e.do(t, e.c.ChannelConfirmDelete, req{id: id.String(), target: "/x", hx: true})
	mustContain(t, w, "2 links terão a URL deste canal invalidada")
}

func TestChannelDeleteGoesToTrashEvenWithLinks(t *testing.T) {
	e := newEnv(t)
	id := e.newChannel(t, "WhatsApp", "wapp")
	e.newLink(t, "um", e.newMarketplace(t, "Loja"), true)
	w := e.do(t, e.c.ChannelDelete, req{method: http.MethodPost, id: id.String(), target: "/x", hx: true})
	if w.Code != http.StatusOK || !strings.Contains(w.Header().Get("HX-Trigger"), "Canal enviado para a lixeira.") {
		t.Fatalf("%d %v", w.Code, w.Header())
	}
	mustNotContain(t, w, "<code>/wapp</code>")
	mustContain(t, w, "Nenhum canal cadastrado")
	w = e.do(t, e.c.ChannelDelete, req{method: http.MethodPost, id: id.String(), target: "/x", hx: true})
	if !strings.Contains(w.Header().Get("HX-Trigger"), "não encontrado") {
		t.Fatalf("excluir de novo deve avisar: %v", w.Header())
	}
}

func TestChannelForeignAndMalformedIDs(t *testing.T) {
	e, other := newEnv(t), newEnv(t)
	foreign := other.newChannel(t, "De outra org", "outra")
	for _, id := range []string{foreign.String(), "x", uuid.NewString()} {
		for name, h := range map[string]http.HandlerFunc{"edit": e.c.ChannelEdit, "confirm": e.c.ChannelConfirmDelete} {
			w := e.do(t, h, req{id: id, target: "/x", hx: true})
			if w.Code != http.StatusOK || !strings.Contains(w.Header().Get("HX-Trigger"), "Canal não encontrado") {
				t.Fatalf("%s %q: %d %v", name, id, w.Code, w.Header())
			}
			mustNotContain(t, w, "De outra org")
		}
	}
}
