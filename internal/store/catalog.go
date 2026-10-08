package store

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type Catalog struct {
	Pool    *pgxpool.Pool
	newCode func() (string, error)
}

// ListOptions define filtros comuns às listagens administrativas. Page starts at
// one; callers that do not need pagination may leave it zero.
type ListOptions struct {
	Query         string
	Trash         bool
	MarketplaceID uuid.UUID
	Active        *bool
	Page, PerPage int
}

func NewCatalog(p *pgxpool.Pool) *Catalog { return &Catalog{Pool: p, newCode: domain.NewCode} }

// NewCatalogWithCodeGenerator is used by integration tests to make a database
// collision deterministic. Application code must use NewCatalog.
func NewCatalogWithCodeGenerator(p *pgxpool.Pool, generator func() (string, error)) *Catalog {
	return &Catalog{Pool: p, newCode: generator}
}
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
	if l.ImageURL != nil && !domain.URL(*l.ImageURL) {
		return l, domain.ValidationError{Fields: map[string]string{"image_url": "URL deve usar http ou https"}}
	}
	if l.ShortenPolicyOverride != nil && !domain.PolicyValid(*l.ShortenPolicyOverride) {
		return l, domain.ValidationError{Fields: map[string]string{"shorten_policy_override": "política inválida"}}
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
	if _, err = tx.Exec(ctx, "INSERT INTO affiliate_links(id,org_id,marketplace_id,title,image_url,destination_url,shorten_policy_override,active,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$9)", l.ID, l.OrgID, l.MarketplaceID, l.Title, l.ImageURL, l.DestinationURL, l.ShortenPolicyOverride, l.Active, now); err != nil {
		return l, err
	}
	for n := 0; n < 5; n++ {
		l.Code, err = c.newCode()
		if err != nil {
			return l, err
		}
		var inserted string
		err = tx.QueryRow(ctx, "INSERT INTO short_codes(code,org_id,target_type,target_id,created_at) VALUES($1,$2,'affiliate_link',$3,$4) ON CONFLICT (code) DO NOTHING RETURNING code", l.Code, l.OrgID, l.ID, now).Scan(&inserted)
		if err == nil && inserted != "" {
			return l, tx.Commit(ctx)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return l, err
		}
	}
	return l, domain.ErrConflict
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
		args = append(args, "%"+o.Query+"%")
		where += fmt.Sprintf(" AND name ILIKE $%d", len(args))
	}
	total, err := c.count(ctx, "marketplaces", where, args)
	if err != nil {
		return nil, 0, err
	}
	q, args := paged("SELECT id,org_id,name,shorten_policy,created_at,updated_at,deleted_at,purged_at FROM marketplaces WHERE "+where+" ORDER BY lower(name)", args, o)
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
		args = append(args, "%"+o.Query+"%")
		where += fmt.Sprintf(" AND (name ILIKE $%d OR segment ILIKE $%d)", len(args), len(args))
	}
	total, err := c.count(ctx, "channels", where, args)
	if err != nil {
		return nil, 0, err
	}
	q, args := paged("SELECT id,org_id,name,segment,created_at,updated_at,deleted_at,purged_at FROM channels WHERE "+where+" ORDER BY lower(name)", args, o)
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
		args = append(args, "%"+o.Query+"%")
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
	q, args := paged("SELECT l.id,l.org_id,l.marketplace_id,l.title,l.image_url,l.destination_url,l.shorten_policy_override,l.active,l.created_at,l.updated_at,l.deleted_at,l.purged_at,s.code,m.id,m.org_id,m.name,m.shorten_policy,m.created_at,m.updated_at,m.deleted_at,m.purged_at FROM affiliate_links l JOIN marketplaces m ON m.id=l.marketplace_id JOIN short_codes s ON s.target_id=l.id AND s.target_type='affiliate_link' WHERE "+where+" ORDER BY l.created_at DESC", args, o)
	rows, err := c.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []domain.AffiliateLink{}
	for rows.Next() {
		var l domain.AffiliateLink
		var mp domain.Marketplace
		var override *string
		if err := rows.Scan(&l.ID, &l.OrgID, &l.MarketplaceID, &l.Title, &l.ImageURL, &l.DestinationURL, &override, &l.Active, &l.CreatedAt, &l.UpdatedAt, &l.DeletedAt, &l.PurgedAt, &l.Code, &mp.ID, &mp.OrgID, &mp.Name, &mp.ShortenPolicy, &mp.CreatedAt, &mp.UpdatedAt, &mp.DeletedAt, &mp.PurgedAt); err != nil {
			return nil, 0, err
		}
		if override != nil {
			v := domain.Policy(*override)
			l.ShortenPolicyOverride = &v
		}
		l.Marketplace = mp
		items = append(items, l)
	}
	return items, total, rows.Err()
}

