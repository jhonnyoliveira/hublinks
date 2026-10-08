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

func (c *Catalog) CreateLink(ctx context.Context, l domain.AffiliateLink) (domain.AffiliateLink, error) {
	fields := map[string]string{}
	if !domain.Text(l.Title, 1, 200) {
		fields["title"] = "título deve ter de 1 a 200 caracteres"
	}
	if !domain.URL(l.DestinationURL) {
		fields["destination_url"] = "URL deve usar http ou https"
	}
	if l.ImageURL != nil && !domain.URL(*l.ImageURL) {
		fields["image_url"] = "URL deve usar http ou https"
	}
	if l.ShortenPolicyOverride != nil && !domain.PolicyValid(*l.ShortenPolicyOverride) {
		fields["shorten_policy_override"] = "política inválida"
	}
	if len(fields) > 0 {
		return l, domain.ValidationError{Fields: fields}
	}
	tx, err := c.Pool.Begin(ctx)
	if err != nil {
		return l, err
	}
	defer tx.Rollback(ctx)
	var mp domain.Marketplace
	if err = tx.QueryRow(ctx, "SELECT id,org_id,name,shorten_policy,created_at,updated_at,deleted_at,purged_at FROM marketplaces WHERE id=$1 AND org_id=$2 AND deleted_at IS NULL AND purged_at IS NULL FOR SHARE", l.MarketplaceID, l.OrgID).Scan(&mp.ID, &mp.OrgID, &mp.Name, &mp.ShortenPolicy, &mp.CreatedAt, &mp.UpdatedAt, &mp.DeletedAt, &mp.PurgedAt); err != nil {
		return l, domain.ErrNotFound
	}
	l.ID, _ = uuid.NewV7()
	now := time.Now().UTC()
	l.CreatedAt, l.UpdatedAt, l.Marketplace = now, now, mp
	if _, err = tx.Exec(ctx, "INSERT INTO affiliate_links(id,org_id,marketplace_id,title,image_url,destination_url,shorten_policy_override,active,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$9)", l.ID, l.OrgID, l.MarketplaceID, l.Title, l.ImageURL, l.DestinationURL, l.ShortenPolicyOverride, l.Active, now); err != nil {
		return l, err
	}
	l.Code, err = c.insertShortCode(ctx, tx, l, now)
	if err != nil {
		return l, err
	}
	return l, tx.Commit(ctx)
}

func (c *Catalog) ListLinks(ctx context.Context, orgID uuid.UUID) ([]domain.AffiliateLink, error) {
	items, _, err := c.ListLinksWithOptions(ctx, orgID, ListOptions{})
	return items, err
}

func (c *Catalog) ListLinksWithOptions(ctx context.Context, orgID uuid.UUID, o ListOptions) ([]domain.AffiliateLink, int, error) {
	where := "l.org_id=$1 AND l.purged_at IS NULL"
	args := []any{orgID}
	if o.Trash {
		where += " AND l.deleted_at IS NOT NULL"
	} else {
		where += " AND l.deleted_at IS NULL"
	}
	if o.Query != "" {
		args = append(args, likePattern(o.Query))
		where += fmt.Sprintf(" AND l.title ILIKE $%d", len(args))
	}
	if o.MarketplaceID != uuid.Nil {
		args = append(args, o.MarketplaceID)
		where += fmt.Sprintf(" AND l.marketplace_id=$%d", len(args))
	}
	if o.Active != nil {
		args = append(args, *o.Active)
		where += fmt.Sprintf(" AND l.active=$%d", len(args))
	}
	total, err := c.count(ctx, "affiliate_links l", where, args)
	if err != nil {
		return nil, 0, err
	}
	q, args := paged("SELECT "+linkColumns+linkFrom+where+" "+orderBy(o.Sort, map[string]string{"title": "lower(l.title)", "created": "l.created_at", "updated": "l.updated_at"}, "ORDER BY l.created_at DESC", "l.id"), args, o)
	rows, err := c.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []domain.AffiliateLink{}
	for rows.Next() {
		l, err := scanLink(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, l)
	}
	return items, total, rows.Err()
}

// linkColumns e linkFrom são a projeção comum de listagem e detalhe de links
// (link, código curto e marketplace).
const linkColumns = "l.id,l.org_id,l.marketplace_id,l.title,l.image_url,l.destination_url,l.shorten_policy_override,l.active,l.created_at,l.updated_at,l.deleted_at,l.purged_at,s.code,m.id,m.org_id,m.name,m.shorten_policy,m.created_at,m.updated_at,m.deleted_at,m.purged_at"
const linkFrom = " FROM affiliate_links l JOIN marketplaces m ON m.id=l.marketplace_id JOIN short_codes s ON s.target_id=l.id AND s.target_type='affiliate_link' WHERE "

