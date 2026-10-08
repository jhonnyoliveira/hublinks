package store

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"time"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/domain"
)

func (c *Catalog) CreateMarketplace(ctx context.Context, orgID uuid.UUID, name string, p domain.Policy) (domain.Marketplace, error) {
	fields := map[string]string{}
	if !domain.Text(name, 1, 80) {
		fields["name"] = "nome deve ter de 1 a 80 caracteres"
	}
	if !domain.PolicyValid(p) {
		fields["shorten_policy"] = "política inválida"
	}
	if len(fields) > 0 {
		return domain.Marketplace{}, domain.ValidationError{Fields: fields}
	}
	id, _ := uuid.NewV7()
	now := time.Now().UTC()
	_, err := c.Pool.Exec(ctx, "INSERT INTO marketplaces(id,org_id,name,shorten_policy,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$5)", id, orgID, name, p, now)
	return domain.Marketplace{ID: id, OrgID: orgID, Name: name, ShortenPolicy: p, CreatedAt: now, UpdatedAt: now}, uniqueConflict(err)
}

func (c *Catalog) ListMarketplaces(ctx context.Context, orgID uuid.UUID) ([]domain.Marketplace, error) {
	items, _, err := c.ListMarketplacesWithOptions(ctx, orgID, ListOptions{})
	return items, err
}

func (c *Catalog) ListMarketplacesWithOptions(ctx context.Context, orgID uuid.UUID, o ListOptions) ([]domain.Marketplace, int, error) {
	where := "org_id=$1 AND purged_at IS NULL"
	args := []any{orgID}
	if o.Trash {
		where += " AND deleted_at IS NOT NULL"
	} else {
		where += " AND deleted_at IS NULL"
	}
	if o.Query != "" {
		args = append(args, likePattern(o.Query))
		where += fmt.Sprintf(" AND name ILIKE $%d", len(args))
	}
	total, err := c.count(ctx, "marketplaces", where, args)
	if err != nil {
		return nil, 0, err
	}
	q, args := paged("SELECT id,org_id,name,shorten_policy,created_at,updated_at,deleted_at,purged_at FROM marketplaces WHERE "+where+""+" "+orderBy(o.Sort, map[string]string{"name": "lower(name)", "created": "created_at", "updated": "updated_at"}, "ORDER BY lower(name)", "id"), args, o)
	rows, err := c.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []domain.Marketplace{}
	for rows.Next() {
		var m domain.Marketplace
		if err := rows.Scan(&m.ID, &m.OrgID, &m.Name, &m.ShortenPolicy, &m.CreatedAt, &m.UpdatedAt, &m.DeletedAt, &m.PurgedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, m)
	}
	return items, total, rows.Err()
}

// SoftDeleteMarketplace envia o marketplace à lixeira se ele não tiver links
// fora dela (FR-008c). A linha fica travada durante a verificação, e o
// CreateLink a trava em modo compartilhado, então um link não pode surgir entre
// a checagem e a exclusão.
func (c *Catalog) SoftDeleteMarketplace(ctx context.Context, orgID, id uuid.UUID) error {
	tx, err := c.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var locked uuid.UUID
	err = tx.QueryRow(ctx, "SELECT id FROM marketplaces WHERE id=$1 AND org_id=$2 AND deleted_at IS NULL AND purged_at IS NULL FOR UPDATE", id, orgID).Scan(&locked)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	rows, err := tx.Query(ctx, "SELECT id,title FROM affiliate_links WHERE org_id=$1 AND marketplace_id=$2 AND deleted_at IS NULL AND purged_at IS NULL ORDER BY created_at", orgID, id)
	if err != nil {
		return err
	}
	links := []domain.LinkRef{}
	for rows.Next() {
		var link domain.LinkRef
		if err := rows.Scan(&link.ID, &link.Title); err != nil {
			rows.Close()
			return err
		}
		links = append(links, link)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(links) > 0 {
		return domain.MarketplaceInUseError{Links: links}
	}
	if _, err = tx.Exec(ctx, "UPDATE marketplaces SET deleted_at=$3,updated_at=$3 WHERE id=$1 AND org_id=$2", id, orgID, time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (c *Catalog) RestoreMarketplace(ctx context.Context, orgID, id uuid.UUID) error {
	tag, err := c.Pool.Exec(ctx, "UPDATE marketplaces SET deleted_at=NULL,updated_at=$3 WHERE id=$1 AND org_id=$2 AND deleted_at IS NOT NULL AND purged_at IS NULL", id, orgID, time.Now().UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (c *Catalog) UpdateMarketplace(ctx context.Context, orgID, id uuid.UUID, name *string, policy *domain.Policy) error {
	if name != nil && !domain.Text(*name, 1, 80) {
		return domain.ValidationError{Fields: map[string]string{"name": "nome inválido"}}
	}
	if policy != nil && !domain.PolicyValid(*policy) {
		return domain.ValidationError{Fields: map[string]string{"shorten_policy": "política inválida"}}
	}
	tag, err := c.Pool.Exec(ctx, "UPDATE marketplaces SET name=COALESCE($3,name),shorten_policy=COALESCE($4,shorten_policy),updated_at=$5 WHERE id=$1 AND org_id=$2 AND deleted_at IS NULL AND purged_at IS NULL", id, orgID, name, policy, time.Now().UTC())
	if err != nil {
		return uniqueConflict(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (c *Catalog) GetMarketplace(ctx context.Context, orgID, id uuid.UUID, trash bool) (domain.Marketplace, error) {
	items, _, err := c.ListMarketplacesWithOptions(ctx, orgID, ListOptions{Trash: trash, Query: ""})
	if err != nil {
		return domain.Marketplace{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return domain.Marketplace{}, domain.ErrNotFound
}

func (c *Catalog) CountActiveLinksByMarketplace(ctx context.Context, orgID, marketplaceID uuid.UUID) (int, error) {
	var count int
	err := c.Pool.QueryRow(ctx, "SELECT count(*) FROM affiliate_links WHERE org_id=$1 AND marketplace_id=$2 AND active AND deleted_at IS NULL AND purged_at IS NULL", orgID, marketplaceID).Scan(&count)
	return count, err
}
