package store

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
	"time"
)

// bootstrapLockKey identifica o advisory lock do bootstrap (valor arbitrário e fixo).
const bootstrapLockKey int64 = 0x68756c6e6b73

func Bootstrap(ctx context.Context, pool *pgxpool.Pool, orgName, email, password string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Serializa réplicas que sobem juntas numa instalação nova: sem isso, todas veem
	// "sem organização e sem usuários" e só uma consegue criar (as outras falham
	// com violação de unicidade). O lock vale até o fim da transação.
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", bootstrapLockKey); err != nil {
		return err
	}
	now := time.Now().UTC()
	var orgID uuid.UUID
	err = tx.QueryRow(ctx, "SELECT id FROM organizations ORDER BY created_at LIMIT 1").Scan(&orgID)
	if err != nil {
		orgID, _ = uuid.NewV7()
		slug := "default"
		if _, err = tx.Exec(ctx, "INSERT INTO organizations (id,name,slug,created_at) VALUES ($1,$2,$3,$4)", orgID, orgName, slug, now); err != nil {
			return err
		}
	}
	var count int
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		if strings.TrimSpace(email) == "" || password == "" {
			return fmt.Errorf("ADMIN_EMAIL e ADMIN_PASSWORD são obrigatórias na primeira execução")
		}
		if len(password) < 12 {
			return fmt.Errorf("ADMIN_PASSWORD deve ter ao menos 12 caracteres")
		}
		id, _ := uuid.NewV7()
		hash, err := auth.Hash(password)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "INSERT INTO users (id,email,password_hash,created_at) VALUES ($1,$2,$3,$4)", id, strings.ToLower(strings.TrimSpace(email)), hash, now); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "INSERT INTO memberships (user_id,org_id,role) VALUES ($1,$2,'admin')", id, orgID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
