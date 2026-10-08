package store_test

import (
	"context"
	"github.com/hublinks/hublinks/internal/store"
	"github.com/hublinks/hublinks/internal/store/testdb"
	"testing"
)

func TestBootstrapCreatesInitialOrgAndAdmin(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	if err := store.Bootstrap(ctx, p, "Org teste", "ADMIN@EXAMPLE.TEST", "senha-inicial-segura"); err != nil {
		t.Fatal(err)
	}
	var users, orgs, members int
	if err := p.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&users); err != nil {
		t.Fatal(err)
	}
	if err := p.QueryRow(ctx, "SELECT count(*) FROM organizations").Scan(&orgs); err != nil {
		t.Fatal(err)
	}
	if err := p.QueryRow(ctx, "SELECT count(*) FROM memberships WHERE role='admin'").Scan(&members); err != nil {
		t.Fatal(err)
	}
	if users != 1 || orgs != 1 || members != 1 {
		t.Fatalf("bootstrap incorreto: users=%d orgs=%d members=%d", users, orgs, members)
	}
	if err := store.Bootstrap(ctx, p, "Outra", "outro@example.test", "outra-senha-segura"); err != nil {
		t.Fatal(err)
	}
	if err := p.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&users); err != nil || users != 1 {
		t.Fatalf("bootstrap recriou admin: %d, %v", users, err)
	}
}
