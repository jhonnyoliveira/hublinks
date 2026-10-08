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
	if r.Method == http.MethodGet {
		items, err := c.Store.ListMarketplaces(r.Context(), s.OrgID)
		if err != nil {
			errorJSON(w, 500, "internal_error", "erro ao listar marketplaces")
			return
		}
		jsonOut(w, 200, map[string]any{"items": items})
		return
	}
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
	if r.Method == http.MethodGet {
		items, err := c.Store.ListChannels(r.Context(), s.OrgID)
		if err != nil {
			errorJSON(w, 500, "internal_error", "erro ao listar canais")
			return
		}
		jsonOut(w, 200, map[string]any{"items": items})
		return
	}
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
	if r.Method == http.MethodGet {
		items, err := c.Store.ListLinks(r.Context(), s.OrgID)
		if err != nil {
			errorJSON(w, 500, "internal_error", "erro ao listar links")
			return
		}
		jsonOut(w, 200, map[string]any{"items": items})
		return
	}
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
func (c Catalog) MarketplaceItem(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		errorJSON(w, 404, "not_found", "não encontrado")
		return
	}
	if r.Method == http.MethodPatch {
		var in struct {
			Name   *string        `json:"name"`
			Policy *domain.Policy `json:"shorten_policy"`
		}
		if json.NewDecoder(r.Body).Decode(&in) != nil {
			errorJSON(w, 400, "invalid_json", "JSON inválido")
			return
		}
		err = c.Store.UpdateMarketplace(r.Context(), s.OrgID, id, in.Name, in.Policy)
		if err == nil {
			jsonOut(w, 200, map[string]bool{"updated": true})
		} else {
			catalogError(w, err)
		}
		return
	}
	if r.Method != http.MethodDelete {
		http.Error(w, "método não permitido", 405)
		return
	}
	if err = c.Store.SoftDeleteMarketplace(r.Context(), s.OrgID, id); err != nil {
		catalogError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (c Catalog) ChannelItem(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		errorJSON(w, 404, "not_found", "não encontrado")
		return
	}
	if r.Method == http.MethodPatch {
		var in struct {
			Name    *string `json:"name"`
			Segment *string `json:"segment"`
		}
		if json.NewDecoder(r.Body).Decode(&in) != nil {
			errorJSON(w, 400, "invalid_json", "JSON inválido")
			return
		}
		err = c.Store.UpdateChannel(r.Context(), s.OrgID, id, in.Name, in.Segment)
		if err == nil {
			jsonOut(w, 200, map[string]bool{"updated": true})
		} else {
			catalogError(w, err)
		}
		return
	}
	if r.Method != http.MethodDelete {
		http.Error(w, "método não permitido", 405)
		return
	}
	if err = c.Store.SoftDeleteChannel(r.Context(), s.OrgID, id); err != nil {
		catalogError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (c Catalog) LinkItem(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		errorJSON(w, 404, "not_found", "não encontrado")
		return
	}
	if r.Method != http.MethodDelete {
		http.Error(w, "método não permitido", 405)
		return
	}
	if err = c.Store.SoftDeleteLink(r.Context(), s.OrgID, id); err != nil {
		catalogError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (c Catalog) MarketplaceRestore(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		errorJSON(w, 404, "not_found", "não encontrado")
		return
	}
	if err = c.Store.RestoreMarketplace(r.Context(), s.OrgID, id); err != nil {
		catalogError(w, err)
		return
	}
	jsonOut(w, 200, map[string]bool{"restored": true})
}
func (c Catalog) ChannelRestore(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		errorJSON(w, 404, "not_found", "não encontrado")
		return
	}
	if err = c.Store.RestoreChannel(r.Context(), s.OrgID, id); err != nil {
		catalogError(w, err)
		return
	}
	jsonOut(w, 200, map[string]bool{"restored": true})
}
func (c Catalog) LinkRestore(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		errorJSON(w, 404, "not_found", "não encontrado")
		return
	}
	if err = c.Store.RestoreLink(r.Context(), s.OrgID, id); err != nil {
		catalogError(w, err)
		return
	}
	jsonOut(w, 200, map[string]bool{"restored": true})
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
