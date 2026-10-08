package store_test

import (
	"context"
	"github.com/hublinks/hublinks/internal/store"
	"github.com/hublinks/hublinks/internal/store/testdb"
	"testing"
)

func TestMigrationsAreIdempotent(t *testing.T) {
	p := testdb.New(t)
	if err := store.Up(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"organizations", "users", "memberships", "sessions", "app_settings"} {
		var exists bool
		if err := p.QueryRow(context.Background(), "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name=$1)", table).Scan(&exists); err != nil || !exists {
			t.Fatalf("tabela %s ausente: %v", table, err)
		}
	}
}
