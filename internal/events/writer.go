package events

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

const nilUUID = "00000000-0000-0000-0000-000000000000"

type DBWriter struct{ Pool *pgxpool.Pool }

func (w DBWriter) Write(ctx context.Context, events []Event) error {
	tx, err := w.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, e := range events {
		id, _ := uuid.NewV7()
		channel := e.ChannelID
		if channel == "" {
			channel = nilUUID
		}
		uniqueURL, uniqueTarget := false, false
		if !e.IsBot {
			tag, err := tx.Exec(ctx, "INSERT INTO visitor_seen(target_type,target_id,scope,channel_id,visitor_id,org_id,first_seen_at) VALUES('affiliate_link',$1,'url',$2,$3,$4,$5) ON CONFLICT DO NOTHING", e.TargetID, channel, e.VisitorID, e.OrgID, time.Now().UTC())
			if err != nil {
				return err
			}
			uniqueURL = tag.RowsAffected() == 1
			tag, err = tx.Exec(ctx, "INSERT INTO visitor_seen(target_type,target_id,scope,channel_id,visitor_id,org_id,first_seen_at) VALUES('affiliate_link',$1,'target',$2,$3,$4,$5) ON CONFLICT DO NOTHING", e.TargetID, nilUUID, e.VisitorID, e.OrgID, time.Now().UTC())
			if err != nil {
				return err
			}
			uniqueTarget = tag.RowsAffected() == 1
		}
		if _, err = tx.Exec(ctx, "INSERT INTO click_events(id,occurred_at,org_id,event_type,code,target_type,target_id,channel_id,visitor_id,is_unique_url,is_unique_target,is_bot,referer,user_agent) VALUES($1,$2,$3,'click',$4,'affiliate_link',$5,NULLIF($6,'')::uuid,$7,$8,$9,$10,$11,$12)", id, e.OccurredAt, e.OrgID, e.Code, e.TargetID, e.ChannelID, e.VisitorID, uniqueURL, uniqueTarget, e.IsBot, e.Referer, e.UserAgent); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
