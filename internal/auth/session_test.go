package auth_test

import (
	"context"
	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/auth"
	"github.com/hublinks/hublinks/internal/store/testdb"
	"testing"
	"time"
)

func TestSessionLifecycle(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	user, _ := uuid.NewV7()
	org, _ := uuid.NewV7()
	now := time.Now().UTC()
	_, err := p.Exec(ctx, "INSERT INTO organizations(id,name,slug,created_at) VALUES($1,'Org','org',$2)", org, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Exec(ctx, "INSERT INTO users(id,email,password_hash,created_at) VALUES($1,'admin@example.test','hash',$2)", user, now)
	if err != nil {
		t.Fatal(err)
	}
	m := auth.Manager{Pool: p}
	raw, s, err := m.Create(ctx, user, org)
	if err != nil {
		t.Fatal(err)
	}
	if raw == "" || s.CSRF == "" {
		t.Fatal("sessão sem tokens")
	}
	got, err := m.Lookup(ctx, raw)
	if err != nil || got.UserID != user || got.OrgID != org {
		t.Fatalf("lookup: %#v, %v", got, err)
	}
	if err := m.Destroy(ctx, raw); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Lookup(ctx, raw); err == nil {
		t.Fatal("sessão removida ainda encontrada")
	}
}
