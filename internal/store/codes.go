package store

import (
	"context"
	"errors"
	"time"

	"github.com/hublinks/hublinks/internal/domain"
	"github.com/jackc/pgx/v5"
)

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

// insertShortCode gera e grava o código curto do link na transação do chamador,
// com até 5 tentativas quando o código já existe (unicidade global).
func (c *Catalog) insertShortCode(ctx context.Context, tx pgx.Tx, l domain.AffiliateLink, now time.Time) (string, error) {
	for n := 0; n < 5; n++ {
		code, err := c.newCode()
		if err != nil {
			return "", err
		}
		var inserted string
		err = tx.QueryRow(ctx, "INSERT INTO short_codes(code,org_id,target_type,target_id,created_at) VALUES($1,$2,'affiliate_link',$3,$4) ON CONFLICT (code) DO NOTHING RETURNING code", code, l.OrgID, l.ID, now).Scan(&inserted)
		if err == nil && inserted != "" {
			return code, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return "", err
		}
	}
	return "", domain.ErrConflict
}
