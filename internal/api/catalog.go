package api

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/auth"
	"github.com/hublinks/hublinks/internal/domain"
	"github.com/hublinks/hublinks/internal/store"
	"net/http"
)

type Catalog struct{ Store *store.Catalog }

func (c Catalog) Marketplace(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	if r.Method != http.MethodPost {
		http.Error(w, "método não permitido", 405)
		return
	}
	var in struct {
		Name   string        `json:"name"`
		Policy domain.Policy `json:"shorten_policy"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		errorJSON(w, 400, "invalid_json", "JSON inválido")
		return
	}
	v, e := c.Store.CreateMarketplace(r.Context(), s.OrgID, in.Name, in.Policy)
	if e != nil {
		catalogError(w, e)
		return
	}
	jsonOut(w, 201, v)
}
func (c Catalog) Channel(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	if r.Method != http.MethodPost {
		http.Error(w, "método não permitido", 405)
		return
	}
	var in struct {
		Name    string `json:"name"`
		Segment string `json:"segment"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		errorJSON(w, 400, "invalid_json", "JSON inválido")
		return
	}
	v, e := c.Store.CreateChannel(r.Context(), s.OrgID, in.Name, in.Segment)
	if e != nil {
		catalogError(w, e)
		return
	}
	jsonOut(w, 201, v)
}
func (c Catalog) Link(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	if r.Method != http.MethodPost {
		http.Error(w, "método não permitido", 405)
		return
	}
	var in struct {
		Title          string         `json:"title"`
		DestinationURL string         `json:"destination_url"`
		MarketplaceID  uuid.UUID      `json:"marketplace_id"`
		ImageURL       *string        `json:"image_url"`
		Policy         *domain.Policy `json:"shorten_policy_override"`
		Active         *bool          `json:"active"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		errorJSON(w, 400, "invalid_json", "JSON inválido")
		return
	}
	active := true
	if in.Active != nil {
		active = *in.Active
	}
	v, e := c.Store.CreateLink(r.Context(), domain.AffiliateLink{OrgID: s.OrgID, Title: in.Title, DestinationURL: in.DestinationURL, MarketplaceID: in.MarketplaceID, ImageURL: in.ImageURL, ShortenPolicyOverride: in.Policy, Active: active})
	if e != nil {
		catalogError(w, e)
		return
	}
	jsonOut(w, 201, map[string]any{"link": v, "trackable": v.Trackable()})
}
func jsonOut(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func errorJSON(w http.ResponseWriter, status int, code, msg string) {
	jsonOut(w, status, map[string]any{"error": map[string]any{"code": code, "message": msg}})
}
func catalogError(w http.ResponseWriter, e error) {
	var ve domain.ValidationError
	if errors.As(e, &ve) {
		jsonOut(w, 422, map[string]any{"error": map[string]any{"code": "validation_failed", "message": e.Error(), "fields": ve.Fields}})
		return
	}
	if errors.Is(e, domain.ErrNotFound) {
		errorJSON(w, 404, "not_found", e.Error())
		return
	}
	errorJSON(w, 409, "conflict", e.Error())
}
