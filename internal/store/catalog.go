package store

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type Catalog struct{ Pool *pgxpool.Pool }

func NewCatalog(p *pgxpool.Pool) *Catalog { return &Catalog{p} }
func (c *Catalog) Resolve(ctx context.Context, code string) (domain.AffiliateLink, error) {
	const q = `SELECT l.id,l.org_id,l.marketplace_id,l.title,l.image_url,l.destination_url,l.shorten_policy_override,l.active,l.created_at,l.updated_at,l.deleted_at,l.purged_at,m.id,m.org_id,m.name,m.shorten_policy,m.created_at,m.updated_at,m.deleted_at,m.purged_at FROM short_codes s JOIN affiliate_links l ON l.id=s.target_id JOIN marketplaces m ON m.id=l.marketplace_id WHERE s.code=$1`
	var l domain.AffiliateLink
	var override *string
	var mp domain.Marketplace
	err := c.Pool.QueryRow(ctx, q, code).Scan(&l.ID, &l.OrgID, &l.MarketplaceID, &l.Title, &l.ImageURL, &l.DestinationURL, &override, &l.Active, &l.CreatedAt, &l.UpdatedAt, &l.DeletedAt, &l.PurgedAt, &mp.ID, &mp.OrgID, &mp.Name, &mp.ShortenPolicy, &mp.CreatedAt, &mp.UpdatedAt, &mp.DeletedAt, &mp.PurgedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return l, domain.ErrNotFound
	}
	if err != nil {
		return l, err
	}
	if override != nil {
		x := domain.Policy(*override)
		l.ShortenPolicyOverride = &x
	}
	l.Code = code
	l.Marketplace = mp
	if !l.Active || l.DeletedAt != nil || l.PurgedAt != nil || mp.DeletedAt != nil || mp.PurgedAt != nil {
		return l, domain.ErrNotFound
	}
	return l, nil
}
func (c *Catalog) Channel(ctx context.Context, orgID uuid.UUID, segment string) (uuid.UUID, error) {
	var id uuid.UUID
	err := c.Pool.QueryRow(ctx, "SELECT id FROM channels WHERE org_id=$1 AND segment=$2 AND deleted_at IS NULL AND purged_at IS NULL", orgID, segment).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return id, domain.ErrNotFound
	}
	return id, err
}
func (c *Catalog) CreateMarketplace(ctx context.Context, orgID uuid.UUID, name string, p domain.Policy) (domain.Marketplace, error) {
	if !domain.Text(name, 1, 80) || !domain.PolicyValid(p) {
		return domain.Marketplace{}, domain.ValidationError{Fields: map[string]string{"name": "dados inválidos"}}
	}
	id, _ := uuid.NewV7()
	now := time.Now().UTC()
	_, err := c.Pool.Exec(ctx, "INSERT INTO marketplaces(id,org_id,name,shorten_policy,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$5)", id, orgID, name, p, now)
	return domain.Marketplace{ID: id, OrgID: orgID, Name: name, ShortenPolicy: p, CreatedAt: now, UpdatedAt: now}, err
}
func (c *Catalog) CreateChannel(ctx context.Context, orgID uuid.UUID, name, segment string) (domain.Channel, error) {
	if !domain.Text(name, 1, 60) || !domain.Segment(segment) {
		return domain.Channel{}, domain.ValidationError{Fields: map[string]string{"segment": "segmento inválido"}}
	}
	id, _ := uuid.NewV7()
	now := time.Now().UTC()
	_, err := c.Pool.Exec(ctx, "INSERT INTO channels(id,org_id,name,segment,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$5)", id, orgID, name, segment, now)
	return domain.Channel{ID: id, OrgID: orgID, Name: name, Segment: segment, CreatedAt: now, UpdatedAt: now}, err
}
func (c *Catalog) CreateLink(ctx context.Context, l domain.AffiliateLink) (domain.AffiliateLink, error) {
	if !domain.Text(l.Title, 1, 200) || !domain.URL(l.DestinationURL) {
		return l, domain.ValidationError{Fields: map[string]string{"destination_url": "URL deve usar http ou https"}}
	}
	tx, err := c.Pool.Begin(ctx)
	if err != nil {
		return l, err
	}
	defer tx.Rollback(ctx)
	var mp domain.Marketplace
	if err = tx.QueryRow(ctx, "SELECT id,org_id,name,shorten_policy,created_at,updated_at,deleted_at,purged_at FROM marketplaces WHERE id=$1 AND org_id=$2 AND deleted_at IS NULL AND purged_at IS NULL", l.MarketplaceID, l.OrgID).Scan(&mp.ID, &mp.OrgID, &mp.Name, &mp.ShortenPolicy, &mp.CreatedAt, &mp.UpdatedAt, &mp.DeletedAt, &mp.PurgedAt); err != nil {
		return l, domain.ErrNotFound
	}
	l.ID, _ = uuid.NewV7()
	now := time.Now().UTC()
	l.CreatedAt, l.UpdatedAt, l.Marketplace = now, now, mp
	for n := 0; n < 5; n++ {
		l.Code, err = domain.NewCode()
		if err != nil {
			return l, err
		}
		_, err = tx.Exec(ctx, "INSERT INTO affiliate_links(id,org_id,marketplace_id,title,image_url,destination_url,shorten_policy_override,active,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$9)", l.ID, l.OrgID, l.MarketplaceID, l.Title, l.ImageURL, l.DestinationURL, l.ShortenPolicyOverride, l.Active, now)
		if err != nil {
			return l, err
		}
		_, err = tx.Exec(ctx, "INSERT INTO short_codes(code,org_id,target_type,target_id,created_at) VALUES($1,$2,'affiliate_link',$3,$4)", l.Code, l.OrgID, l.ID, now)
		if err == nil {
			break
		}
		return l, err
	}
	return l, tx.Commit(ctx)
}

