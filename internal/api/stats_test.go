package api_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/api"
	"github.com/hublinks/hublinks/internal/auth"
	"github.com/hublinks/hublinks/internal/stats"
	"github.com/hublinks/hublinks/internal/store/testdb"
)

type statsEnv struct {
	api          api.Stats
	session      auth.Session
	linkA, linkB uuid.UUID
}

func newStatsEnv(t *testing.T) statsEnv {
	t.Helper()
	pool := testdb.New(t)
	ctx := context.Background()
	org, mp, mpDirect, a, b, ch := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO organizations(id,name,slug,created_at) VALUES($1,'Org',$2,now())", []any{org, org.String()}},
		{"INSERT INTO marketplaces(id,org_id,name,shorten_policy,created_at,updated_at) VALUES($1,$2,'Loja','shorten',now(),now())", []any{mp, org}},
		{"INSERT INTO marketplaces(id,org_id,name,shorten_policy,created_at,updated_at) VALUES($1,$2,'Direta','direct',now(),now())", []any{mpDirect, org}},
		{"INSERT INTO affiliate_links(id,org_id,marketplace_id,title,destination_url,active,created_at,updated_at) VALUES($1,$2,$3,'Fone','https://example.com',true,now(),now())", []any{a, org, mp}},
		{"INSERT INTO affiliate_links(id,org_id,marketplace_id,title,destination_url,active,created_at,updated_at) VALUES($1,$2,$3,'Cabo','https://example.com',true,now(),now())", []any{b, org, mpDirect}},
		{"INSERT INTO short_codes(code,org_id,target_type,target_id,created_at) VALUES('aaaaaaa',$1,'affiliate_link',$2,now()),('bbbbbbb',$1,'affiliate_link',$3,now())", []any{org, a, b}},
		{"INSERT INTO channels(id,org_id,name,segment,created_at,updated_at) VALUES($1,$2,'WhatsApp','wapp',now(),now())", []any{ch, org}},
	} {
		if _, err := pool.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []*uuid.UUID{nil, nil, &ch} {
		if _, err := pool.Exec(ctx, "INSERT INTO click_events(id,occurred_at,org_id,event_type,code,target_type,target_id,channel_id,visitor_id,is_unique_url,is_unique_target) VALUES($1,now(),$2,'click','aaaaaaa','affiliate_link',$3,$4,$5,true,true)", uuid.New(), org, a, c, make([]byte, 32)); err != nil {
			t.Fatal(err)
		}
	}
	// Histórico do link direct: um clique de quando ainda era rastreável.
	if _, err := pool.Exec(ctx, "INSERT INTO click_events(id,occurred_at,org_id,event_type,code,target_type,target_id,visitor_id,is_unique_url,is_unique_target) VALUES($1,now(),$2,'click','bbbbbbb','affiliate_link',$3,$4,true,true)", uuid.New(), org, b, make([]byte, 32)); err != nil {
		t.Fatal(err)
	}
	tz, _ := time.LoadLocation("America/Sao_Paulo")
	return statsEnv{api: api.Stats{Service: stats.Service{Pool: pool, TZ: tz}, BaseURL: "https://hub.example/"}, session: auth.Session{OrgID: org}, linkA: a, linkB: b}
}

func get(t *testing.T, h http.HandlerFunc, s auth.Session, target string, pathID string) (int, map[string]any) {
	t.Helper()
	w := call(t, func(w http.ResponseWriter, r *http.Request) {
		if pathID != "" {
			r.SetPathValue("id", pathID)
		}
		h(w, r)
	}, s, http.MethodGet, target, nil)
	return w.Code, decoded(t, w)
}

func TestStatsLinksList(t *testing.T) {
	e := newStatsEnv(t)
	code, body := get(t, e.api.Links, e.session, "/api/v1/stats/links?period=30d", "")
	if code != 200 || body["total"] != float64(2) || body["page"] != float64(1) {
		t.Fatalf("%d %v", code, body)
	}
	items := body["items"].([]any)
	first := items[0].(map[string]any)
	link := first["link"].(map[string]any)
	if link["code"] != "aaaaaaa" || link["trackable"] != true || link["marketplace"] != "Loja" || first["clicks"] != float64(3) || first["unique"] != float64(3) {
		t.Fatalf("primeiro item = %v", first)
	}
	direct := items[1].(map[string]any)
	if direct["clicks"] != nil || direct["unique"] != nil || direct["link"].(map[string]any)["trackable"] != false || direct["history"].(map[string]any)["clicks"] != float64(1) {
		t.Fatalf("link direct = %v", direct)
	}
}

func TestStatsLinkDetail(t *testing.T) {
	e := newStatsEnv(t)
	code, body := get(t, e.api.Link, e.session, "/api/v1/stats/links/x?period=7d", e.linkA.String())
	if code != 200 {
		t.Fatalf("%d %v", code, body)
	}
	if body["totals"].(map[string]any)["clicks"] != float64(3) || len(body["daily"].([]any)) != 7 {
		t.Fatalf("detalhe = %v", body)
	}
	urls := map[string]float64{}
	for _, c := range body["by_channel"].([]any) {
		m := c.(map[string]any)
		urls[m["url"].(string)] = m["clicks"].(float64)
	}
	if urls["https://hub.example/aaaaaaa"] != 2 || urls["https://hub.example/wapp/aaaaaaa"] != 1 {
		t.Fatalf("by_channel = %v", urls)
	}
	_, body = get(t, e.api.Link, e.session, "/api/v1/stats/links/x", e.linkB.String())
	if body["totals"] != nil || body["link"].(map[string]any)["trackable"] != false || body["history"] == nil {
		t.Fatalf("direct = %v", body)
	}
	if code, _ = get(t, e.api.Link, e.session, "/api/v1/stats/links/x", uuid.NewString()); code != 404 {
		t.Fatalf("inexistente: %d", code)
	}
	if code, _ = get(t, e.api.Link, e.session, "/api/v1/stats/links/x", "nao-e-uuid"); code != 404 {
		t.Fatalf("id inválido: %d", code)
	}
	if code, _ = get(t, e.api.Link, auth.Session{OrgID: uuid.New()}, "/api/v1/stats/links/x", e.linkA.String()); code != 404 {
		t.Fatalf("outra org: %d", code)
	}
}

func TestStatsChannelsAndSummary(t *testing.T) {
	e := newStatsEnv(t)
	code, body := get(t, e.api.Channels, e.session, "/api/v1/stats/channels", "")
	if code != 200 || len(body["items"].([]any)) != 2 {
		t.Fatalf("canais: %d %v", code, body)
	}
	code, body = get(t, e.api.Summary, e.session, "/api/v1/stats/summary?period=7d", "")
	if code != 200 || body["clicks"] != float64(4) || body["active_links"] != float64(2) || len(body["top_links"].([]any)) != 1 || len(body["daily"].([]any)) != 7 {
		t.Fatalf("summary: %d %v", code, body)
	}
}

func TestStatsInvalidPeriod(t *testing.T) {
	e := newStatsEnv(t)
	for _, target := range []string{"/x?period=1y", "/x?from=2026-10-05&to=2026-10-01", "/x?from=2024-01-01&to=2026-10-01"} {
		for _, h := range []http.HandlerFunc{e.api.Summary, e.api.Links, e.api.Channels} {
			code, body := get(t, h, e.session, target, "")
			if code != 422 || body["error"].(map[string]any)["code"] != "validation_failed" {
				t.Fatalf("%s: %d %v", target, code, body)
			}
		}
	}
}
