package maintenance_test

import (
	"context"
	"testing"
	"time"

	"github.com/hublinks/hublinks/internal/stats"
)

// at devolve o instante (UTC, RFC 3339) de hh:mm locais, n dias antes de hoje.
func (e env) at(daysAgo int, hour, min int) string {
	d := e.mgr.Today().AddDate(0, 0, -daysAgo)
	return time.Date(d.Year(), d.Month(), d.Day(), hour, min, 0, 0, tz).UTC().Format(time.RFC3339)
}

func (e env) events(t *testing.T) int {
	return e.count(t, "SELECT count(*) FROM click_events")
}

func TestCleanupRemovesOnlyAggregatedDaysBeyondRetention(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.event(t, e.at(400, 12, 0), evOpt{uurl: true, utarget: true}) // real, antigo e consolidado: sai
	e.event(t, e.at(400, 12, 5), evOpt{bot: true})                 // robô, idem: sai
	e.event(t, e.at(100, 12, 0), evOpt{uurl: true, utarget: true}) // real dentro da retenção: fica
	e.event(t, e.at(40, 12, 0), evOpt{bot: true})                  // robô com 40 dias (> 30): sai
	e.event(t, e.at(20, 12, 0), evOpt{bot: true})                  // robô com 20 dias: fica
	if _, err := e.mgr.AggregatePending(ctx); err != nil {
		t.Fatal(err)
	}
	// dia antigo SEM consolidação: preservado mesmo além da retenção
	exec(t, e.pool, "DELETE FROM aggregated_days WHERE org_id=$1 AND day=$2::date", e.org, e.mgr.Today().AddDate(0, 0, -400).Format("2006-01-02"))
	e.event(t, e.at(500, 12, 0), evOpt{uurl: true, utarget: true}) // dia 500 nunca consolidado
	n, err := e.mgr.Cleanup(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// o dia -400 perdeu a marca, então também fica; só o robô de 40 dias sai
	if n != 1 || e.events(t) != 5 {
		t.Fatalf("removidos=%d, restantes=%d", n, e.events(t))
	}
	// consolida de novo o dia -400 e limpa: agora saem o real e o robô daquele dia
	exec(t, e.pool, "INSERT INTO aggregated_days(org_id,day,aggregated_at) VALUES($1,$2::date,now())", e.org, e.mgr.Today().AddDate(0, 0, -400).Format("2006-01-02"))
	if n, err = e.mgr.Cleanup(ctx); err != nil || n != 2 {
		t.Fatalf("removidos=%d (%v)", n, err)
	}
	if e.events(t) != 3 { // -500 (sem consolidar), -100 e robô de -20
		t.Fatalf("restantes=%d", e.events(t))
	}
}

func TestCleanupCutIsByLocalDayAtTheBoundary(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	// dia-limite dos eventos reais: hoje − 395. 23:30 locais dele é preservado;
	// 23:30 locais do dia anterior sai.
	e.event(t, e.at(395, 23, 30), evOpt{uurl: true, utarget: true})
	e.event(t, e.at(395, 0, 0), evOpt{uurl: true, utarget: true})
	e.event(t, e.at(396, 23, 30), evOpt{uurl: true, utarget: true})
	// robôs: dia-limite hoje − 30
	e.event(t, e.at(30, 23, 30), evOpt{bot: true})
	e.event(t, e.at(31, 23, 30), evOpt{bot: true})
	if _, err := e.mgr.AggregatePending(ctx); err != nil {
		t.Fatal(err)
	}
	n, err := e.mgr.Cleanup(ctx)
	if err != nil || n != 2 {
		t.Fatalf("removidos=%d (%v)", n, err)
	}
	if e.count(t, "SELECT count(*) FROM click_events WHERE NOT is_bot") != 2 || e.count(t, "SELECT count(*) FROM click_events WHERE is_bot") != 1 {
		t.Fatalf("restantes inesperados: reais=%d robôs=%d", e.count(t, "SELECT count(*) FROM click_events WHERE NOT is_bot"), e.count(t, "SELECT count(*) FROM click_events WHERE is_bot"))
	}
}

// SC-006: as contagens totais e únicas são idênticas antes e depois da limpeza.
func TestCleanupPreservesTotalsAndUniques(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	wapp := e.channel(t, "WhatsApp", "wapp")
	for i := 0; i < 10; i++ {
		e.event(t, e.at(420, 10+i%5, i), evOpt{uurl: i < 4, utarget: i < 3})
		e.event(t, e.at(420, 18, i), evOpt{channel: &wapp, uurl: i < 2, utarget: false})
	}
	for i := 0; i < 5; i++ {
		e.event(t, e.at(2, 9, i), evOpt{uurl: i == 0, utarget: i == 0})
	}
	e.event(t, e.at(0, 8, 0), evOpt{uurl: true, utarget: true}) // hoje, fica em click_events
	e.event(t, e.at(420, 11, 0), evOpt{bot: true, uurl: true, utarget: true})

	before := e.totals(t)
	chBefore := e.channelTotals(t)
	if before.Clicks != 26 || before.Unique != 5 {
		t.Fatalf("antes da limpeza: %+v", before)
	}
	if _, err := e.mgr.Run(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if got := e.count(t, "SELECT count(*) FROM click_events WHERE occurred_at < $1", e.mgr.Today().AddDate(0, 0, -395).UTC()); got != 0 {
		t.Fatalf("eventos antigos restantes: %d", got)
	}
	if after := e.totals(t); after != before {
		t.Fatalf("totais mudaram: %+v → %+v", before, after)
	}
	if after := e.channelTotals(t); len(after) != len(chBefore) {
		t.Fatalf("canais mudaram: %v → %v", chBefore, after)
	} else {
		for k, v := range chBefore {
			if after[k] != v {
				t.Fatalf("canal %s mudou: %v → %v", k, v, after[k])
			}
		}
	}
	// rodar de novo não muda nada (idempotente)
	if _, err := e.mgr.Run(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if again := e.totals(t); again != before {
		t.Fatalf("segunda execução mudou os totais: %+v", again)
	}
}

func (e env) channelTotals(t *testing.T) map[string]stats.Counts {
	t.Helper()
	p := stats.Period{From: time.Date(2020, 1, 1, 0, 0, 0, 0, tz), To: e.st.Today(), TZ: tz}
	got, err := e.st.Channels(context.Background(), e.org, p)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]stats.Counts{}
	for _, c := range got {
		key := ""
		if c.Channel != nil {
			key = c.Channel.Segment
		}
		out[key] = stats.Counts{Clicks: c.Clicks, Unique: c.Unique}
	}
	return out
}

func TestCleanupRunsInBatches(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	day := e.mgr.Today().AddDate(0, 0, -420).Add(12 * time.Hour).UTC()
	exec(t, e.pool, `INSERT INTO click_events(id,occurred_at,org_id,event_type,code,target_type,target_id,visitor_id)
		SELECT gen_random_uuid(), $1::timestamptz + (g || ' seconds')::interval, $2, 'click','aaaaaaa','affiliate_link',$3, decode(repeat('00',32),'hex')
		FROM generate_series(1, 25000) g`, day, e.org, e.link)
	if _, err := e.mgr.AggregatePending(ctx); err != nil {
		t.Fatal(err)
	}
	n, err := e.mgr.Cleanup(ctx)
	if err != nil || n != 25000 || e.events(t) != 0 {
		t.Fatalf("removidos=%d restantes=%d (%v)", n, e.events(t), err)
	}
	if e.count(t, "SELECT sum(clicks) FROM click_daily WHERE org_id=$1", e.org) != 25000 {
		t.Fatal("as contagens consolidadas precisam sobreviver à limpeza")
	}
}
