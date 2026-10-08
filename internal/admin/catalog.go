package admin

import (
	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/auth"
	"github.com/hublinks/hublinks/internal/domain"
	"github.com/hublinks/hublinks/internal/store"
	webtmpl "github.com/hublinks/hublinks/web"
	"net/http"
)

type Catalog struct{ Store *store.Catalog }

func (c Catalog) Marketplaces(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	var errText string
	if r.Method == http.MethodPost {
		_, err := c.Store.CreateMarketplace(r.Context(), s.OrgID, r.FormValue("name"), domain.Policy(r.FormValue("shorten_policy")))
		if err == nil {
			http.Redirect(w, r, "/admin/marketplaces", http.StatusSeeOther)
			return
		}
		errText = err.Error()
	}
	items, err := c.Store.ListMarketplaces(r.Context(), s.OrgID)
	if err != nil {
		http.Error(w, "erro interno", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = webtmpl.Render(w, "admin", "admin/marketplaces", struct {
		CSRF, Error string
		Items       []domain.Marketplace
	}{s.CSRF, errText, items})
}
func (c Catalog) Channels(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	var errText string
	if r.Method == http.MethodPost {
		_, err := c.Store.CreateChannel(r.Context(), s.OrgID, r.FormValue("name"), r.FormValue("segment"))
		if err == nil {
			http.Redirect(w, r, "/admin/channels", http.StatusSeeOther)
			return
		}
		errText = err.Error()
	}
	items, err := c.Store.ListChannels(r.Context(), s.OrgID)
	if err != nil {
		http.Error(w, "erro interno", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = webtmpl.Render(w, "admin", "admin/channels", struct {
		CSRF, Error string
		Items       []domain.Channel
	}{s.CSRF, errText, items})
}

func (c Catalog) Links(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	var errText string
	if r.Method == http.MethodPost {
		marketplaceID, err := uuid.Parse(r.FormValue("marketplace_id"))
		if err == nil {
			_, err = c.Store.CreateLink(r.Context(), domain.AffiliateLink{OrgID: s.OrgID, MarketplaceID: marketplaceID, Title: r.FormValue("title"), DestinationURL: r.FormValue("destination_url"), Active: true})
		}
		if err == nil {
			http.Redirect(w, r, "/admin/links", http.StatusSeeOther)
			return
		}
		errText = "Não foi possível criar o link: " + err.Error()
	}
	items, err := c.Store.ListLinks(r.Context(), s.OrgID)
	if err != nil {
		http.Error(w, "erro interno", 500)
		return
	}
	marketplaces, err := c.Store.ListMarketplaces(r.Context(), s.OrgID)
	if err != nil {
		http.Error(w, "erro interno", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = webtmpl.Render(w, "admin", "admin/links", struct {
		CSRF, Error  string
		Items        []domain.AffiliateLink
		Marketplaces []domain.Marketplace
	}{s.CSRF, errText, items, marketplaces})
}
