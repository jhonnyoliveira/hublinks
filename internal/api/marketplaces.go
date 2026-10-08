package api

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/auth"
	"github.com/hublinks/hublinks/internal/domain"
)

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
		if err = c.Service.UpdateMarketplace(r.Context(), s.OrgID, id, in.Name, in.Policy); err != nil {
			catalogError(w, err)
			return
		}
		c.writeMarketplace(w, r, s.OrgID, id, 200)
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
	c.writeMarketplace(w, r, s.OrgID, id, 200)
}

func (c Catalog) writeMarketplace(w http.ResponseWriter, r *http.Request, org, id uuid.UUID, status int) {
	v, err := c.Service.Store.GetMarketplace(r.Context(), org, id, false)
	if err != nil {
		catalogError(w, err)
		return
	}
	jsonOut(w, status, v)
}
