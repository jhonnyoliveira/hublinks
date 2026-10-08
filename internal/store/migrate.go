package store

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/hublinks/hublinks/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func sqlDB(pool *pgxpool.Pool) *sql.DB { return stdlib.OpenDB(*pool.Config().ConnConfig) }
func Up(ctx context.Context, pool *pgxpool.Pool) error {
	db := sqlDB(pool)
	defer db.Close()
	goose.SetBaseFS(migrations.Files)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	if err := goose.UpContext(ctx, db, "."); err != nil {
		return fmt.Errorf("aplicar migrações: %w", err)
	}
	return nil
}
func Status(ctx context.Context, pool *pgxpool.Pool) error {
	db := sqlDB(pool)
	defer db.Close()
	goose.SetBaseFS(migrations.Files)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.StatusContext(ctx, db, ".")
}
