package maintenance

import "context"

// cleanupBatch limita cada DELETE para não prender o banco em remoções grandes.
const cleanupBatch = 10000

// Cleanup remove eventos além da retenção: reais com dia local anterior a
// hoje − EVENTS_RETENTION_DAYS e de robôs anterior a hoje − BOT_EVENTS_RETENTION_DAYS,
// mas somente de dias já presentes em aggregated_days (as contagens ficam em
// click_daily). O corte é por dia civil em REPORT_TZ: dia local < corte equivale a
// occurred_at < meia-noite local do corte.
func (m Manager) Cleanup(ctx context.Context) (int64, error) {
	today := m.Today()
	realCut := today.AddDate(0, 0, -m.Config.EventsRetentionDays).UTC()
	botCut := today.AddDate(0, 0, -m.Config.BotEventsRetentionDays).UTC()
	var total int64
	for {
		tag, err := m.Pool.Exec(ctx, `
			DELETE FROM click_events WHERE ctid = ANY(ARRAY(
				SELECT e.ctid FROM click_events e
				WHERE ((NOT e.is_bot AND e.occurred_at < $2) OR (e.is_bot AND e.occurred_at < $3))
				  AND EXISTS (SELECT 1 FROM aggregated_days a WHERE a.org_id=e.org_id AND a.day=(e.occurred_at AT TIME ZONE $1::text)::date)
				LIMIT $4))`, m.loc().String(), realCut, botCut, cleanupBatch)
		if err != nil {
			return total, err
		}
		total += tag.RowsAffected()
		if tag.RowsAffected() < cleanupBatch {
			return total, nil
		}
	}
}
