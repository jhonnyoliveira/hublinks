package stats_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/domain"
	"github.com/hublinks/hublinks/internal/stats"
	"github.com/hublinks/hublinks/internal/store/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

var tz, _ = time.LoadLocation("America/Sao_Paulo")

type fixture struct {
	pool         *pgxpool.Pool
	svc          stats.Service
	org          uuid.UUID
	linkA, linkB uuid.UUID
	wapp, ig     uuid.UUID
	period       stats.Period
}

func exec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func seedOrg(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	org := uuid.New()
	exec(t, pool, "INSERT INTO organizations(id,name,slug,created_at) VALUES($1,'Org',$2,now())", org, org.String())
	return org
}

func seedLink(t *testing.T, pool *pgxpool.Pool, org uuid.UUID, title, code, policy string, override *string) uuid.UUID {
	mp, link := uuid.New(), uuid.New()
	exec(t, pool, "INSERT INTO marketplaces(id,org_id,name,shorten_policy,created_at,updated_at) VALUES($1,$2,$3,$4,now(),now())", mp, org, "Loja "+title, policy)
	exec(t, pool, "INSERT INTO affiliate_links(id,org_id,marketplace_id,title,destination_url,shorten_policy_override,active,created_at,updated_at) VALUES($1,$2,$3,$4,'https://example.com',$5,true,now(),now())", link, org, mp, title, override)
	exec(t, pool, "INSERT INTO short_codes(code,org_id,target_type,target_id,created_at) VALUES($1,$2,'affiliate_link',$3,now())", code, org, link)
	return link
}

func seedChannel(t *testing.T, pool *pgxpool.Pool, org uuid.UUID, name, segment string) uuid.UUID {
	id := uuid.New()
	exec(t, pool, "INSERT INTO channels(id,org_id,name,segment,created_at,updated_at) VALUES($1,$2,$3,$4,now(),now())", id, org, name, segment)
	return id
}

func event(t *testing.T, pool *pgxpool.Pool, org, target uuid.UUID, channel *uuid.UUID, at string, uurl, utarget, bot bool) {
	occurred, err := time.Parse(time.RFC3339, at)
	if err != nil {
		t.Fatal(err)
	}
	exec(t, pool, "INSERT INTO click_events(id,occurred_at,org_id,event_type,code,target_type,target_id,channel_id,visitor_id,is_unique_url,is_unique_target,is_bot) VALUES($1,$2,$3,'click','aaaaaaa','affiliate_link',$4,$5,$6,$7,$8,$9)",
		uuid.New(), occurred, org, target, channel, make([]byte, 32), uurl, utarget, bot)
}

func daily(t *testing.T, pool *pgxpool.Pool, org, target, channel uuid.UUID, day string, clicks, uurl, utarget int) {
	exec(t, pool, "INSERT INTO click_daily(org_id,day,event_type,target_type,target_id,channel_id,clicks,unique_url,unique_target) VALUES($1,$2::date,'click','affiliate_link',$3,$4,$5,$6,$7)", org, day, target, channel, clicks, uurl, utarget)
}

