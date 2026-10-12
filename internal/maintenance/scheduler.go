package maintenance

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Start agenda a manutenção diária em MAINTENANCE_AT (HH:MM, em REPORT_TZ) até
// ctx ser cancelado. Cada instância agenda, mas só uma executa por vez
// (advisory lock). Um atraso é recuperado na execução seguinte, que consolida
// todos os dias pendentes.
func (m Manager) Start(ctx context.Context) error {
	clock, err := time.Parse("15:04", m.Config.MaintenanceAt)
	if err != nil {
		return fmt.Errorf("MAINTENANCE_AT inválido: %w", err)
	}
	go func() {
		for {
			next := m.nextRun(clock, m.now())
			timer := time.NewTimer(time.Until(next))
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
				if _, ran, err := m.RunExclusive(ctx, nil); err == nil && !ran {
					slog.Info("manutenção já está em andamento em outra instância")
				}
			}
		}
	}()
	return nil
}

// nextRun devolve o próximo horário de execução depois de now, em REPORT_TZ.
func (m Manager) nextRun(clock, now time.Time) time.Time {
	local := now.In(m.loc())
	next := time.Date(local.Year(), local.Month(), local.Day(), clock.Hour(), clock.Minute(), 0, 0, m.loc())
	if !next.After(local) {
		next = time.Date(local.Year(), local.Month(), local.Day()+1, clock.Hour(), clock.Minute(), 0, 0, m.loc())
	}
	return next
}
