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
	ID            uuid.UUID  `json:"id"`
	OrgID         uuid.UUID  `json:"-"`
	Name          string     `json:"name"`
	ShortenPolicy Policy     `json:"shorten_policy"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DeletedAt     *time.Time `json:"deleted_at"`
	PurgedAt      *time.Time `json:"-"`
}
type Channel struct {
	ID        uuid.UUID  `json:"id"`
	OrgID     uuid.UUID  `json:"-"`
	Name      string     `json:"name"`
	Segment   string     `json:"segment"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at"`
	PurgedAt  *time.Time `json:"-"`
}
type AffiliateLink struct {
	ID                    uuid.UUID   `json:"id"`
	OrgID                 uuid.UUID   `json:"-"`
	MarketplaceID         uuid.UUID   `json:"-"`
	Title                 string      `json:"title"`
	ImageURL              *string     `json:"image_url"`
	DestinationURL        string      `json:"destination_url"`
	ShortenPolicyOverride *Policy     `json:"shorten_policy_override"`
	Active                bool        `json:"active"`
	Code                  string      `json:"code"`
	Marketplace           Marketplace `json:"marketplace"`
	CreatedAt             time.Time   `json:"created_at"`
	UpdatedAt             time.Time   `json:"updated_at"`
	DeletedAt             *time.Time  `json:"deleted_at"`
	PurgedAt              *time.Time  `json:"-"`
}

func (l AffiliateLink) EffectivePolicy() Policy {
	if l.ShortenPolicyOverride != nil {
		return *l.ShortenPolicyOverride
	}
	return l.Marketplace.ShortenPolicy
}
func (l AffiliateLink) Trackable() bool { return l.EffectivePolicy() == PolicyShorten }
