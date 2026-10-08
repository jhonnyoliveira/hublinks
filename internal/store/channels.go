package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (c *Catalog) Channel(ctx context.Context, orgID uuid.UUID, segment string) (uuid.UUID, error) {
	var id uuid.UUID
	err := c.Pool.QueryRow(ctx, "SELECT id FROM channels WHERE org_id=$1 AND segment=$2 AND deleted_at IS NULL AND purged_at IS NULL", orgID, segment).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return id, domain.ErrNotFound
	}
	return id, err
}

func (c *Catalog) CreateChannel(ctx context.Context, orgID uuid.UUID, name, segment string) (domain.Channel, error) {
	fields := map[string]string{}
	if !domain.Text(name, 1, 60) {
		fields["name"] = "nome deve ter de 1 a 60 caracteres"
	}
	if !domain.Segment(segment) {
		fields["segment"] = "segmento inválido ou reservado"
	}
	if len(fields) > 0 {
		return domain.Channel{}, domain.ValidationError{Fields: fields}
	}
	id, _ := uuid.NewV7()
	now := time.Now().UTC()
	_, err := c.Pool.Exec(ctx, "INSERT INTO channels(id,org_id,name,segment,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$5)", id, orgID, name, segment, now)
	return domain.Channel{ID: id, OrgID: orgID, Name: name, Segment: segment, CreatedAt: now, UpdatedAt: now}, uniqueConflict(err)
}

func (c *Catalog) ListChannels(ctx context.Context, orgID uuid.UUID) ([]domain.Channel, error) {
	items, _, err := c.ListChannelsWithOptions(ctx, orgID, ListOptions{})
	return items, err
}

func (c *Catalog) ListChannelsWithOptions(ctx context.Context, orgID uuid.UUID, o ListOptions) ([]domain.Channel, int, error) {
	where := "org_id=$1 AND purged_at IS NULL"
	args := []any{orgID}
	if o.Trash {
		where += " AND deleted_at IS NOT NULL"
	} else {
		where += " AND deleted_at IS NULL"
	}
	if o.Query != "" {
		args = append(args, likePattern(o.Query))
		where += fmt.Sprintf(" AND (name ILIKE $%d OR segment ILIKE $%d)", len(args), len(args))
	}
	total, err := c.count(ctx, "channels", where, args)
	if err != nil {
		return nil, 0, err
	}
	q, args := paged("SELECT id,org_id,name,segment,created_at,updated_at,deleted_at,purged_at FROM channels WHERE "+where+""+" "+orderBy(o.Sort, map[string]string{"name": "lower(name)", "segment": "segment", "created": "created_at", "updated": "updated_at"}, "ORDER BY lower(name)", "id"), args, o)
	rows, err := c.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []domain.Channel{}
	for rows.Next() {
		var v domain.Channel
		if err := rows.Scan(&v.ID, &v.OrgID, &v.Name, &v.Segment, &v.CreatedAt, &v.UpdatedAt, &v.DeletedAt, &v.PurgedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, v)
	}
	return items, total, rows.Err()
}

func (c *Catalog) SoftDeleteChannel(ctx context.Context, orgID, id uuid.UUID) error {
	tag, err := c.Pool.Exec(ctx, "UPDATE channels SET deleted_at=$3,updated_at=$3 WHERE id=$1 AND org_id=$2 AND deleted_at IS NULL AND purged_at IS NULL", id, orgID, time.Now().UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (c *Catalog) RestoreChannel(ctx context.Context, orgID, id uuid.UUID) error {
	tag, err := c.Pool.Exec(ctx, "UPDATE channels SET deleted_at=NULL,updated_at=$3 WHERE id=$1 AND org_id=$2 AND deleted_at IS NOT NULL AND purged_at IS NULL", id, orgID, time.Now().UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (c *Catalog) UpdateChannel(ctx context.Context, orgID, id uuid.UUID, name, segment *string) error {
	if name != nil && !domain.Text(*name, 1, 60) {
		return domain.ValidationError{Fields: map[string]string{"name": "nome inválido"}}
	}
	if segment != nil && !domain.Segment(*segment) {
		return domain.ValidationError{Fields: map[string]string{"segment": "segmento inválido"}}
	}
	tag, err := c.Pool.Exec(ctx, "UPDATE channels SET name=COALESCE($3,name),segment=COALESCE($4,segment),updated_at=$5 WHERE id=$1 AND org_id=$2 AND deleted_at IS NULL AND purged_at IS NULL", id, orgID, name, segment, time.Now().UTC())
	if err != nil {
		return uniqueConflict(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (c *Catalog) GetChannel(ctx context.Context, orgID, id uuid.UUID, trash bool) (domain.Channel, int, error) {
	var v domain.Channel
	q := "SELECT id,org_id,name,segment,created_at,updated_at,deleted_at,purged_at FROM channels WHERE id=$1 AND org_id=$2 AND purged_at IS NULL"
	if trash {
		q += " AND deleted_at IS NOT NULL"
	} else {
		q += " AND deleted_at IS NULL"
	}
	err := c.Pool.QueryRow(ctx, q, id, orgID).Scan(&v.ID, &v.OrgID, &v.Name, &v.Segment, &v.CreatedAt, &v.UpdatedAt, &v.DeletedAt, &v.PurgedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, 0, domain.ErrNotFound
	}
	if err != nil {
		return v, 0, err
	}
	count, err := c.CountActiveLinksByChannel(ctx, orgID)
	return v, count, err
}

func (c *Catalog) CountActiveLinksByChannel(ctx context.Context, orgID uuid.UUID) (int, error) {
	var count int
	err := c.Pool.QueryRow(ctx, "SELECT count(*) FROM affiliate_links WHERE org_id=$1 AND active AND deleted_at IS NULL AND purged_at IS NULL", orgID).Scan(&count)
	return count, err
}