// newFixture: hoje é 12/10/2026 (REPORT_TZ). O dia 10/10 está agregado; o dia
// 11/10 não (cai em click_events) e 12/10 é o dia corrente.
func newFixture(t *testing.T) fixture {
	pool := testdb.New(t)
	f := fixture{pool: pool, org: seedOrg(t, pool)}
	f.svc = stats.Service{Pool: pool, TZ: tz, Now: func() time.Time { return time.Date(2026, 10, 12, 12, 0, 0, 0, tz) }}
	f.linkA = seedLink(t, pool, f.org, "Fone", "aaaaaaa", "shorten", nil)
	direct := "direct"
	f.linkB = seedLink(t, pool, f.org, "Cabo", "bbbbbbb", "shorten", &direct)
	f.wapp = seedChannel(t, pool, f.org, "WhatsApp", "wapp")
	f.ig = seedChannel(t, pool, f.org, "Instagram", "ig")

	// 10/10: agregado. click_events do mesmo dia deve ser ignorado (já está em click_daily).
	exec(t, pool, "INSERT INTO aggregated_days(org_id,day,aggregated_at) VALUES($1,'2026-10-10',now())", f.org)
	daily(t, pool, f.org, f.linkA, uuid.Nil, "2026-10-10", 5, 3, 3)
	daily(t, pool, f.org, f.linkA, f.wapp, "2026-10-10", 2, 2, 1)
	daily(t, pool, f.org, f.linkB, uuid.Nil, "2026-10-10", 4, 4, 4) // histórico de quando era shorten
	event(t, pool, f.org, f.linkA, nil, "2026-10-10T15:00:00Z", true, true, false)
	// 02:30 UTC de 12/10 = 23:30 de 11/10 em Brasília: conta em 11/10.
	event(t, pool, f.org, f.linkA, nil, "2026-10-12T02:30:00Z", true, true, false)
	// Hoje: um real pelo canal (único na URL, não no alvo) e um robô.
	event(t, pool, f.org, f.linkA, &f.wapp, "2026-10-12T15:00:00Z", true, false, false)
	event(t, pool, f.org, f.linkA, nil, "2026-10-12T15:00:00Z", true, true, true)

	// Outra organização com dados, que nunca pode aparecer.
	other := seedOrg(t, pool)
	otherLink := seedLink(t, pool, other, "Outro", "ccccccc", "shorten", nil)
	event(t, pool, other, otherLink, nil, "2026-10-12T15:00:00Z", true, true, false)

	p, err := stats.ParsePeriod("7d", "", "", f.svc.Today(), tz)
	if err != nil {
		t.Fatal(err)
	}
	f.period = p
	return f
}

func TestSummaryUnionsDailyAndCurrentDay(t *testing.T) {
	f := newFixture(t)
	s, err := f.svc.Summary(context.Background(), f.org, f.period)
	if err != nil {
		t.Fatal(err)
	}
	// A: 7 (10/10) + 1 (11/10) + 1 (hoje) = 9, mais 4 do histórico de B.
	if s.Clicks != 13 || s.Unique != 9 || s.ActiveLinks != 2 {
		t.Fatalf("summary = %+v", s)
	}
	if len(s.Daily) != 7 {
		t.Fatalf("dias: %d", len(s.Daily))
	}
	byDay := map[string]stats.DayCount{}
	for _, d := range s.Daily {
		byDay[d.Day] = d
	}
	if byDay["2026-10-10"].Clicks != 11 || byDay["2026-10-11"] != (stats.DayCount{Day: "2026-10-11", Clicks: 1, Unique: 1}) || byDay["2026-10-12"].Clicks != 1 || byDay["2026-10-06"].Clicks != 0 {
		t.Fatalf("daily = %+v", s.Daily)
	}
	if len(s.TopLinks) != 1 || s.TopLinks[0].Link.ID != f.linkA || *s.TopLinks[0].Clicks != 9 {
		t.Fatalf("top = %+v", s.TopLinks)
	}
}

func TestLinksUniqueIsTargetReachAndDirectIsNotTrackable(t *testing.T) {
	f := newFixture(t)
	items, total, err := f.svc.Links(context.Background(), f.org, f.period, stats.LinkFilter{})
	if err != nil || total != 2 || len(items) != 2 {
		t.Fatalf("items=%d total=%d err=%v", len(items), total, err)
	}
	a, b := items[0], items[1]
	if a.Link.ID != f.linkA || !a.Link.Trackable || *a.Clicks != 9 || *a.Unique != 5 {
		t.Fatalf("A = %+v", a)
	}
	if b.Link.ID != f.linkB || b.Link.Trackable || b.Clicks != nil || b.Unique != nil || b.History == nil || b.History.Clicks != 4 {
		t.Fatalf("B = %+v", b)
	}
	// Filtro por texto e paginação.
	items, total, err = f.svc.Links(context.Background(), f.org, f.period, stats.LinkFilter{Q: "fone"})
	if err != nil || total != 1 || len(items) != 1 || items[0].Link.ID != f.linkA {
		t.Fatalf("q: %+v %d %v", items, total, err)
	}
	items, total, _ = f.svc.Links(context.Background(), f.org, f.period, stats.LinkFilter{Page: 2, PerPage: 1})
	if total != 2 || len(items) != 1 || items[0].Link.ID != f.linkB {
		t.Fatalf("página 2: %+v %d", items, total)
	}
}

