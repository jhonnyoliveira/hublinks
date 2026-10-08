// Package testdb creates isolated PostgreSQL databases for integration tests.
package testdb

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

func New(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL não definida; teste de integração requer PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatalf("TEST_DATABASE_URL inválida: %v", err)
	}
	dbName := "hl_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	admin, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Skipf("PostgreSQL indisponível para integração: %v", err)
	}
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+dbName); err != nil {
		admin.Close()
		t.Skipf("não foi possível criar banco temporário: %v", err)
	}
	cfg.ConnConfig.Database = dbName
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		_, _ = admin.Exec(ctx, "DROP DATABASE "+dbName)
		admin.Close()
		t.Fatal(err)
	}
	if err = store.Up(ctx, pool); err != nil {
		pool.Close()
		_, _ = admin.Exec(ctx, "DROP DATABASE "+dbName)
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		drop, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = admin.Exec(drop, fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", dbName))
		admin.Close()
	})
	return pool
}
