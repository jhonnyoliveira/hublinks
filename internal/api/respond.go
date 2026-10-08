package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/domain"
	"github.com/hublinks/hublinks/internal/httpx"
	"github.com/hublinks/hublinks/internal/store"
)

func jsonOut(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// errorJSON escreve o envelope de erro padrão de /api/v1.
func errorJSON(w http.ResponseWriter, status int, code, msg string) {
	httpx.APIError(w, status, code, msg, nil)
}

func catalogError(w http.ResponseWriter, e error) {
	var ve domain.ValidationError
	if errors.As(e, &ve) {
		httpx.APIError(w, 422, "validation_failed", e.Error(), ve.Fields)
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
	if errors.Is(e, domain.ErrConflict) {
		errorJSON(w, 409, "conflict", e.Error())
		return
	}
	errorJSON(w, 500, "internal_error", "erro interno")
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
	o := store.ListOptions{Query: q.Get("q"), Trash: q.Get("trash") == "true", Sort: q.Get("sort"), Page: page, PerPage: per}
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
