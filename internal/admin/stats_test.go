package admin_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/domain"
)

// click grava um evento real (não robô) agora, no canal informado (nil = URL curta).
func (e env) click(t *testing.T, link uuid.UUID, channel *uuid.UUID, uniqueURL, uniqueTarget, bot bool) {
	t.Helper()
	_, err := e.pool.Exec(context.Background(), "INSERT INTO click_events(id,occurred_at,org_id,event_type,code,target_type,target_id,channel_id,visitor_id,is_unique_url,is_unique_target,is_bot) VALUES($1,$2,$3,'click','aaaaaaa','affiliate_link',$4,$5,$6,$7,$8,$9)",
		uuid.New(), time.Now().UTC(), e.session.OrgID, link, channel, make([]byte, 32), uniqueURL, uniqueTarget, bot)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDashboardShowsTotalsTopLinksChannelsAndNotice(t *testing.T) {
	e := newEnv(t)
	mp := e.newMarketplace(t, "Loja")
	fone := e.fullLink(t, "Fone <i>bt</i>", mp, nil, true)
	for range 3 {
		e.click(t, fone, nil, true, true, false)
	}
	e.click(t, fone, nil, true, true, true) // robô não conta

	w := e.do(t, e.dash.Show, req{target: "/admin"})
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	mustContain(t, w, "<html", "Cliques", "Visitantes únicos", "Links ativos",
		`<td class="num">3</td>`, "Fone &lt;i&gt;bt&lt;/i&gt;", "URL curta (sem canal)",
		"Links mais clicados", "Como os únicos são contados", "/static/vendor/uplot.iife.min.js", "/static/js/chart.js",
		`data-chart-data`, "Ver dados em tabela", `aria-current="true"`)
	mustNotContain(t, w, "<i>bt</i>")
	// seletor de período atualiza só o painel e a URL
	mustContain(t, w, `hx-target="#dashboard"`, `hx-push-url="true"`, "period=30d")
}

func TestDashboardPeriodSelectionAndFragment(t *testing.T) {
	e := newEnv(t)
	w := e.do(t, e.dash.Show, req{target: "/admin?period=30d", hx: true, hxTarget: "dashboard"})
	mustContain(t, w, `id="dashboard"`, `<a href="/admin?period=30d" hx-get="/admin?period=30d" hx-target="#dashboard" hx-swap="outerHTML" hx-push-url="true" aria-current="true">30 dias</a>`)
	mustNotContain(t, w, "<html")

	w = e.do(t, e.dash.Show, req{target: "/admin?from=2026-01-01&to=2026-01-10"})
	mustContain(t, w, "01/01/2026 a 10/01/2026", `name="from" value="2026-01-01"`)
	mustNotContain(t, w, `aria-current="true"`)

	w = e.do(t, e.dash.Show, req{target: "/admin?period=1y"})
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	mustContain(t, w, "Período inválido", `aria-current="true">7 dias`)
}

func TestLinksListShowsClicksAndNeverZeroForDirect(t *testing.T) {
	e := newEnv(t)
	mp := e.newMarketplace(t, "Loja")
	direct := domain.PolicyDirect
	trackable := e.fullLink(t, "Rastreável", mp, nil, true)
	notTrackable := e.fullLink(t, "Direto", mp, &direct, true)
	e.click(t, trackable, nil, true, true, false)
	e.click(t, trackable, nil, false, false, false)
	e.click(t, notTrackable, nil, true, true, false) // histórico

	w := e.do(t, e.c.Links, req{target: "/admin/links"})
	body := w.Body.String()
	row := func(id uuid.UUID) string {
		start := strings.Index(body, `id="link-`+id.String()+`"`)
		if start < 0 {
			t.Fatalf("linha de %s ausente", id)
		}
		return body[start : start+strings.Index(body[start:], "</tr>")]
	}
	if r := row(trackable); !strings.Contains(r, `<td class="num">2</td><td class="num hide-sm">1</td>`) {
		t.Fatalf("linha rastreável:\n%s", r)
	}
	if r := row(notTrackable); !strings.Contains(r, "Não rastreável") || !strings.Contains(r, "histórico: 1 cliques") || strings.Contains(r, `<td class="num">0</td>`) {
		t.Fatalf("linha direct:\n%s", r)
	}
	mustContain(t, w, `id="period-state"`, `hx-include="#period-state"`, `aria-current="true">7 dias`)
}

func TestLinkToggleRowKeepsCounts(t *testing.T) {
	e := newEnv(t)
	mp := e.newMarketplace(t, "Loja")
	id := e.fullLink(t, "Fone", mp, nil, true)
	e.click(t, id, nil, true, true, false)
	w := e.do(t, e.c.LinkToggle, req{method: http.MethodPost, target: "/admin/links/" + id.String() + "/toggle", id: id.String(), form: form("active", "false"), hx: true, currentURL: "http://x/admin/links?period=30d"})
	mustContain(t, w, `<td class="num">1</td>`)
}

func TestLinkShowStatsByChannelAndDirectNotice(t *testing.T) {
	e := newEnv(t)
	mp := e.newMarketplace(t, "Loja")
	id := e.fullLink(t, "Fone", mp, nil, true)
	ch, err := e.c.Service.CreateChannel(context.Background(), e.session.OrgID, "WhatsApp", "wapp")
	if err != nil {
		t.Fatal(err)
	}
	e.click(t, id, nil, true, true, false)
	e.click(t, id, &ch.ID, true, false, false)

	w := e.do(t, e.c.LinkShow, req{target: "/admin/links/" + id.String(), id: id.String()})
	mustContain(t, w, "Por canal", "WhatsApp", "https://hub.example/wapp/", "URL curta (sem canal)", "Cliques por dia", "data-chart-data", "/static/js/chart.js")

	w = e.do(t, e.c.LinkShow, req{target: "/admin/links/" + id.String() + "?period=90d", id: id.String(), hx: true, hxTarget: "link-stats"})
	mustContain(t, w, `id="link-stats"`, `>90 dias</a>`)
	mustNotContain(t, w, "<html")

	direct := domain.PolicyDirect
	d := e.fullLink(t, "Direto", mp, &direct, true)
	e.click(t, d, nil, true, true, false)
	w = e.do(t, e.c.LinkShow, req{target: "/admin/links/" + d.String(), id: d.String()})
	mustContain(t, w, "Não rastreável", "Histórico de quando o link era encurtado")
	mustNotContain(t, w, "Visitantes únicos")
}