func scanLink(row pgx.Row) (domain.AffiliateLink, error) {
	var l domain.AffiliateLink
	var mp domain.Marketplace
	var override *string
	if err := row.Scan(&l.ID, &l.OrgID, &l.MarketplaceID, &l.Title, &l.ImageURL, &l.DestinationURL, &override, &l.Active, &l.CreatedAt, &l.UpdatedAt, &l.DeletedAt, &l.PurgedAt, &l.Code, &mp.ID, &mp.OrgID, &mp.Name, &mp.ShortenPolicy, &mp.CreatedAt, &mp.UpdatedAt, &mp.DeletedAt, &mp.PurgedAt); err != nil {
		return l, err
	}
	if override != nil {
		v := domain.Policy(*override)
		l.ShortenPolicyOverride = &v
	}
	l.Marketplace = mp
	return l, nil
}

func (c *Catalog) SoftDeleteLink(ctx context.Context, orgID, id uuid.UUID) error {
	tag, err := c.Pool.Exec(ctx, "UPDATE affiliate_links SET deleted_at=$3,updated_at=$3 WHERE id=$1 AND org_id=$2 AND deleted_at IS NULL AND purged_at IS NULL", id, orgID, time.Now().UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (c *Catalog) RestoreLink(ctx context.Context, orgID, id uuid.UUID) error {
	tag, err := c.Pool.Exec(ctx, `UPDATE affiliate_links l SET deleted_at=NULL,updated_at=$3 FROM marketplaces m WHERE l.id=$1 AND l.org_id=$2 AND l.marketplace_id=m.id AND m.org_id=$2 AND m.deleted_at IS NULL AND m.purged_at IS NULL AND l.deleted_at IS NOT NULL AND l.purged_at IS NULL`, id, orgID, time.Now().UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var marketplaceID uuid.UUID
		err := c.Pool.QueryRow(ctx, "SELECT marketplace_id FROM affiliate_links WHERE id=$1 AND org_id=$2 AND deleted_at IS NOT NULL AND purged_at IS NULL", id, orgID).Scan(&marketplaceID)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return err
		}
		var active bool
		err = c.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM marketplaces WHERE id=$1 AND org_id=$2 AND deleted_at IS NULL AND purged_at IS NULL)", marketplaceID, orgID).Scan(&active)
		if err != nil {
			return err
		}
		if !active {
			return domain.ErrConflict
		}
		return domain.ErrNotFound
	}
	return nil
}

// UpdateLink changes only supplied fields and returns the code so callers can
// invalidate the public resolution cache after the transaction succeeds.
func (c *Catalog) UpdateLink(ctx context.Context, orgID, id uuid.UUID, title, destinationURL, imageURL *string, marketplaceID *uuid.UUID, policy *domain.Policy, active *bool) (string, error) {
	if title != nil && !domain.Text(*title, 1, 200) {
		return "", domain.ValidationError{Fields: map[string]string{"title": "título deve ter de 1 a 200 caracteres"}}
	}
	if destinationURL != nil && !domain.URL(*destinationURL) {
		return "", domain.ValidationError{Fields: map[string]string{"destination_url": "URL deve usar http ou https"}}
	}
	if imageURL != nil && *imageURL != "" && !domain.URL(*imageURL) {
		return "", domain.ValidationError{Fields: map[string]string{"image_url": "URL deve usar http ou https"}}
	}
	// policy vazia ("") remove a política própria e volta a valer a do marketplace.
	if policy != nil && *policy != "" && !domain.PolicyValid(*policy) {
		return "", domain.ValidationError{Fields: map[string]string{"shorten_policy_override": "política inválida"}}
	}
	if marketplaceID != nil {
		var exists bool
		err := c.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM marketplaces WHERE id=$1 AND org_id=$2 AND deleted_at IS NULL AND purged_at IS NULL)", *marketplaceID, orgID).Scan(&exists)
		if err != nil {
			return "", err
		}
		if !exists {
			return "", domain.ErrNotFound
		}
	}
	var code string
	err := c.Pool.QueryRow(ctx, `UPDATE affiliate_links l SET
		title=COALESCE($3,l.title), destination_url=COALESCE($4,l.destination_url),
		image_url=CASE WHEN $5::text IS NULL THEN l.image_url ELSE NULLIF($5::text,'') END,
		marketplace_id=COALESCE($6,l.marketplace_id), shorten_policy_override=CASE WHEN $7::text IS NULL THEN l.shorten_policy_override ELSE NULLIF($7::text,'') END,
		active=COALESCE($8,l.active), updated_at=$9
		FROM short_codes s WHERE l.id=$1 AND l.org_id=$2 AND l.deleted_at IS NULL AND l.purged_at IS NULL
		AND s.target_id=l.id AND s.target_type='affiliate_link' RETURNING s.code`, id, orgID, title, destinationURL, imageURL, marketplaceID, policy, active, time.Now().UTC()).Scan(&code)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	return code, err
}

func (c *Catalog) GetLink(ctx context.Context, orgID, id uuid.UUID, trash bool) (domain.AffiliateLink, error) {
	where := "l.id=$1 AND l.org_id=$2 AND l.purged_at IS NULL AND l.deleted_at IS NULL"
	if trash {
		where = "l.id=$1 AND l.org_id=$2 AND l.purged_at IS NULL AND l.deleted_at IS NOT NULL"
	}
	l, err := scanLink(c.Pool.QueryRow(ctx, "SELECT "+linkColumns+linkFrom+where, id, orgID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AffiliateLink{}, domain.ErrNotFound
	}
	return l, err
}
