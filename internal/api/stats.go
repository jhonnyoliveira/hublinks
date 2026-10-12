package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/auth"
	"github.com/hublinks/hublinks/internal/domain"
	"github.com/hublinks/hublinks/internal/httpx"
	"github.com/hublinks/hublinks/internal/stats"
)

// Stats serve /api/v1/stats/*. BaseURL monta as URLs públicas do detalhe.
type Stats struct {
	Service stats.Service
	BaseURL string
}

// period lê period/from/to e responde 422 quando inválidos.
func (s Stats) period(w http.ResponseWriter, r *http.Request) (stats.Period, bool) {
	q := r.URL.Query()
	p, err := stats.ParsePeriod(q.Get("period"), q.Get("from"), q.Get("to"), time.Now(), s.Service.Location())
	if err != nil {
		httpx.APIError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error(), map[string]string{"period": err.Error()})
		return p, false
	}
	return p, true
}

func statsError(w http.ResponseWriter, err error) {
	if errors.Is(err, domain.ErrNotFound) {
		errorJSON(w, http.StatusNotFound, "not_found", "não encontrado")
		return
	}
	errorJSON(w, http.StatusInternalServerError, "internal_error", "erro ao consultar estatísticas")
}

func (s Stats) Summary(w http.ResponseWriter, r *http.Request) {
	session, _ := auth.FromContext(r.Context())
	p, ok := s.period(w, r)
	if !ok {
		return
	}
	out, err := s.Service.Summary(r.Context(), session.OrgID, p)
	if err != nil {
		statsError(w, err)
		return
	}
	jsonOut(w, http.StatusOK, out)
}

func (s Stats) Links(w http.ResponseWriter, r *http.Request) {
	session, _ := auth.FromContext(r.Context())
	p, ok := s.period(w, r)
	if !ok {
		return
	}
	o := listOptions(r)
	f := stats.LinkFilter{Q: o.Query, Page: o.Page, PerPage: o.PerPage}
	if o.MarketplaceID != uuid.Nil {
		f.MarketplaceID = &o.MarketplaceID
	}
	items, total, err := s.Service.Links(r.Context(), session.OrgID, p, f)
	if err != nil {
		statsError(w, err)
		return
	}
	jsonOut(w, http.StatusOK, listResponse(r, items, total))
}

type channelItem struct {
	stats.ChannelStat
	URL string `json:"url"`
}

func (s Stats) Link(w http.ResponseWriter, r *http.Request) {
	session, _ := auth.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "não encontrado")
		return
	}
	p, ok := s.period(w, r)
	if !ok {
		return
	}
	d, err := s.Service.Link(r.Context(), session.OrgID, id, p)
	if err != nil {
		statsError(w, err)
		return
	}
	base := strings.TrimSuffix(s.BaseURL, "/")
	byChannel := make([]channelItem, 0, len(d.ByChannel))
	for _, c := range d.ByChannel {
		url := base + "/" + d.Link.Code
		if c.Channel != nil {
			url = base + "/" + c.Channel.Segment + "/" + d.Link.Code
		}
		byChannel = append(byChannel, channelItem{c, url})
	}
	jsonOut(w, http.StatusOK, map[string]any{"link": d.Link, "totals": d.Totals, "history": d.History, "by_channel": byChannel, "daily": d.Daily})
}

func (s Stats) Channels(w http.ResponseWriter, r *http.Request) {
	session, _ := auth.FromContext(r.Context())
	p, ok := s.period(w, r)
	if !ok {
		return
	}
	items, err := s.Service.Channels(r.Context(), session.OrgID, p)
	if err != nil {
		statsError(w, err)
		return
	}
	jsonOut(w, http.StatusOK, map[string]any{"items": items})
}
