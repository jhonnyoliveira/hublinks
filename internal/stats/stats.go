package stats

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type Totals struct {
	Clicks, Unique int
	Trackable      bool
}
type Service struct {
	Pool *pgxpool.Pool
	TZ   *time.Location
}

func (s Service) LinkTotals(ctx context.Context, orgID, linkID uuid.UUID, from, to time.Time) (Totals, error) {
	var t Totals
	t.Trackable = true
	err := s.Pool.QueryRow(ctx, `SELECT COALESCE(sum(clicks),0),COALESCE(sum(unique_target),0) FROM (SELECT clicks,unique_target FROM click_daily WHERE org_id=$1 AND target_id=$2 AND day >= $3::date AND day < $4::date UNION ALL SELECT 1,is_unique_target::int FROM click_events WHERE org_id=$1 AND target_id=$2 AND occurred_at >= $3 AND occurred_at < $4 AND NOT is_bot) x`, orgID, linkID, from.UTC(), to.UTC()).Scan(&t.Clicks, &t.Unique)
	return t, err
}
func (s Service) Summary(ctx context.Context, orgID uuid.UUID, from, to time.Time) (Totals, error) {
	var t Totals
	t.Trackable = true
	err := s.Pool.QueryRow(ctx, `SELECT COALESCE(sum(clicks),0),COALESCE(sum(unique_target),0) FROM (SELECT clicks,unique_target FROM click_daily WHERE org_id=$1 AND day >= $2::date AND day < $3::date UNION ALL SELECT 1,is_unique_target::int FROM click_events WHERE org_id=$1 AND occurred_at >= $2 AND occurred_at < $3 AND NOT is_bot)x`, orgID, from.UTC(), to.UTC()).Scan(&t.Clicks, &t.Unique)
	return t, err
}
