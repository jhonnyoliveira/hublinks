package maintenance

import (
	"context"
	"fmt"
	"time"
)

func (m Manager) Start(ctx context.Context) error {
	clock, err := time.Parse("15:04", m.Config.MaintenanceAt)
	if err != nil {
		return fmt.Errorf("MAINTENANCE_AT inválido: %w", err)
	}
	go func() {
		for {
			now := time.Now().In(m.Config.ReportTZ)
			next := time.Date(now.Year(), now.Month(), now.Day(), clock.Hour(), clock.Minute(), 0, 0, m.Config.ReportTZ)
			if !next.After(now) {
				next = next.AddDate(0, 0, 1)
			}
			timer := time.NewTimer(time.Until(next))
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
				m.runLocked(ctx)
			}
		}
	}()
	return nil
}
func (m Manager) runLocked(ctx context.Context) {
	conn, err := m.Pool.Acquire(ctx)
	if err != nil {
		return
	}
	defer conn.Release()
	var locked bool
	if err = conn.QueryRow(ctx, "SELECT pg_try_advisory_lock(845801)").Scan(&locked); err != nil || !locked {
		return
	}
	defer conn.Exec(context.Background(), "SELECT pg_advisory_unlock(845801)")
	_ = m.Run(ctx, nil)
}
