package maintenance

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// TrashReport conta o que virou exclusão definitiva.
type TrashReport struct{ Links, Marketplaces, Channels int }

// ExpireTrash torna definitiva (purged_at) a exclusão de itens na lixeira há mais
// de TRASH_RETENTION_DAYS. Para links, apaga também os registros de unicidade
// (visitor_seen). Os códigos curtos permanecem (nunca são reutilizados) e
// click_daily continua compondo os totais.
func (m Manager) ExpireTrash(ctx context.Context) (TrashReport, error) {
	var rep TrashReport
	cut := m.now().UTC().AddDate(0, 0, -m.Config.TrashRetentionDays)
	now := m.now().UTC()
	tx, err := m.Pool.Begin(ctx)
	if err != nil {
		return rep, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, "UPDATE affiliate_links SET purged_at=$2 WHERE deleted_at < $1 AND purged_at IS NULL RETURNING id", cut, now)
	if err != nil {
		return rep, err
	}
	// A conexão da transação não aceita outro comando enquanto o cursor está
	// aberto: lê todos os ids antes de apagar visitor_seen.
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return rep, err
	}
	rep.Links = len(ids)
	if len(ids) > 0 {
		if _, err = tx.Exec(ctx, "DELETE FROM visitor_seen WHERE target_type='affiliate_link' AND target_id = ANY($1)", ids); err != nil {
			return rep, err
		}
	}
	tag, err := tx.Exec(ctx, "UPDATE marketplaces SET purged_at=$2 WHERE deleted_at < $1 AND purged_at IS NULL", cut, now)
	if err != nil {
		return rep, err
	}
	rep.Marketplaces = int(tag.RowsAffected())
	if tag, err = tx.Exec(ctx, "UPDATE channels SET purged_at=$2 WHERE deleted_at < $1 AND purged_at IS NULL", cut, now); err != nil {
		return rep, err
	}
	rep.Channels = int(tag.RowsAffected())
	if err = tx.Commit(ctx); err != nil {
		return rep, err
	}
	if rep.Links+rep.Marketplaces+rep.Channels > 0 && m.OnPurge != nil {
		m.OnPurge()
	}
	return rep, nil
}