func TestLinkDetailByChannelUsesUniqueURL(t *testing.T) {
	f := newFixture(t)
	d, err := f.svc.Link(context.Background(), f.org, f.linkA, f.period)
	if err != nil {
		t.Fatal(err)
	}
	if d.Totals == nil || *d.Totals != (stats.Counts{Clicks: 9, Unique: 5}) || d.History != nil {
		t.Fatalf("totais = %+v", d)
	}
	if len(d.ByChannel) != 2 {
		t.Fatalf("canais = %+v", d.ByChannel)
	}
	if d.ByChannel[0].Channel != nil || d.ByChannel[0].Clicks != 6 || d.ByChannel[0].Unique != 4 {
		t.Fatalf("sem canal = %+v", d.ByChannel[0])
	}
	if d.ByChannel[1].Channel == nil || d.ByChannel[1].Channel.Segment != "wapp" || d.ByChannel[1].Clicks != 3 || d.ByChannel[1].Unique != 3 {
		t.Fatalf("wapp = %+v", d.ByChannel[1])
	}
	// Link direct: sem totais, só histórico, e o histórico por dia é preservado.
	d, err = f.svc.Link(context.Background(), f.org, f.linkB, f.period)
	if err != nil || d.Totals != nil || d.Link.Trackable || d.History == nil || d.History.Clicks != 4 || len(d.Daily) != 7 {
		t.Fatalf("direct = %+v (%v)", d, err)
	}
}

func TestChannelsUniqueIsSumUniqueURL(t *testing.T) {
	f := newFixture(t)
	got, err := f.svc.Channels(context.Background(), f.org, f.period)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]stats.ChannelStat{}
	for _, c := range got {
		key := ""
		if c.Channel != nil {
			key = c.Channel.Segment
		}
		by[key] = c
	}
	if by[""].Clicks != 10 || by[""].Unique != 8 { // A: 6 (3+1 únicos) e B: 4 (4 únicos)
		t.Fatalf("sem canal = %+v", by[""])
	}
	if by["wapp"].Clicks != 3 || by["wapp"].Unique != 3 {
		t.Fatalf("wapp = %+v", by["wapp"])
	}
	if c, ok := by["ig"]; !ok || c.Clicks != 0 {
		t.Fatalf("canal sem cliques deveria aparecer com zero: %+v", by)
	}
}

func TestStatsAreScopedToOrganization(t *testing.T) {
	f := newFixture(t)
	other := seedOrg(t, f.pool)
	ctx := context.Background()
	s, err := f.svc.Summary(ctx, other, f.period)
	if err != nil || s.Clicks != 0 || s.ActiveLinks != 0 || len(s.TopLinks) != 0 {
		t.Fatalf("summary de outra org = %+v (%v)", s, err)
	}
	if items, total, _ := f.svc.Links(ctx, other, f.period, stats.LinkFilter{}); total != 0 || len(items) != 0 {
		t.Fatalf("links de outra org: %+v", items)
	}
	if _, err := f.svc.Link(ctx, other, f.linkA, f.period); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("link de outra org: %v", err)
	}
	ch, _ := f.svc.Channels(ctx, other, f.period)
	if len(ch) != 0 {
		t.Fatalf("canais de outra org: %+v", ch)
	}
}

func TestCustomPeriodExcludesOtherDays(t *testing.T) {
	f := newFixture(t)
	p, err := stats.ParsePeriod("", "2026-10-11", "2026-10-11", f.svc.Today(), tz)
	if err != nil {
		t.Fatal(err)
	}
	s, err := f.svc.Summary(context.Background(), f.org, p)
	if err != nil || s.Clicks != 1 || len(s.Daily) != 1 {
		t.Fatalf("summary = %+v (%v)", s, err)
	}
}

func TestTotalsForLinks(t *testing.T) {
	f := newFixture(t)
	got, err := f.svc.Totals(context.Background(), f.org, f.period, []uuid.UUID{f.linkA, f.linkB, uuid.New()})
	if err != nil || len(got) != 2 || got[f.linkA] != (stats.Counts{Clicks: 9, Unique: 5}) || got[f.linkB].Clicks != 4 {
		t.Fatalf("totals = %+v (%v)", got, err)
	}
	if got, _ := f.svc.Totals(context.Background(), f.org, f.period, nil); len(got) != 0 {
		t.Fatalf("sem ids: %+v", got)
	}
	other, _ := f.svc.Totals(context.Background(), seedOrg(t, f.pool), f.period, []uuid.UUID{f.linkA})
	if len(other) != 0 {
		t.Fatalf("outra org: %+v", other)
	}
}
