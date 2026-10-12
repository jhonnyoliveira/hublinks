package maintenance_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hublinks/hublinks/internal/maintenance"
)

func TestAggregatesByLocalDayIgnoringBots(t *testing.T) {
	e := newEnv(t)
	wapp := e.channel(t, "WhatsApp", "wapp")
	// 09/10 em Brasília (UTC-3): 12:00 e 23:30 locais (este já é 10/10 em UTC)
	e.event(t, "2026-10-09T15:00:00Z", evOpt{uurl: true, utarget: true})
	e.event(t, "2026-10-09T16:00:00Z", evOpt{uurl: false, utarget: false})
	e.event(t, "2026-10-09T17:00:00Z", evOpt{channel: &wapp, uurl: true, utarget: false})
	e.event(t, "2026-10-10T02:30:00Z", evOpt{uurl: true, utarget: false}) // 23:30 de 09/10
	e.event(t, "2026-10-09T18:00:00Z", evOpt{bot: true, uurl: true, utarget: true})
	// 10/10 local começa às 03:00 UTC
	e.event(t, "2026-10-10T03:00:00Z", evOpt{uurl: true, utarget: true})

	n, err := e.mgr.AggregatePending(context.Background())
	if err != nil || n != 2 {
		t.Fatalf("dias consolidados = %d (%v)", n, err)
	}
	got := e.daily(t, e.org)
	want := []dailyRow{
		{"2026-10-09", nilUUID, 3, 2, 1}, // 3 cliques sem canal, robô fora
		{"2026-10-09", wapp.String(), 1, 1, 0},
		{"2026-10-10", nilUUID, 1, 1, 1},
	}
	if len(got) != len(want) {
		t.Fatalf("click_daily = %v", got)
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("faltou %+v em %v", w, got)
		}
	}
}

func TestAggregationIsIdempotentAndRegistersDays(t *testing.T) {
	e := newEnv(t)
	e.event(t, "2026-10-09T15:00:00Z", evOpt{uurl: true, utarget: true})
	e.event(t, "2026-10-09T15:05:00Z", evOpt{uurl: false, utarget: false})
	ctx := context.Background()
	if _, err := e.mgr.AggregatePending(ctx); err != nil {
		t.Fatal(err)
	}
	first := e.daily(t, e.org)
	if e.count(t, "SELECT count(*) FROM aggregated_days WHERE org_id=$1 AND day='2026-10-09'", e.org) != 1 {
		t.Fatal("aggregated_days deveria registrar o dia")
	}
	// segunda execução: nada pendente
	if n, err := e.mgr.AggregatePending(ctx); err != nil || n != 0 {
		t.Fatalf("segunda execução: %d (%v)", n, err)
	}
	// reconsolidar o mesmo dia produz os mesmos valores
	day := time.Date(2026, 10, 9, 0, 0, 0, 0, tz)
	for i := 0; i < 2; i++ {
		if _, err := e.mgr.AggregateDay(ctx, day); err != nil {
			t.Fatal(err)
		}
		again := e.daily(t, e.org)
		if len(again) != len(first) {
			t.Fatalf("reexecução %d mudou o resultado: %v vs %v", i+1, again, first)
		}
		for r := range first {
			if !again[r] {
				t.Fatalf("reexecução %d mudou %v", i+1, r)
			}
		}
	}
	if e.count(t, "SELECT count(*) FROM aggregated_days WHERE org_id=$1", e.org) != 1 {
		t.Fatal("aggregated_days não deveria duplicar")
	}
}

func TestCatchesUpEveryPendingPastDayButNotToday(t *testing.T) {
	e := newEnv(t)
	for _, at := range []string{"2026-09-20T15:00:00Z", "2026-10-01T15:00:00Z", "2026-10-11T15:00:00Z"} {
		e.event(t, at, evOpt{uurl: true, utarget: true})
	}
	e.event(t, "2026-10-12T15:00:00Z", evOpt{uurl: true, utarget: true}) // hoje (12:00 local), não consolida
	ctx := context.Background()
	if n, err := e.mgr.AggregatePending(ctx); err != nil || n != 3 {
		t.Fatalf("dias consolidados = %d (%v)", n, err)
	}
	if e.count(t, "SELECT count(*) FROM aggregated_days WHERE org_id=$1", e.org) != 3 || e.count(t, "SELECT count(*) FROM click_daily WHERE org_id=$1 AND day='2026-10-12'", e.org) != 0 {
		t.Fatal("o dia corrente não pode ser consolidado")
	}
	if e.count(t, "SELECT count(*) FROM aggregated_days WHERE org_id=$1 AND day='2026-10-12'", e.org) != 0 {
		t.Fatal("o dia corrente não pode constar em aggregated_days")
	}
	// um evento da 23h59 de hoje, depois de virar o dia, passa a ser pendente
	later := e.mgr
	later.Now = func() time.Time { return fixedNow.AddDate(0, 0, 1) }
	if n, err := later.AggregatePending(ctx); err != nil || n != 1 {
		t.Fatalf("no dia seguinte, hoje vira passado: %d (%v)", n, err)
	}
}

