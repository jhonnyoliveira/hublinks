package maintenance_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/domain"
	"github.com/hublinks/hublinks/internal/redirect"
	"github.com/hublinks/hublinks/internal/store"
)

// trash marca o link como excluído há daysAgo dias.
func (e env) trash(t *testing.T, link uuid.UUID, daysAgo int) {
	t.Helper()
	exec(t, e.pool, "UPDATE affiliate_links SET deleted_at=$2 WHERE id=$1", link, fixedNow.AddDate(0, 0, -daysAgo).UTC())
}

func (e env) seen(t *testing.T, link uuid.UUID) int {
	return e.count(t, "SELECT count(*) FROM visitor_seen WHERE target_id=$1", link)
}

func (e env) addSeen(t *testing.T, link uuid.UUID, v byte) {
	t.Helper()
	vis := make([]byte, 32)
	vis[0] = v
	exec(t, e.pool, "INSERT INTO visitor_seen(target_type,target_id,scope,channel_id,visitor_id,org_id,first_seen_at) VALUES('affiliate_link',$1,'target',$2,$3,$4,now()) ON CONFLICT DO NOTHING", link, nilUUID, vis, e.org)
}

func TestExpireTrashPurgesAfterRetentionAndDropsVisitorSeen(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	old := e.link
	recent := newLink(t, e.pool, e.org, "bbbbbbb")
	live := newLink(t, e.pool, e.org, "ccccccc")
	for _, l := range []uuid.UUID{old, recent, live} {
		e.addSeen(t, l, 1)
		e.addSeen(t, l, 2)
	}
	e.trash(t, old, 31)
	e.trash(t, recent, 29)

	purged := 0
	e.mgr.OnPurge = func() { purged++ }
	rep, err := e.mgr.ExpireTrash(ctx)
	if err != nil || rep.Links != 1 {
		t.Fatalf("relatório = %+v (%v)", rep, err)
	}
	if e.count(t, "SELECT count(*) FROM affiliate_links WHERE id=$1 AND purged_at IS NOT NULL", old) != 1 {
		t.Fatal("o link com 31 dias deveria ter purged_at")
	}
	if e.count(t, "SELECT count(*) FROM affiliate_links WHERE purged_at IS NOT NULL") != 1 {
		t.Fatal("somente o link vencido deveria ser purgado")
	}
	if e.seen(t, old) != 0 || e.seen(t, recent) != 2 || e.seen(t, live) != 2 {
		t.Fatalf("visitor_seen: antigo=%d recente=%d ativo=%d", e.seen(t, old), e.seen(t, recent), e.seen(t, live))
	}
	if purged != 1 {
		t.Fatalf("OnPurge chamado %d vezes (esperado 1)", purged)
	}
	// idempotente: nada novo para purgar e o cache não é limpo à toa
	if rep, err = e.mgr.ExpireTrash(ctx); err != nil || rep.Links != 0 || purged != 1 {
		t.Fatalf("segunda execução: %+v (%v) OnPurge=%d", rep, err, purged)
	}
}

func TestExpireTrashPurgesMarketplacesAndChannelsAndFreesSegment(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	ch := e.channel(t, "WhatsApp", "wapp")
	mp := uuid.New()
	exec(t, e.pool, "INSERT INTO marketplaces(id,org_id,name,shorten_policy,created_at,updated_at,deleted_at) VALUES($1,$2,'Velha','shorten',now(),now(),$3)", mp, e.org, fixedNow.AddDate(0, 0, -40).UTC())
	exec(t, e.pool, "UPDATE channels SET deleted_at=$2 WHERE id=$1", ch, fixedNow.AddDate(0, 0, -35).UTC())
	// o segmento fica reservado enquanto o canal está na lixeira
	if _, err := e.pool.Exec(ctx, "INSERT INTO channels(id,org_id,name,segment,created_at,updated_at) VALUES($1,$2,'Outro','wapp',now(),now())", uuid.New(), e.org); err == nil {
		t.Fatal("o segmento deveria estar reservado na lixeira")
	}
	rep, err := e.mgr.ExpireTrash(ctx)
	if err != nil || rep.Marketplaces != 1 || rep.Channels != 1 {
		t.Fatalf("relatório = %+v (%v)", rep, err)
	}
	// depois da exclusão definitiva o segmento fica livre
	if _, err = e.pool.Exec(ctx, "INSERT INTO channels(id,org_id,name,segment,created_at,updated_at) VALUES($1,$2,'Outro','wapp',now(),now())", uuid.New(), e.org); err != nil {
		t.Fatalf("o segmento deveria estar livre após a purga: %v", err)
	}
}

func TestPurgedLinkKeepsHistoryAndCodeIsNeverReused(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.event(t, e.at(3, 10, 0), evOpt{uurl: true, utarget: true})
	e.event(t, e.at(3, 11, 0), evOpt{uurl: false, utarget: false})
	if _, err := e.mgr.Run(ctx, nil); err != nil {
		t.Fatal(err)
	}
	before := e.totals(t)
	e.trash(t, e.link, 45)
	if _, err := e.mgr.ExpireTrash(ctx); err != nil {
		t.Fatal(err)
	}
	// o histórico continua compondo os totais
	if after := e.totals(t); after != before || before.Clicks != 2 {
		t.Fatalf("totais: %+v → %+v", before, after)
	}
	sum, err := e.st.Summary(ctx, e.org, periodAll(e))
	if err != nil || sum.Clicks != 2 {
		t.Fatalf("o dashboard deveria continuar somando o link excluído: %+v (%v)", sum, err)
	}
	// o código responde 404 e a linha de short_codes permanece
	h := &redirect.Handler{Catalog: store.NewCatalog(e.pool), Cache: redirect.NewCache(), BaseURL: "https://hub.example"}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/aaaaaaa", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("link purgado: %d", w.Code)
	}
	if e.count(t, "SELECT count(*) FROM short_codes WHERE code='aaaaaaa'") != 1 {
		t.Fatal("o código curto deve permanecer reservado")
	}
	if _, err = e.pool.Exec(ctx, "INSERT INTO short_codes(code,org_id,target_type,target_id,created_at) VALUES('aaaaaaa',$1,'affiliate_link',$2,now())", e.org, uuid.New()); err == nil {
		t.Fatal("o código não pode ser reutilizado")
	}
}

func TestRestoreBeforeDeadlineKeepsVisitorSeen(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.addSeen(t, e.link, 1)
	e.trash(t, e.link, 10)
	if rep, err := e.mgr.ExpireTrash(ctx); err != nil || rep.Links != 0 {
		t.Fatalf("antes do prazo nada é purgado: %+v (%v)", rep, err)
	}
	if err := store.NewCatalog(e.pool).RestoreLink(ctx, e.org, e.link); err != nil {
		t.Fatal(err)
	}
	if e.seen(t, e.link) != 1 {
		t.Fatal("restaurar antes do prazo mantém visitor_seen: o visitante antigo não vira único novo")
	}
	// depois de restaurado e passado o prazo original, não há o que purgar
	if rep, err := e.mgr.ExpireTrash(ctx); err != nil || rep.Links != 0 {
		t.Fatalf("link restaurado não expira: %+v (%v)", rep, err)
	}
	// um link já purgado não pode mais ser restaurado
	e.trash(t, e.link, 45)
	if _, err := e.mgr.ExpireTrash(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.NewCatalog(e.pool).RestoreLink(ctx, e.org, e.link); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("restaurar um link purgado deveria falhar com não encontrado: %v", err)
	}
}
