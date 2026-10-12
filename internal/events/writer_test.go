package events_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/events"
	"github.com/hublinks/hublinks/internal/store/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

func visitor(b byte) []byte {
	v := make([]byte, 32)
	v[0] = b
	return v
}

func ev(org, target uuid.UUID, channel string, v byte, bot bool) events.Event {
	return events.Event{OccurredAt: time.Now().UTC(), OrgID: org.String(), TargetID: target.String(), ChannelID: channel, Code: "abc1234", VisitorID: visitor(v), IsBot: bot}
}

type flags struct{ url, target bool }

func stored(t *testing.T, pool *pgxpool.Pool, target uuid.UUID) []flags {
	t.Helper()
	rows, err := pool.Query(context.Background(), "SELECT is_unique_url,is_unique_target FROM click_events WHERE target_id=$1 ORDER BY occurred_at,id", target)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []flags
	for rows.Next() {
		var f flags
		if err := rows.Scan(&f.url, &f.target); err != nil {
			t.Fatal(err)
		}
		out = append(out, f)
	}
	return out
}

func TestWriterUniqueFlagsWithoutChannel(t *testing.T) {
	pool := testdb.New(t)
	w := events.DBWriter{Pool: pool}
	org, target := uuid.New(), uuid.New()
	if err := w.Write(context.Background(), []events.Event{ev(org, target, "", 1, false), ev(org, target, "", 1, false), ev(org, target, "", 2, false)}); err != nil {
		t.Fatal(err)
	}
	got := stored(t, pool, target)
	want := []flags{{true, true}, {false, false}, {true, true}}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("flags = %v", got)
	}
}

func TestWriterChannelThenShortURL(t *testing.T) {
	pool := testdb.New(t)
	w := events.DBWriter{Pool: pool}
	org, target, wapp := uuid.New(), uuid.New(), uuid.New().String()
	// R5: o mesmo visitante pelo canal e depois pela URL curta.
	if err := w.Write(context.Background(), []events.Event{ev(org, target, wapp, 1, false)}); err != nil {
		t.Fatal(err)
	}
	if err := w.Write(context.Background(), []events.Event{ev(org, target, "", 1, false)}); err != nil {
		t.Fatal(err)
	}
	got := stored(t, pool, target)
	if len(got) != 2 || got[0] != (flags{true, true}) || got[1] != (flags{true, false}) {
		t.Fatalf("flags = %v", got)
	}
	var channelNull int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM click_events WHERE target_id=$1 AND channel_id IS NULL", target).Scan(&channelNull); err != nil || channelNull != 1 {
		t.Fatalf("eventos sem canal: %d (%v)", channelNull, err)
	}
}

func TestWriterConcurrentSameVisitorSingleUnique(t *testing.T) {
	pool := testdb.New(t)
	w := events.DBWriter{Pool: pool}
	org, target := uuid.New(), uuid.New()
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := w.Write(context.Background(), []events.Event{ev(org, target, "", 7, false)}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	var url, tgt, total int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FILTER (WHERE is_unique_url), count(*) FILTER (WHERE is_unique_target), count(*) FROM click_events WHERE target_id=$1", target).Scan(&url, &tgt, &total); err != nil {
		t.Fatal(err)
	}
	if url != 1 || tgt != 1 || total != 20 {
		t.Fatalf("únicos url=%d alvo=%d total=%d", url, tgt, total)
	}
}

func TestWriterBotSkipsVisitorSeen(t *testing.T) {
	pool := testdb.New(t)
	w := events.DBWriter{Pool: pool}
	org, target := uuid.New(), uuid.New()
	if err := w.Write(context.Background(), []events.Event{ev(org, target, "", 1, true)}); err != nil {
		t.Fatal(err)
	}
	got := stored(t, pool, target)
	if len(got) != 1 || got[0] != (flags{false, false}) {
		t.Fatalf("flags = %v", got)
	}
	var seen int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM visitor_seen WHERE target_id=$1", target).Scan(&seen); err != nil || seen != 0 {
		t.Fatalf("visitor_seen: %d (%v)", seen, err)
	}
	var isBot bool
	if err := pool.QueryRow(context.Background(), "SELECT is_bot FROM click_events WHERE target_id=$1", target).Scan(&isBot); err != nil || !isBot {
		t.Fatalf("is_bot = %v (%v)", isBot, err)
	}
}
