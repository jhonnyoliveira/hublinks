package store

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("configurar banco: %w", err)
	}
	cfg.ConnConfig.RuntimeParams["TimeZone"] = "UTC"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("conectar banco: %w", err)
	}
	check, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := pool.Ping(check); err != nil {
		pool.Close()
		return nil, fmt.Errorf("verificar banco: %w", err)
	}
	return pool, nil
}
