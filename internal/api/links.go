package api

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/auth"
	"github.com/hublinks/hublinks/internal/domain"
)

func (c Catalog) Link(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	if r.Method == http.MethodGet {
		items, total, err := c.Service.Links(r.Context(), s.OrgID, listOptions(r))
		if err != nil {
			errorJSON(w, 500, "internal_error", "erro ao listar links")
			return
		}
		jsonOut(w, 200, listResponse(r, items, total))
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
	v, e := c.Service.CreateLink(r.Context(), domain.AffiliateLink{OrgID: s.OrgID, Title: in.Title, DestinationURL: in.DestinationURL, MarketplaceID: in.MarketplaceID, ImageURL: in.ImageURL, ShortenPolicyOverride: in.Policy, Active: active})
	if e != nil {
		catalogError(w, e)
		return
	}
	jsonOut(w, 201, v)
}

func (c Catalog) LinkItem(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		errorJSON(w, 404, "not_found", "não encontrado")
		return
	}
	if r.Method == http.MethodGet {
		v, e := c.Service.Link(r.Context(), s.OrgID, id, r.URL.Query().Get("trash") == "true")
		if e != nil {
			catalogError(w, e)
			return
		}
		jsonOut(w, 200, v)
		return
	}
	if r.Method == http.MethodPatch {
		var in struct {
			Title, DestinationURL, ImageURL *string
			MarketplaceID                   *uuid.UUID     `json:"marketplace_id"`
			Policy                          *domain.Policy `json:"shorten_policy_override"`
			Active                          *bool          `json:"active"`
		}
		if json.NewDecoder(r.Body).Decode(&in) != nil {
			errorJSON(w, 400, "invalid_json", "JSON inválido")
			return
		}
		if err = c.Service.UpdateLink(r.Context(), s.OrgID, id, in.Title, in.DestinationURL, in.ImageURL, in.MarketplaceID, in.Policy, in.Active); err != nil {
			catalogError(w, err)
			return
		}
		v, err := c.Service.Link(r.Context(), s.OrgID, id, false)
		if err != nil {
			catalogError(w, err)
			return
		}
		jsonOut(w, 200, v)
		return
	}
	if r.Method != http.MethodDelete {
		http.Error(w, "método não permitido", 405)
		return
	}
	if err = c.Service.DeleteLink(r.Context(), s.OrgID, id); err != nil {
		catalogError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c Catalog) LinkRestore(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		errorJSON(w, 404, "not_found", "não encontrado")
		return
	}
	if err = c.Service.RestoreLink(r.Context(), s.OrgID, id); err != nil {
		catalogError(w, err)
		return
	}
	v, err := c.Service.Link(r.Context(), s.OrgID, id, false)
	if err != nil {
		catalogError(w, err)
		return
	}
	jsonOut(w, 200, v)
}
