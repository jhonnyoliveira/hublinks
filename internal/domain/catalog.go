package domain

import (
	"github.com/google/uuid"
	"time"
)

type Policy string

const (
	PolicyShorten Policy = "shorten"
	PolicyDirect  Policy = "direct"
)

type Marketplace struct {
	ID, OrgID            uuid.UUID
	Name                 string
	ShortenPolicy        Policy
	CreatedAt, UpdatedAt time.Time
	DeletedAt, PurgedAt  *time.Time
}
type Channel struct {
	ID, OrgID            uuid.UUID
	Name, Segment        string
	CreatedAt, UpdatedAt time.Time
	DeletedAt, PurgedAt  *time.Time
}
type AffiliateLink struct {
	ID, OrgID, MarketplaceID uuid.UUID
	Title                    string
	ImageURL                 *string
	DestinationURL           string
	ShortenPolicyOverride    *Policy
	Active                   bool
	Code                     string
	Marketplace              Marketplace
	CreatedAt, UpdatedAt     time.Time
	DeletedAt, PurgedAt      *time.Time
}

func (l AffiliateLink) EffectivePolicy() Policy {
	if l.ShortenPolicyOverride != nil {
		return *l.ShortenPolicyOverride
	}
	return l.Marketplace.ShortenPolicy
}
func (l AffiliateLink) Trackable() bool { return l.EffectivePolicy() == PolicyShorten }