func TestBotOnlyDaysAreMarkedSoCleanupCanRemoveThem(t *testing.T) {
	e := newEnv(t)
	e.event(t, "2026-10-09T15:00:00Z", evOpt{bot: true, uurl: false})
	if n, err := e.mgr.AggregatePending(context.Background()); err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if e.count(t, "SELECT count(*) FROM aggregated_days WHERE org_id=$1 AND day='2026-10-09'", e.org) != 1 || e.count(t, "SELECT count(*) FROM click_daily WHERE org_id=$1", e.org) != 0 {
		t.Fatal("dia só com robôs: marcado como consolidado, sem contagens")
	}
}

func TestReaggregationPicksUpLateEventsAndRefusesCleanedDays(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.event(t, "2026-10-09T15:00:00Z", evOpt{uurl: true, utarget: true})
	if _, err := e.mgr.AggregatePending(ctx); err != nil {
		t.Fatal(err)
	}
	e.event(t, "2026-10-09T16:00:00Z", evOpt{uurl: false, utarget: false}) // chegou depois da consolidação
	day := time.Date(2026, 10, 9, 0, 0, 0, 0, tz)
	if _, err := e.mgr.AggregateDay(ctx, day); err != nil {
		t.Fatal(err)
	}
	if !e.daily(t, e.org)[dailyRow{"2026-10-09", nilUUID, 2, 1, 1}] {
		t.Fatalf("a reconsolidação deveria incluir o evento tardio: %v", e.daily(t, e.org))
	}
	// dia fora da retenção já consolidado: reconsolidar apagaria o histórico
	old := e.mgr.Today().AddDate(0, 0, -400)
	e.event(t, old.Add(15*time.Hour).UTC().Format(time.RFC3339), evOpt{uurl: true, utarget: true})
	if _, err := e.mgr.AggregatePending(ctx); err != nil {
		t.Fatal(err)
	}
	before := e.count(t, "SELECT sum(clicks) FROM click_daily WHERE org_id=$1", e.org)
	if _, err := e.mgr.AggregateDay(ctx, old); !errors.Is(err, maintenance.ErrDayCleaned) {
		t.Fatalf("esperava ErrDayCleaned: %v", err)
	}
	if after := e.count(t, "SELECT sum(clicks) FROM click_daily WHERE org_id=$1", e.org); after != before {
		t.Fatalf("a recusa não pode alterar contagens: %d → %d", before, after)
	}
	// o dia corrente e o futuro são recusados
	if _, err := e.mgr.AggregateDay(ctx, e.mgr.Today()); err == nil {
		t.Fatal("hoje não pode ser consolidado")
	}
}

func TestAggregationIsPerOrganization(t *testing.T) {
	e := newEnv(t)
	other := newOrg(t, e.pool)
	otherLink := newLink(t, e.pool, other, "bbbbbbb")
	e.event(t, "2026-10-09T15:00:00Z", evOpt{uurl: true, utarget: true})
	e.event(t, "2026-10-09T15:00:00Z", evOpt{org: other, target: otherLink, uurl: true, utarget: true})
	e.event(t, "2026-10-09T15:01:00Z", evOpt{org: other, target: otherLink, uurl: false, utarget: false})
	if n, err := e.mgr.AggregatePending(context.Background()); err != nil || n != 2 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if !e.daily(t, e.org)[dailyRow{"2026-10-09", nilUUID, 1, 1, 1}] || !e.daily(t, other)[dailyRow{"2026-10-09", nilUUID, 2, 1, 1}] {
		t.Fatalf("contagens por organização: %v / %v", e.daily(t, e.org), e.daily(t, other))
	}
	if e.count(t, "SELECT count(*) FROM click_daily WHERE target_id=$1 AND org_id=$2", otherLink, e.org) != 0 {
		t.Fatal("nenhuma linha pode vazar entre organizações")
	}
}