func (c *Catalog) count(ctx context.Context, table, where string, args []any) (int, error) {
	var total int
	err := c.Pool.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE "+where, args...).Scan(&total)
	return total, err
}
func paged(q string, args []any, o ListOptions) (string, []any) {
	if o.Page < 1 {
		return q, args
	}
	per := o.PerPage
	if per < 1 {
		per = 20
	}
	if per > 100 {
		per = 100
	}
	args = append(args, per, (o.Page-1)*per)
	return fmt.Sprintf("%s LIMIT $%d OFFSET $%d", q, len(args)-1, len(args)), args
}

func (c *Catalog) SoftDeleteMarketplace(ctx context.Context, orgID, id uuid.UUID) error {
	rows, err := c.Pool.Query(ctx, "SELECT id,title FROM affiliate_links WHERE org_id=$1 AND marketplace_id=$2 AND deleted_at IS NULL AND purged_at IS NULL ORDER BY created_at", orgID, id)
	if err != nil {
		return err
	}
	defer rows.Close()
	links := []domain.LinkRef{}
	for rows.Next() {
		var link domain.LinkRef
		if err := rows.Scan(&link.ID, &link.Title); err != nil {
			return err
		}
		links = append(links, link)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(links) > 0 {
		return domain.MarketplaceInUseError{Links: links}
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

// UpdateLink changes only supplied fields and returns the code so callers can
// invalidate the public resolution cache after the transaction succeeds.
func (c *Catalog) UpdateLink(ctx context.Context, orgID, id uuid.UUID, title, destinationURL, imageURL *string, marketplaceID *uuid.UUID, policy *domain.Policy, active *bool) (string, error) {
	if title != nil && !domain.Text(*title, 1, 200) {
		return "", domain.ValidationError{Fields: map[string]string{"title": "título inválido"}}
	}
	if destinationURL != nil && !domain.URL(*destinationURL) {
		return "", domain.ValidationError{Fields: map[string]string{"destination_url": "URL deve usar http ou https"}}
	}
	if imageURL != nil && *imageURL != "" && !domain.URL(*imageURL) {
		return "", domain.ValidationError{Fields: map[string]string{"image_url": "URL deve usar http ou https"}}
	}
	if policy != nil && !domain.PolicyValid(*policy) {
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
	var image any = imageURL
	if imageURL != nil && *imageURL == "" {
		image = nil
	}
	var code string
	err := c.Pool.QueryRow(ctx, `UPDATE affiliate_links l SET
		title=COALESCE($3,l.title), destination_url=COALESCE($4,l.destination_url),
		image_url=CASE WHEN $5::text IS NULL THEN l.image_url ELSE NULLIF($5::text,'') END,
		marketplace_id=COALESCE($6,l.marketplace_id), shorten_policy_override=COALESCE($7,l.shorten_policy_override),
		active=COALESCE($8,l.active), updated_at=$9
		FROM short_codes s WHERE l.id=$1 AND l.org_id=$2 AND l.deleted_at IS NULL AND l.purged_at IS NULL
		AND s.target_id=l.id AND s.target_type='affiliate_link' RETURNING s.code`, id, orgID, title, destinationURL, image, marketplaceID, policy, active, time.Now().UTC()).Scan(&code)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	return code, err
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
func (c *Catalog) GetLink(ctx context.Context, orgID, id uuid.UUID, trash bool) (domain.AffiliateLink, error) {
	items, _, err := c.ListLinksWithOptions(ctx, orgID, ListOptions{Trash: trash})
	if err != nil {
		return domain.AffiliateLink{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return domain.AffiliateLink{}, domain.ErrNotFound
}
func (c *Catalog) CountActiveLinksByChannel(ctx context.Context, orgID uuid.UUID) (int, error) {
	var count int
	err := c.Pool.QueryRow(ctx, "SELECT count(*) FROM affiliate_links WHERE org_id=$1 AND active AND deleted_at IS NULL AND purged_at IS NULL", orgID).Scan(&count)
	return count, err
}

func (c *Catalog) CountActiveLinksByMarketplace(ctx context.Context, orgID, marketplaceID uuid.UUID) (int, error) {
	var count int
	err := c.Pool.QueryRow(ctx, "SELECT count(*) FROM affiliate_links WHERE org_id=$1 AND marketplace_id=$2 AND active AND deleted_at IS NULL AND purged_at IS NULL", orgID, marketplaceID).Scan(&count)
	return count, err
}