func (c *Catalog) ListMarketplaces(ctx context.Context, orgID uuid.UUID) ([]domain.Marketplace, error) {
	rows, err := c.Pool.Query(ctx, `SELECT id,org_id,name,shorten_policy,created_at,updated_at,deleted_at,purged_at FROM marketplaces WHERE org_id=$1 AND deleted_at IS NULL AND purged_at IS NULL ORDER BY lower(name)`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.Marketplace{}
	for rows.Next() {
		var m domain.Marketplace
		if err := rows.Scan(&m.ID, &m.OrgID, &m.Name, &m.ShortenPolicy, &m.CreatedAt, &m.UpdatedAt, &m.DeletedAt, &m.PurgedAt); err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	return items, rows.Err()
}
func (c *Catalog) ListChannels(ctx context.Context, orgID uuid.UUID) ([]domain.Channel, error) {
	rows, err := c.Pool.Query(ctx, `SELECT id,org_id,name,segment,created_at,updated_at,deleted_at,purged_at FROM channels WHERE org_id=$1 AND deleted_at IS NULL AND purged_at IS NULL ORDER BY lower(name)`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.Channel{}
	for rows.Next() {
		var v domain.Channel
		if err := rows.Scan(&v.ID, &v.OrgID, &v.Name, &v.Segment, &v.CreatedAt, &v.UpdatedAt, &v.DeletedAt, &v.PurgedAt); err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return items, rows.Err()
}

func (c *Catalog) ListLinks(ctx context.Context, orgID uuid.UUID) ([]domain.AffiliateLink, error) {
	rows, err := c.Pool.Query(ctx, `SELECT l.id,l.org_id,l.marketplace_id,l.title,l.image_url,l.destination_url,l.shorten_policy_override,l.active,l.created_at,l.updated_at,l.deleted_at,l.purged_at,s.code,m.id,m.org_id,m.name,m.shorten_policy,m.created_at,m.updated_at,m.deleted_at,m.purged_at FROM affiliate_links l JOIN marketplaces m ON m.id=l.marketplace_id JOIN short_codes s ON s.target_id=l.id AND s.target_type='affiliate_link' WHERE l.org_id=$1 AND l.deleted_at IS NULL AND l.purged_at IS NULL ORDER BY l.created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.AffiliateLink{}
	for rows.Next() {
		var l domain.AffiliateLink
		var mp domain.Marketplace
		var override *string
		if err := rows.Scan(&l.ID, &l.OrgID, &l.MarketplaceID, &l.Title, &l.ImageURL, &l.DestinationURL, &override, &l.Active, &l.CreatedAt, &l.UpdatedAt, &l.DeletedAt, &l.PurgedAt, &l.Code, &mp.ID, &mp.OrgID, &mp.Name, &mp.ShortenPolicy, &mp.CreatedAt, &mp.UpdatedAt, &mp.DeletedAt, &mp.PurgedAt); err != nil {
			return nil, err
		}
		if override != nil {
			v := domain.Policy(*override)
			l.ShortenPolicyOverride = &v
		}
		l.Marketplace = mp
		items = append(items, l)
	}
	return items, rows.Err()
}

func (c *Catalog) SoftDeleteMarketplace(ctx context.Context, orgID, id uuid.UUID) error {
	var links int
	if err := c.Pool.QueryRow(ctx, "SELECT count(*) FROM affiliate_links WHERE org_id=$1 AND marketplace_id=$2 AND deleted_at IS NULL AND purged_at IS NULL", orgID, id).Scan(&links); err != nil {
		return err
	}
	if links > 0 {
		return domain.ErrConflict
	}
	tag, err := c.Pool.Exec(ctx, "UPDATE marketplaces SET deleted_at=$3,updated_at=$3 WHERE id=$1 AND org_id=$2 AND deleted_at IS NULL AND purged_at IS NULL", id, orgID, time.Now().UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
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
func (c *Catalog) RestoreLink(ctx context.Context, orgID, id uuid.UUID) error {
	tag, err := c.Pool.Exec(ctx, `UPDATE affiliate_links l SET deleted_at=NULL,updated_at=$3 FROM marketplaces m WHERE l.id=$1 AND l.org_id=$2 AND l.marketplace_id=m.id AND m.org_id=$2 AND m.deleted_at IS NULL AND m.purged_at IS NULL AND l.deleted_at IS NOT NULL AND l.purged_at IS NULL`, id, orgID, time.Now().UTC())
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
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
