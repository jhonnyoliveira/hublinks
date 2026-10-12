package maintenance_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/config"
	"github.com/hublinks/hublinks/internal/maintenance"
	"github.com/hublinks/hublinks/internal/metrics"
	"github.com/hublinks/hublinks/internal/stats"
	"github.com/hublinks/hublinks/internal/store/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

var tz, _ = time.LoadLocation("America/Sao_Paulo")

// hoje nos testes: 12/10/2026 12:00 em Brasília.
var fixedNow = time.Date(2026, 10, 12, 12, 0, 0, 0, tz)

type env struct {
	pool *pgxpool.Pool
	mgr  maintenance.Manager
	st   stats.Service
	org  uuid.UUID
	link uuid.UUID
}

func exec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func newOrg(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	org := uuid.New()
	exec(t, pool, "INSERT INTO organizations(id,name,slug,created_at) VALUES($1,'Org',$2,now())", org, org.String())
	return org
}

func newLink(t *testing.T, pool *pgxpool.Pool, org uuid.UUID, code string) uuid.UUID {
	t.Helper()
	mp, link := uuid.New(), uuid.New()
	exec(t, pool, "INSERT INTO marketplaces(id,org_id,name,shorten_policy,created_at,updated_at) VALUES($1,$2,$3,'shorten',now(),now())", mp, org, "Loja "+code)
	exec(t, pool, "INSERT INTO affiliate_links(id,org_id,marketplace_id,title,destination_url,active,created_at,updated_at) VALUES($1,$2,$3,'Produto','https://example.com/p',true,now(),now())", link, org, mp)
	exec(t, pool, "INSERT INTO short_codes(code,org_id,target_type,target_id,created_at) VALUES($1,$2,'affiliate_link',$3,now())", code, org, link)
	return link
}

func newEnv(t *testing.T) env {
	t.Helper()
	pool := testdb.New(t)
	cfg := config.Config{ReportTZ: tz, EventsRetentionDays: 395, BotEventsRetentionDays: 30, TrashRetentionDays: 30}
	now := func() time.Time { return fixedNow }
	org := newOrg(t, pool)
	return env{
		pool: pool,
		mgr:  maintenance.Manager{Pool: pool, Config: cfg, Now: now},
		st:   stats.Service{Pool: pool, TZ: tz, Now: now},
		org:  org,
		link: newLink(t, pool, org, "aaaaaaa"),
	}
}

type evOpt struct {
	channel *uuid.UUID
	uurl    bool
	utarget bool
	bot     bool
	org     uuid.UUID
	target  uuid.UUID
}

// event grava um clique em at (RFC 3339). Sem org/target usa os do ambiente.
func (e env) event(t *testing.T, at string, o evOpt) {
	t.Helper()
	when, err := time.Parse(time.RFC3339, at)
	if err != nil {
		t.Fatal(err)
	}
	org, target := e.org, e.link
	if o.org != uuid.Nil {
		org = o.org
	}
	if o.target != uuid.Nil {
		target = o.target
	}
	exec(t, e.pool, "INSERT INTO click_events(id,occurred_at,org_id,event_type,code,target_type,target_id,channel_id,visitor_id,is_unique_url,is_unique_target,is_bot) VALUES($1,$2,$3,'click','aaaaaaa','affiliate_link',$4,$5,$6,$7,$8,$9)",
		uuid.New(), when, org, target, o.channel, make([]byte, 32), o.uurl, o.utarget, o.bot)
}

func (e env) channel(t *testing.T, name, segment string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	exec(t, e.pool, "INSERT INTO channels(id,org_id,name,segment,created_at,updated_at) VALUES($1,$2,$3,$4,now(),now())", id, e.org, name, segment)
	return id
}

func (e env) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := e.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return n
}

type dailyRow struct {
	Day, Channel          string
	Clicks, UURL, UTarget int
}

func (e env) daily(t *testing.T, org uuid.UUID) map[dailyRow]bool {
	t.Helper()
	rows, err := e.pool.Query(context.Background(), "SELECT day::text, channel_id::text, clicks, unique_url, unique_target FROM click_daily WHERE org_id=$1", org)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[dailyRow]bool{}
	for rows.Next() {
		var r dailyRow
		if err := rows.Scan(&r.Day, &r.Channel, &r.Clicks, &r.UURL, &r.UTarget); err != nil {
			t.Fatal(err)
		}
		out[r] = true
	}
	return out
}

const nilUUID = "00000000-0000-0000-0000-000000000000"

// totals soma cliques e únicos de todo o histórico (de 2020 até hoje).
func (e env) totals(t *testing.T) stats.Counts {
	t.Helper()
	p := stats.Period{From: time.Date(2020, 1, 1, 0, 0, 0, 0, tz), To: e.st.Today(), TZ: tz}
	got, err := e.st.Totals(context.Background(), e.org, p, []uuid.UUID{e.link})
	if err != nil {
		t.Fatal(err)
	}
	return got[e.link]
}

func periodAll(e env) stats.Period {
	return stats.Period{From: time.Date(2020, 1, 1, 0, 0, 0, 0, tz), To: e.st.Today(), TZ: tz}
}

func metricsFor(m maintenance.Manager) maintenance.Manager {
	m.Metrics = metrics.New()
	return m
}

func gaugeValue(t *testing.T, g prometheus.Gauge) float64 {
	t.Helper()
	return testutil.ToFloat64(g)
}

func counterValue(t *testing.T, c prometheus.Counter) float64 {
	t.Helper()
	return testutil.ToFloat64(c)
}
