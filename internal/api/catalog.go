package api

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/auth"
	"github.com/hublinks/hublinks/internal/domain"
	"github.com/hublinks/hublinks/internal/service"
	"github.com/hublinks/hublinks/internal/store"
	"net/http"
	"strconv"
)

type Catalog struct{ Service service.Catalog }

func (c Catalog) Marketplace(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	if r.Method == http.MethodGet {
		items, total, err := c.Service.Store.ListMarketplacesWithOptions(r.Context(), s.OrgID, listOptions(r))
		if err != nil {
			errorJSON(w, 500, "internal_error", "erro ao listar marketplaces")
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
		Name   string        `json:"name"`
		Policy domain.Policy `json:"shorten_policy"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		errorJSON(w, 400, "invalid_json", "JSON inválido")
		return
	}
	v, e := c.Service.CreateMarketplace(r.Context(), s.OrgID, in.Name, in.Policy)
	if e != nil {
		catalogError(w, e)
		return
	}
	jsonOut(w, 201, v)
}
func (c Catalog) Channel(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	if r.Method == http.MethodGet {
		items, total, err := c.Service.Store.ListChannelsWithOptions(r.Context(), s.OrgID, listOptions(r))
		if err != nil {
			errorJSON(w, 500, "internal_error", "erro ao listar canais")
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
		Name    string `json:"name"`
		Segment string `json:"segment"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		errorJSON(w, 400, "invalid_json", "JSON inválido")
		return
	}
	v, e := c.Service.CreateChannel(r.Context(), s.OrgID, in.Name, in.Segment)
	if e != nil {
		catalogError(w, e)
		return
	}
	jsonOut(w, 201, v)
}
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
func (c Catalog) MarketplaceItem(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		errorJSON(w, 404, "not_found", "não encontrado")
		return
	}
	if r.Method == http.MethodGet {
		v, e := c.Service.Store.GetMarketplace(r.Context(), s.OrgID, id, r.URL.Query().Get("trash") == "true")
		if e != nil {
			catalogError(w, e)
			return
		}
		jsonOut(w, 200, v)
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
		err = c.Service.UpdateMarketplace(r.Context(), s.OrgID, id, in.Name, in.Policy)
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
	if err = c.Service.DeleteMarketplace(r.Context(), s.OrgID, id); err != nil {
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
	if r.Method == http.MethodGet {
		v, count, e := c.Service.Store.GetChannel(r.Context(), s.OrgID, id, r.URL.Query().Get("trash") == "true")
		if e != nil {
			catalogError(w, e)
			return
		}
		jsonOut(w, 200, map[string]any{"id": v.ID, "name": v.Name, "segment": v.Segment, "active_links_count": count, "deleted_at": v.DeletedAt, "created_at": v.CreatedAt, "updated_at": v.UpdatedAt})
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
		err = c.Service.UpdateChannel(r.Context(), s.OrgID, id, in.Name, in.Segment)
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
	if err = c.Service.DeleteChannel(r.Context(), s.OrgID, id); err != nil {
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
func (c Catalog) MarketplaceRestore(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		errorJSON(w, 404, "not_found", "não encontrado")
		return
	}
	if err = c.Service.RestoreMarketplace(r.Context(), s.OrgID, id); err != nil {
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
	if err = c.Service.RestoreChannel(r.Context(), s.OrgID, id); err != nil {
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
	if err = c.Service.RestoreLink(r.Context(), s.OrgID, id); err != nil {
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
	var inUse domain.MarketplaceInUseError
	if errors.As(e, &inUse) {
		jsonOut(w, 409, map[string]any{"error": map[string]any{"code": "conflict", "message": e.Error()}, "links": inUse.Links})
		return
	}
	errorJSON(w, 409, "conflict", e.Error())
}

func listOptions(r *http.Request) store.ListOptions {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	per, _ := strconv.Atoi(q.Get("per_page"))
	if per < 1 {
		per = 20
	}
	if per > 100 {
		per = 100
	}
	o := store.ListOptions{Query: q.Get("q"), Trash: q.Get("trash") == "true", Page: page, PerPage: per}
	if id, err := uuid.Parse(q.Get("marketplace_id")); err == nil {
		o.MarketplaceID = id
	}
	if raw := q.Get("active"); raw != "" {
		if v, err := strconv.ParseBool(raw); err == nil {
			o.Active = &v
		}
	}
	return o
}
func listResponse(r *http.Request, items any, total int) map[string]any {
	o := listOptions(r)
	return map[string]any{"items": items, "page": o.Page, "per_page": o.PerPage, "total": total}
}
