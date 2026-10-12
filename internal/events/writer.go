package events

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const nilUUID = "00000000-0000-0000-0000-000000000000"

const insertSeen = `INSERT INTO visitor_seen(target_type,target_id,scope,channel_id,visitor_id,org_id,first_seen_at)
VALUES('affiliate_link',$1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`

type DBWriter struct{ Pool *pgxpool.Pool }

// Write grava o lote em uma transação. A unicidade vem do INSERT … ON CONFLICT
// DO NOTHING em visitor_seen: sob concorrência, só uma transação insere a linha
// (a outra espera o commit e vê o conflito), então há exatamente um "único".
// Robôs são gravados sem tocar em visitor_seen.
func (w DBWriter) Write(ctx context.Context, events []Event) error {
	tx, err := w.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rows := make([][]any, 0, len(events))
	for _, e := range events {
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		channel := e.ChannelID
		var channelCol any
		if channel == "" {
			channel = nilUUID
		} else {
			channelCol = channel
		}
		uniqueURL, uniqueTarget := false, false
		if !e.IsBot {
			tag, err := tx.Exec(ctx, insertSeen, e.TargetID, "url", channel, e.VisitorID, e.OrgID, e.OccurredAt)
			if err != nil {
				return err
			}
			uniqueURL = tag.RowsAffected() == 1
			tag, err = tx.Exec(ctx, insertSeen, e.TargetID, "target", nilUUID, e.VisitorID, e.OrgID, e.OccurredAt)
			if err != nil {
				return err
			}
			uniqueTarget = tag.RowsAffected() == 1
		}
		rows = append(rows, []any{id, e.OccurredAt, e.OrgID, "click", e.Code, "affiliate_link", e.TargetID, channelCol, e.VisitorID, uniqueURL, uniqueTarget, e.IsBot, e.Referer, e.UserAgent})
	}
	_, err = tx.CopyFrom(ctx, pgx.Identifier{"click_events"},
		[]string{"id", "occurred_at", "org_id", "event_type", "code", "target_type", "target_id", "channel_id", "visitor_id", "is_unique_url", "is_unique_target", "is_bot", "referer", "user_agent"},
		pgx.CopyFromRows(rows))
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
