package api

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/auth"
	"github.com/hublinks/hublinks/internal/domain"
)

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
	c.writeChannel(w, r, s.OrgID, v.ID, 201)
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
		jsonOut(w, 200, channelView(v, count))
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
		if err = c.Service.UpdateChannel(r.Context(), s.OrgID, id, in.Name, in.Segment); err != nil {
			catalogError(w, err)
			return
		}
		c.writeChannel(w, r, s.OrgID, id, 200)
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
	c.writeChannel(w, r, s.OrgID, id, 200)
}

func channelView(v domain.Channel, activeLinks int) map[string]any {
	return map[string]any{"id": v.ID, "name": v.Name, "segment": v.Segment, "active_links_count": activeLinks, "deleted_at": v.DeletedAt, "created_at": v.CreatedAt, "updated_at": v.UpdatedAt}
}

func (c Catalog) writeChannel(w http.ResponseWriter, r *http.Request, org, id uuid.UUID, status int) {
	v, count, err := c.Service.Store.GetChannel(r.Context(), org, id, false)
	if err != nil {
		catalogError(w, err)
		return
	}
	jsonOut(w, status, channelView(v, count))
}
