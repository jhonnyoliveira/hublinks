// Package maintenance consolida os cliques por dia, remove eventos antigos de
// dias já consolidados e expira a lixeira. Tudo é idempotente e restrito ao
// fuso de relatório (REPORT_TZ).
package maintenance

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/hublinks/hublinks/internal/config"
	"github.com/hublinks/hublinks/internal/metrics"
	"github.com/jackc/pgx/v5/pgxpool"
)

const dateLayout = "2006-01-02"

// lockID identifica o advisory lock que impede duas manutenções simultâneas
// (o agendador de cada instância e o comando manual).
const lockID = 845801

type Manager struct {
	Pool   *pgxpool.Pool
	Config config.Config
	// Metrics e OnPurge são opcionais. OnPurge limpa o cache de resolução do
	// processo depois de uma exclusão definitiva.
	Metrics *metrics.Metrics
	OnPurge func()
	// Now permite fixar o relógio nos testes (nil = time.Now).
	Now func() time.Time
}

func (m Manager) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

func (m Manager) loc() *time.Location {
	if m.Config.ReportTZ == nil {
		return time.UTC
	}
	return m.Config.ReportTZ
}

// civil devolve a meia-noite de t no fuso de relatório.
func (m Manager) civil(t time.Time) time.Time {
	t = t.In(m.loc())
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, m.loc())
}

// Today é o dia corrente (meia-noite) em REPORT_TZ.
func (m Manager) Today() time.Time { return m.civil(m.now()) }

// ParseDay interpreta YYYY-MM-DD em REPORT_TZ; só aceita dias já encerrados.
func (m Manager) ParseDay(s string) (time.Time, error) {
	d, err := time.ParseInLocation(dateLayout, s, m.loc())
	if err != nil {
		return time.Time{}, fmt.Errorf("dia inválido %q: use YYYY-MM-DD", s)
	}
	if !d.Before(m.Today()) {
		return time.Time{}, fmt.Errorf("o dia %s ainda não terminou em %s: só dias anteriores a hoje podem ser consolidados", s, m.loc())
	}
	return d, nil
}

// Report resume uma execução.
type Report struct {
	AggregatedDays int   // pares (organização, dia) consolidados
	DeletedEvents  int64 // eventos removidos pela retenção
	Purged         TrashReport
}

// Run consolida, limpa e expira a lixeira uma vez. Sem day, consolida todos os
// dias passados ainda não consolidados (recupera atrasos); com day, reconsolida
// somente aquele dia.
func (m Manager) Run(ctx context.Context, day *time.Time) (Report, error) {
	var r Report
	if err := m.CheckTimezone(ctx); err != nil {
		return r, err
	}
	var err error
	if day != nil {
		r.AggregatedDays, err = m.AggregateDay(ctx, *day)
	} else {
		r.AggregatedDays, err = m.AggregatePending(ctx)
	}
	if err != nil {
		return r, err
	}
	if r.DeletedEvents, err = m.Cleanup(ctx); err != nil {
		return r, err
	}
	r.Purged, err = m.ExpireTrash(ctx)
	return r, err
}

// RunExclusive é Run protegido por advisory lock: se outra manutenção estiver em
// andamento, não faz nada e devolve ran=false. Registra duração e falhas.
func (m Manager) RunExclusive(ctx context.Context, day *time.Time) (r Report, ran bool, err error) {
	conn, err := m.Pool.Acquire(ctx)
	if err != nil {
		return r, false, err
	}
	defer conn.Release()
	var locked bool
	if err = conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", lockID).Scan(&locked); err != nil || !locked {
		return r, false, err
	}
	defer func() { _, _ = conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", lockID) }()
	start := time.Now()
	r, err = m.Run(ctx, day)
	if m.Metrics != nil {
		m.Metrics.MaintenanceDuration.Set(time.Since(start).Seconds())
		if err != nil {
			m.Metrics.MaintenanceFailures.Inc()
		} else {
			m.Metrics.MaintenanceLastSuccess.SetToCurrentTime()
		}
	}
	if err != nil {
		slog.Error("manutenção falhou", "error", err)
	} else {
		slog.Info("manutenção concluída", "days_aggregated", r.AggregatedDays, "events_deleted", r.DeletedEvents,
			"links_purged", r.Purged.Links, "marketplaces_purged", r.Purged.Marketplaces, "channels_purged", r.Purged.Channels,
			"duration", time.Since(start).String())
	}
	return r, true, err
}
