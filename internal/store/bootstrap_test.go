package store_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/hublinks/hublinks/internal/auth"
	"github.com/hublinks/hublinks/internal/store"
	"github.com/hublinks/hublinks/internal/store/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

type counts struct{ users, orgs, admins int }

func count(t *testing.T, p *pgxpool.Pool) counts {
	t.Helper()
	var c counts
	ctx := context.Background()
	for q, dst := range map[string]*int{
		"SELECT count(*) FROM users":                          &c.users,
		"SELECT count(*) FROM organizations":                  &c.orgs,
		"SELECT count(*) FROM memberships WHERE role='admin'": &c.admins,
	} {
		if err := p.QueryRow(ctx, q).Scan(dst); err != nil {
			t.Fatal(err)
		}
	}
	return c
}

func TestBootstrapCreatesInitialOrgAndAdmin(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	if err := store.Bootstrap(ctx, p, "Org teste", "  ADMIN@EXAMPLE.TEST ", "senha-inicial-segura"); err != nil {
		t.Fatal(err)
	}
	if c := count(t, p); c != (counts{1, 1, 1}) {
		t.Fatalf("bootstrap incorreto: %+v", c)
	}
	var orgName, email, hash, role string
	if err := p.QueryRow(ctx, "SELECT o.name, u.email, u.password_hash, m.role FROM users u JOIN memberships m ON m.user_id=u.id JOIN organizations o ON o.id=m.org_id").Scan(&orgName, &email, &hash, &role); err != nil {
		t.Fatal(err)
	}
	if orgName != "Org teste" || email != "admin@example.test" || role != "admin" {
		t.Fatalf("org=%q email=%q role=%q", orgName, email, role)
	}
	if strings.Contains(hash, "senha-inicial-segura") || !strings.HasPrefix(hash, "$argon2id$") || !auth.Verify("senha-inicial-segura", hash) {
		t.Fatalf("a senha deve ser guardada como hash argon2id verificável: %q", hash)
	}
}

func TestBootstrapDoesNothingWhenUsersAlreadyExist(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	if err := store.Bootstrap(ctx, p, "Org teste", "admin@example.test", "senha-inicial-segura"); err != nil {
		t.Fatal(err)
	}
	var hashBefore string
	_ = p.QueryRow(ctx, "SELECT password_hash FROM users").Scan(&hashBefore)

	// reinício com outros valores (ou sem as variáveis ADMIN_*) não altera nada nem falha
	for _, args := range [][3]string{{"Outra org", "outro@example.test", "outra-senha-segura"}, {"Org teste", "", ""}, {"Org teste", "admin@example.test", "curta"}} {
		if err := store.Bootstrap(ctx, p, args[0], args[1], args[2]); err != nil {
			t.Fatalf("com usuários existentes o bootstrap deve ser neutro (%v): %v", args, err)
		}
	}
	if c := count(t, p); c != (counts{1, 1, 1}) {
		t.Fatalf("bootstrap recriou dados: %+v", c)
	}
	var orgName, hashAfter string
	_ = p.QueryRow(ctx, "SELECT name FROM organizations").Scan(&orgName)
	_ = p.QueryRow(ctx, "SELECT password_hash FROM users").Scan(&hashAfter)
	if orgName != "Org teste" || hashAfter != hashBefore {
		t.Fatalf("dados existentes foram alterados: org=%q", orgName)
	}
}

func TestBootstrapRejectsShortPasswordAndCreatesNothing(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	err := store.Bootstrap(ctx, p, "Org", "admin@example.test", "onze-chars!") // 11 caracteres
	if err == nil || !strings.Contains(err.Error(), "ADMIN_PASSWORD") || !strings.Contains(err.Error(), "12") {
		t.Fatalf("senha curta deveria ser rejeitada citando a variável: %v", err)
	}
	if c := count(t, p); c != (counts{}) {
		t.Fatalf("a falha deve desfazer tudo (inclusive a organização): %+v", c)
	}
	if err := store.Bootstrap(ctx, p, "Org", "admin@example.test", "doze-chars!!"); err != nil { // 12 caracteres: aceita
		t.Fatalf("12 caracteres deve bastar: %v", err)
	}
}

func TestBootstrapRequiresCredentialsOnFirstRun(t *testing.T) {
	p := testdb.New(t)
	for _, c := range [][2]string{{"", "senha-inicial-segura"}, {"   ", "senha-inicial-segura"}, {"a@example.test", ""}} {
		err := store.Bootstrap(context.Background(), p, "Org", c[0], c[1])
		if err == nil || !strings.Contains(err.Error(), "ADMIN_EMAIL") {
			t.Fatalf("%v: %v", c, err)
		}
	}
	if c := count(t, p); c != (counts{}) {
		t.Fatalf("%+v", c)
	}
}

func TestBootstrapReusesExistingOrganization(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	if _, err := p.Exec(ctx, "INSERT INTO organizations(id,name,slug,created_at) VALUES(gen_random_uuid(),'Existente','existente',now())"); err != nil {
		t.Fatal(err)
	}
	if err := store.Bootstrap(ctx, p, "Nome novo", "admin@example.test", "senha-inicial-segura"); err != nil {
		t.Fatal(err)
	}
	if c := count(t, p); c != (counts{1, 1, 1}) {
		t.Fatalf("não deve criar outra organização: %+v", c)
	}
	var name string
	_ = p.QueryRow(ctx, "SELECT name FROM organizations").Scan(&name)
	if name != "Existente" {
		t.Fatalf("a organização existente não pode ser renomeada: %q", name)
	}
}

// Duas réplicas subindo juntas numa instalação nova: exatamente um administrador
// e uma organização, sem que nenhuma das duas falhe.
func TestBootstrapIsSafeWhenReplicasStartTogether(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make([]error, 6)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = store.Bootstrap(ctx, p, "Org", "admin@example.test", "senha-inicial-segura")
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("réplica %d falhou: %v", i, err)
		}
	}
	if c := count(t, p); c != (counts{1, 1, 1}) {
		t.Fatalf("esperado 1 org, 1 usuário e 1 admin: %+v", c)
	}
}
