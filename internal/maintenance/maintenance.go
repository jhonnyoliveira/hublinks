package maintenance

import (
	"context"
	"fmt"
	"github.com/hublinks/hublinks/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type Manager struct {
	Pool   *pgxpool.Pool
	Config config.Config
}

func (m Manager) CheckTimezone(ctx context.Context) error {
	var value string
	err := m.Pool.QueryRow(ctx, "SELECT value FROM app_settings WHERE key='report_timezone'").Scan(&value)
	if err != nil {
		_, e := m.Pool.Exec(ctx, "INSERT INTO app_settings(key,value) VALUES('report_timezone',$1) ON CONFLICT DO NOTHING", m.Config.ReportTZ.String())
		return e
	}
	if value != m.Config.ReportTZ.String() {
		return fmt.Errorf("REPORT_TZ difere do fuso já usado nas agregações")
	}
	return nil
}
func (m Manager) Aggregate(ctx context.Context, day time.Time) error {
	day = day.In(m.Config.ReportTZ)
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, m.Config.ReportTZ)
	end := start.AddDate(0, 0, 1)
	tx, err := m.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, "SELECT id FROM organizations")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var org string
		if err = rows.Scan(&org); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "DELETE FROM click_daily WHERE org_id=$1 AND day=$2", org, start.Format("2006-01-02")); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO click_daily(org_id,day,event_type,target_type,target_id,channel_id,clicks,unique_url,unique_target) SELECT org_id,$1,'click','affiliate_link',target_id,COALESCE(channel_id,'00000000-0000-0000-0000-000000000000'),count(*),sum(is_unique_url::int),sum(is_unique_target::int) FROM click_events WHERE org_id=$2 AND occurred_at >= $3 AND occurred_at < $4 AND NOT is_bot GROUP BY org_id,target_id,COALESCE(channel_id,'00000000-0000-0000-0000-000000000000')`, start.Format("2006-01-02"), org, start.UTC(), end.UTC())
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "INSERT INTO aggregated_days(org_id,day,aggregated_at) VALUES($1,$2,$3) ON CONFLICT (org_id,day) DO UPDATE SET aggregated_at=EXCLUDED.aggregated_at", org, start.Format("2006-01-02"), time.Now().UTC()); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func (m Manager) Cleanup(ctx context.Context) error {
	today := time.Now().In(m.Config.ReportTZ)
	realCut := today.AddDate(0, 0, -m.Config.EventsRetentionDays)
	botCut := today.AddDate(0, 0, -m.Config.BotEventsRetentionDays)
	_, err := m.Pool.Exec(ctx, `DELETE FROM click_events e WHERE EXISTS (SELECT 1 FROM aggregated_days a WHERE a.org_id=e.org_id AND a.day=(e.occurred_at AT TIME ZONE $1)::date) AND ((NOT e.is_bot AND (e.occurred_at AT TIME ZONE $1)::date < $2) OR (e.is_bot AND (e.occurred_at AT TIME ZONE $1)::date < $3))`, m.Config.ReportTZ.String(), realCut.Format("2006-01-02"), botCut.Format("2006-01-02"))
	return err
}
func (m Manager) ExpireTrash(ctx context.Context) error {
	cut := time.Now().UTC().AddDate(0, 0, -m.Config.TrashRetentionDays)
	tx, err := m.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, "UPDATE affiliate_links SET purged_at=now() WHERE deleted_at < $1 AND purged_at IS NULL RETURNING id", cut)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "DELETE FROM visitor_seen WHERE target_type='affiliate_link' AND target_id=$1", id); err != nil {
			return err
		}
	}
	rows.Close()
	if _, err = tx.Exec(ctx, "UPDATE marketplaces SET purged_at=now() WHERE deleted_at < $1 AND purged_at IS NULL", cut); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE channels SET purged_at=now() WHERE deleted_at < $1 AND purged_at IS NULL", cut); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (m Manager) Run(ctx context.Context, day *time.Time) error {
	if err := m.CheckTimezone(ctx); err != nil {
		return err
	}
	if day != nil {
		if err := m.Aggregate(ctx, *day); err != nil {
			return err
		}
	} else {
		if err := m.Aggregate(ctx, time.Now().In(m.Config.ReportTZ).AddDate(0, 0, -1)); err != nil {
			return err
		}
	}
	if err := m.Cleanup(ctx); err != nil {
		return err
	}
	return m.ExpireTrash(ctx)
}
