package maintenance_test

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestFirstAggregationPinsTheTimezone(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if err := e.mgr.CheckTimezone(ctx); err != nil {
		t.Fatalf("sem consolidação anterior não há o que conferir: %v", err)
	}
	if e.count(t, "SELECT count(*) FROM app_settings WHERE key='report_timezone'") != 0 {
		t.Fatal("conferir na partida não deve fixar o fuso")
	}
	e.event(t, "2026-10-09T15:00:00Z", evOpt{uurl: true, utarget: true})
	if _, err := e.mgr.AggregatePending(ctx); err != nil {
		t.Fatal(err)
	}
	var tzName string
	if err := e.pool.QueryRow(ctx, "SELECT value FROM app_settings WHERE key='report_timezone'").Scan(&tzName); err != nil || tzName != "America/Sao_Paulo" {
		t.Fatalf("fuso gravado = %q (%v)", tzName, err)
	}
	if err := e.mgr.CheckTimezone(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestStartingWithAnotherTimezoneAborts(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.event(t, "2026-10-09T15:00:00Z", evOpt{uurl: true, utarget: true})
	if _, err := e.mgr.AggregatePending(ctx); err != nil {
		t.Fatal(err)
	}
	utc := e.mgr
	utc.Config.ReportTZ = time.UTC
	err := utc.CheckTimezone(ctx)
	if err == nil || !strings.Contains(err.Error(), "REPORT_TZ=UTC") || !strings.Contains(err.Error(), "America/Sao_Paulo") {
		t.Fatalf("o erro deveria citar os dois fusos: %v", err)
	}
	// nenhuma operação de manutenção roda com o fuso errado, nem muda o gravado
	if _, err = utc.Run(ctx, nil); err == nil {
		t.Fatal("Run com outro fuso deveria falhar")
	}
	e.event(t, "2026-10-08T15:00:00Z", evOpt{uurl: true, utarget: true})
	if _, err = utc.AggregatePending(ctx); err == nil {
		t.Fatal("AggregatePending com outro fuso deveria falhar")
	}
	if _, err = utc.AggregateDay(ctx, time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("AggregateDay com outro fuso deveria falhar")
	}
	var tzName string
	_ = e.pool.QueryRow(ctx, "SELECT value FROM app_settings WHERE key='report_timezone'").Scan(&tzName)
	if tzName != "America/Sao_Paulo" {
		t.Fatalf("o fuso gravado não pode mudar: %q", tzName)
	}
	if e.count(t, "SELECT count(*) FROM aggregated_days WHERE day='2026-10-08'") != 0 {
		t.Fatal("a tentativa com outro fuso não pode consolidar nada")
	}
}

func TestParseDayRequiresAPastDay(t *testing.T) {
	e := newEnv(t)
	if d, err := e.mgr.ParseDay("2026-10-09"); err != nil || d.Location().String() != "America/Sao_Paulo" || d.Hour() != 0 {
		t.Fatalf("%v %v", d, err)
	}
	for _, bad := range []string{"", "ontem", "2026-13-01", "2026-10-12", "2026-10-13", "2027-01-01"} {
		if _, err := e.mgr.ParseDay(bad); err == nil {
			t.Errorf("%q deveria ser recusado", bad)
		}
	}
}

func TestRunExclusiveSkipsWhenAnotherMaintenanceHoldsTheLock(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	conn, err := e.pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	var locked bool
	if err = conn.QueryRow(ctx, "SELECT pg_try_advisory_lock(845801)").Scan(&locked); err != nil || !locked {
		t.Fatalf("não consegui o lock: %v %v", locked, err)
	}
	e.event(t, "2026-10-09T15:00:00Z", evOpt{uurl: true, utarget: true})
	_, ran, err := e.mgr.RunExclusive(ctx, nil)
	if err != nil || ran {
		t.Fatalf("com o lock ocupado não deveria rodar: ran=%v err=%v", ran, err)
	}
	if e.count(t, "SELECT count(*) FROM aggregated_days") != 0 {
		t.Fatal("nada deveria ter sido consolidado")
	}
	_, _ = conn.Exec(ctx, "SELECT pg_advisory_unlock(845801)")
	rep, ran, err := e.mgr.RunExclusive(ctx, nil)
	if err != nil || !ran || rep.AggregatedDays != 1 {
		t.Fatalf("liberado o lock, deveria rodar: %+v ran=%v err=%v", rep, ran, err)
	}
}

func TestRunExclusiveRecordsMetrics(t *testing.T) {
	e := newEnv(t)
	m := metricsFor(e.mgr)
	if _, ran, err := m.RunExclusive(context.Background(), nil); err != nil || !ran {
		t.Fatal(err)
	}
	if v := gaugeValue(t, m.Metrics.MaintenanceLastSuccess); v == 0 {
		t.Fatal("deveria registrar o instante do último sucesso")
	}
	// fuso divergente = falha contabilizada
	e.event(t, "2026-10-09T15:00:00Z", evOpt{uurl: true, utarget: true})
	if _, err := m.AggregatePending(context.Background()); err != nil {
		t.Fatal(err)
	}
	m.Config.ReportTZ = time.UTC
	if _, _, err := m.RunExclusive(context.Background(), nil); err == nil {
		t.Fatal("deveria falhar")
	}
	if v := counterValue(t, m.Metrics.MaintenanceFailures); v != 1 {
		t.Fatalf("falhas = %v", v)
	}
}
