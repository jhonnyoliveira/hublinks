// Package service contains catalog use cases shared by the JSON API and panel.
package service

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/domain"
	"github.com/hublinks/hublinks/internal/store"
)

type CodeCache interface {
	Invalidate(string)
	InvalidateAll()
}

type Catalog struct {
	Store   *store.Catalog
	Cache   CodeCache
	BaseURL string
}

type URL struct {
	Channel *ChannelRef `json:"channel,omitempty"`
	Label   string      `json:"label,omitempty"`
	URL     string      `json:"url"`
}
type ChannelRef struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	Segment string    `json:"segment"`
}
type LinkView struct {
	ID                    uuid.UUID          `json:"id"`
	Title                 string             `json:"title"`
	DestinationURL        string             `json:"destination_url"`
	ImageURL              *string            `json:"image_url"`
	Marketplace           domain.Marketplace `json:"marketplace"`
	ShortenPolicyOverride *domain.Policy     `json:"shorten_policy_override"`
	EffectivePolicy       domain.Policy      `json:"effective_policy"`
	Trackable             bool               `json:"trackable"`
	Active                bool               `json:"active"`
	Code                  string             `json:"code"`
	URLs                  []URL              `json:"urls"`
	CreatedAt             string             `json:"created_at"`
	UpdatedAt             string             `json:"updated_at"`
	DeletedAt             any                `json:"deleted_at"`
}

func (s Catalog) base() string { return strings.TrimSuffix(s.BaseURL, "/") }
func (s Catalog) view(ctx context.Context, orgID uuid.UUID, l domain.AffiliateLink) (LinkView, error) {
	v := LinkView{ID: l.ID, Title: l.Title, DestinationURL: l.DestinationURL, ImageURL: l.ImageURL, Marketplace: l.Marketplace, ShortenPolicyOverride: l.ShortenPolicyOverride, EffectivePolicy: l.EffectivePolicy(), Trackable: l.Trackable(), Active: l.Active, Code: l.Code, CreatedAt: l.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"), UpdatedAt: l.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"), DeletedAt: l.DeletedAt}
	if !v.Trackable {
		v.URLs = []URL{{Label: "original", URL: l.DestinationURL}}
		return v, nil
	}
	v.URLs = []URL{{Label: "curta", URL: s.base() + "/" + l.Code}}
	channels, err := s.Store.ListChannels(ctx, orgID)
	if err != nil {
		return LinkView{}, err
	}
	for _, ch := range channels {
		ref := &ChannelRef{ID: ch.ID, Name: ch.Name, Segment: ch.Segment}
		v.URLs = append(v.URLs, URL{Channel: ref, URL: s.base() + "/" + ch.Segment + "/" + l.Code})
	}
	return v, nil
}
func (s Catalog) Link(ctx context.Context, orgID, id uuid.UUID, trash bool) (LinkView, error) {
	l, err := s.Store.GetLink(ctx, orgID, id, trash)
	if err != nil {
		return LinkView{}, err
	}
	return s.view(ctx, orgID, l)
}
func (s Catalog) Links(ctx context.Context, orgID uuid.UUID, o store.ListOptions) ([]LinkView, int, error) {
	links, total, err := s.Store.ListLinksWithOptions(ctx, orgID, o)
	if err != nil {
		return nil, 0, err
	}
	items := make([]LinkView, 0, len(links))
	for _, l := range links {
		v, err := s.view(ctx, orgID, l)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, v)
	}
	return items, total, nil
}
func (s Catalog) invalidate(code string) {
	if s.Cache != nil {
		s.Cache.Invalidate(code)
	}
}
func (s Catalog) CreateLink(ctx context.Context, l domain.AffiliateLink) (LinkView, error) {
	v, err := s.Store.CreateLink(ctx, l)
	if err != nil {
		return LinkView{}, err
	}
	s.invalidate(v.Code)
	return s.view(ctx, l.OrgID, v)
}
func (s Catalog) UpdateLink(ctx context.Context, orgID, id uuid.UUID, title, destination, image *string, marketplace *uuid.UUID, policy *domain.Policy, active *bool) error {
	code, err := s.Store.UpdateLink(ctx, orgID, id, title, destination, image, marketplace, policy, active)
	if err == nil {
		s.invalidate(code)
	}
	return err
}
func (s Catalog) DeleteLink(ctx context.Context, orgID, id uuid.UUID) error {
	l, err := s.Store.GetLink(ctx, orgID, id, false)
	if err != nil {
		return err
	}
	if err = s.Store.SoftDeleteLink(ctx, orgID, id); err == nil {
		s.invalidate(l.Code)
	}
	return err
}
func (s Catalog) RestoreLink(ctx context.Context, orgID, id uuid.UUID) error {
	if err := s.Store.RestoreLink(ctx, orgID, id); err != nil {
		return err
	}
	l, err := s.Store.GetLink(ctx, orgID, id, false)
	if err == nil {
		s.invalidate(l.Code)
	}
	return err
}
func (s Catalog) CreateMarketplace(ctx context.Context, orgID uuid.UUID, name string, policy domain.Policy) (domain.Marketplace, error) {
	v, err := s.Store.CreateMarketplace(ctx, orgID, name, policy)
	if err == nil && s.Cache != nil {
		s.Cache.InvalidateAll()
	}
	return v, err
}
func (s Catalog) UpdateMarketplace(ctx context.Context, orgID, id uuid.UUID, name *string, policy *domain.Policy) error {
	err := s.Store.UpdateMarketplace(ctx, orgID, id, name, policy)
	if err == nil && s.Cache != nil {
		s.Cache.InvalidateAll()
	}
	return err
}
func (s Catalog) DeleteMarketplace(ctx context.Context, orgID, id uuid.UUID) error {
	err := s.Store.SoftDeleteMarketplace(ctx, orgID, id)
	if err == nil && s.Cache != nil {
		s.Cache.InvalidateAll()
	}
	return err
}
func (s Catalog) RestoreMarketplace(ctx context.Context, orgID, id uuid.UUID) error {
	err := s.Store.RestoreMarketplace(ctx, orgID, id)
	if err == nil && s.Cache != nil {
		s.Cache.InvalidateAll()
	}
	return err
}
func (s Catalog) CreateChannel(ctx context.Context, orgID uuid.UUID, name, segment string) (domain.Channel, error) {
	v, err := s.Store.CreateChannel(ctx, orgID, name, segment)
	if err == nil && s.Cache != nil {
		s.Cache.InvalidateAll()
	}
	return v, err
}
func (s Catalog) UpdateChannel(ctx context.Context, orgID, id uuid.UUID, name, segment *string) error {
	err := s.Store.UpdateChannel(ctx, orgID, id, name, segment)
	if err == nil && s.Cache != nil {
		s.Cache.InvalidateAll()
	}
	return err
}
func (s Catalog) DeleteChannel(ctx context.Context, orgID, id uuid.UUID) error {
	err := s.Store.SoftDeleteChannel(ctx, orgID, id)
	if err == nil && s.Cache != nil {
		s.Cache.InvalidateAll()
	}
	return err
}
func (s Catalog) RestoreChannel(ctx context.Context, orgID, id uuid.UUID) error {
	err := s.Store.RestoreChannel(ctx, orgID, id)
	if err == nil && s.Cache != nil {
		s.Cache.InvalidateAll()
	}
	return err
}
