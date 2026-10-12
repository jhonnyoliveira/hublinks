package maintenance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrDayCleaned: reconsolidar um dia cujos eventos podem já ter sido removidos
// pela retenção apagaria as contagens (click_daily é refeito a partir dos eventos).
var ErrDayCleaned = errors.New("dia fora da retenção de eventos: reconsolidar apagaria o histórico")

type orgDay struct {
	org uuid.UUID
	day time.Time
}

// AggregatePending consolida todos os dias passados (anteriores a hoje em
// REPORT_TZ) que têm eventos e ainda não constam em aggregated_days. Dias só com
// eventos de robôs também são marcados, para que a limpeza possa removê-los.
func (m Manager) AggregatePending(ctx context.Context) (int, error) {
	todayStart := m.Today().UTC()
	rows, err := m.Pool.Query(ctx, `
		SELECT DISTINCT e.org_id, (e.occurred_at AT TIME ZONE $1::text)::date AS day
		FROM click_events e
		WHERE e.occurred_at < $2
		  AND NOT EXISTS (SELECT 1 FROM aggregated_days a WHERE a.org_id=e.org_id AND a.day=(e.occurred_at AT TIME ZONE $1::text)::date)
		ORDER BY day, e.org_id`, m.loc().String(), todayStart)
	if err != nil {
		return 0, err
	}
	pending, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (orgDay, error) {
		var od orgDay
		var day time.Time
		err := r.Scan(&od.org, &day)
		od.day = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, m.loc())
		return od, err
	})
	if err != nil {
		return 0, err
	}
	for _, od := range pending {
		if err = m.aggregateOrgDay(ctx, od.org, od.day, false); err != nil {
			return 0, fmt.Errorf("consolidando %s: %w", od.day.Format(dateLayout), err)
		}
	}
	return len(pending), nil
}

// AggregateDay reconsolida um dia para todas as organizações. É idempotente:
// repetir produz os mesmos valores. Recusa dias já fora da retenção de eventos.
func (m Manager) AggregateDay(ctx context.Context, day time.Time) (int, error) {
	day = m.civil(day)
	if !day.Before(m.Today()) {
		return 0, fmt.Errorf("o dia %s ainda não terminou: só dias anteriores a hoje podem ser consolidados", day.Format(dateLayout))
	}
	rows, err := m.Pool.Query(ctx, "SELECT id FROM organizations ORDER BY created_at, id")
	if err != nil {
		return 0, err
	}
	orgs, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return 0, err
	}
	for _, org := range orgs {
		if err = m.aggregateOrgDay(ctx, org, day, true); err != nil {
			return 0, err
		}
	}
	return len(orgs), nil
}

// aggregateOrgDay refaz, em uma transação, as linhas de click_daily de (org, dia)
// a partir de click_events (sem robôs) e registra o dia em aggregated_days.
// reaggregate=true indica reconsolidação explícita de um dia possivelmente já limpo.
func (m Manager) aggregateOrgDay(ctx context.Context, org uuid.UUID, day time.Time, reaggregate bool) error {
	start := m.civil(day)
	end := start.AddDate(0, 0, 1)
	dayStr := start.Format(dateLayout)
	tx, err := m.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = m.pinTimezone(ctx, tx); err != nil {
		return err
	}
	if reaggregate {
		var done bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM aggregated_days WHERE org_id=$1 AND day=$2::date)", org, dayStr).Scan(&done); err != nil {
			return err
		}
		if done && m.Config.EventsRetentionDays > 0 && start.Before(m.Today().AddDate(0, 0, -m.Config.EventsRetentionDays)) {
			return fmt.Errorf("%w (%s)", ErrDayCleaned, dayStr)
		}
	}
	if _, err = tx.Exec(ctx, "DELETE FROM click_daily WHERE org_id=$1 AND day=$2::date", org, dayStr); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO click_daily(org_id,day,event_type,target_type,target_id,channel_id,clicks,unique_url,unique_target)
		SELECT org_id, $2::date, event_type, target_type, target_id,
		       COALESCE(channel_id,'`+nilUUID+`'::uuid),
		       count(*), count(*) FILTER (WHERE is_unique_url), count(*) FILTER (WHERE is_unique_target)
		FROM click_events
		WHERE org_id=$1 AND NOT is_bot AND occurred_at >= $3 AND occurred_at < $4
		GROUP BY org_id, event_type, target_type, target_id, COALESCE(channel_id,'`+nilUUID+`'::uuid)`,
		org, dayStr, start.UTC(), end.UTC())
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO aggregated_days(org_id,day,aggregated_at) VALUES($1,$2::date,$3) ON CONFLICT (org_id,day) DO UPDATE SET aggregated_at=EXCLUDED.aggregated_at", org, dayStr, m.now().UTC()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

const nilUUID = "00000000-0000-0000-0000-000000000000"
