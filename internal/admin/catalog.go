package admin

import (
	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/auth"
	"github.com/hublinks/hublinks/internal/domain"
	"github.com/hublinks/hublinks/internal/service"
	"github.com/hublinks/hublinks/internal/store"
	webtmpl "github.com/hublinks/hublinks/web"
	"net/http"
	"time"
)

type Catalog struct{ Service service.Catalog }

func (c Catalog) Marketplaces(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	var errText string
	if r.Method == http.MethodPost {
		_, err := c.Service.CreateMarketplace(r.Context(), s.OrgID, r.FormValue("name"), domain.Policy(r.FormValue("shorten_policy")))
		if err == nil {
			http.Redirect(w, r, "/admin/marketplaces", http.StatusSeeOther)
			return
		}
		errText = err.Error()
	}
	items, err := c.Service.Store.ListMarketplaces(r.Context(), s.OrgID)
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
		_, err := c.Service.CreateChannel(r.Context(), s.OrgID, r.FormValue("name"), r.FormValue("segment"))
		if err == nil {
			http.Redirect(w, r, "/admin/channels", http.StatusSeeOther)
			return
		}
		errText = err.Error()
	}
	items, err := c.Service.Store.ListChannels(r.Context(), s.OrgID)
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
			_, err = c.Service.CreateLink(r.Context(), domain.AffiliateLink{OrgID: s.OrgID, MarketplaceID: marketplaceID, Title: r.FormValue("title"), DestinationURL: r.FormValue("destination_url"), Active: r.FormValue("active") != ""})
		}
		if err == nil {
			http.Redirect(w, r, "/admin/links", http.StatusSeeOther)
			return
		}
		errText = "Não foi possível criar o link: " + err.Error()
	}
	items, _, err := c.Service.Links(r.Context(), s.OrgID, store.ListOptions{})
	if err != nil {
		http.Error(w, "erro interno", 500)
		return
	}
	marketplaces, err := c.Service.Store.ListMarketplaces(r.Context(), s.OrgID)
	if err != nil {
		http.Error(w, "erro interno", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = webtmpl.Render(w, "admin", "admin/links", struct {
		CSRF, Error  string
		Items        []service.LinkView
		Marketplaces []domain.Marketplace
	}{s.CSRF, errText, items, marketplaces})
}

func (c Catalog) MarketplaceAction(w http.ResponseWriter, r *http.Request) {
	c.action(w, r, func(orgID, id uuid.UUID, restore bool) error {
		if restore {
			return c.Service.RestoreMarketplace(r.Context(), orgID, id)
		}
		return c.Service.DeleteMarketplace(r.Context(), orgID, id)
	})
}
func (c Catalog) ChannelAction(w http.ResponseWriter, r *http.Request) {
	c.action(w, r, func(orgID, id uuid.UUID, restore bool) error {
		if restore {
			return c.Service.RestoreChannel(r.Context(), orgID, id)
		}
		return c.Service.DeleteChannel(r.Context(), orgID, id)
	})
}
func (c Catalog) LinkAction(w http.ResponseWriter, r *http.Request) {
	c.action(w, r, func(orgID, id uuid.UUID, restore bool) error {
		if restore {
			return c.Service.RestoreLink(r.Context(), orgID, id)
		}
		return c.Service.DeleteLink(r.Context(), orgID, id)
	})
}
func (c Catalog) action(w http.ResponseWriter, r *http.Request, fn func(uuid.UUID, uuid.UUID, bool) error) {
	s, _ := auth.FromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err == nil {
		err = fn(s.OrgID, id, r.PathValue("action") == "restore")
	}
	if err != nil {
		http.Error(w, "Não foi possível concluir a ação: "+err.Error(), http.StatusConflict)
		return
	}
	http.Redirect(w, r, "/admin/lixeira", http.StatusSeeOther)
}

type trashItem struct {
	Type, Name string
	ID         uuid.UUID
	Days       int
}

func (c Catalog) Trash(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	items := []trashItem{}
	marketplaces, _, err := c.Service.Store.ListMarketplacesWithOptions(r.Context(), s.OrgID, store.ListOptions{Trash: true})
	if err == nil {
		for _, v := range marketplaces {
			items = append(items, trashItem{"marketplace", v.Name, v.ID, daysRemaining(v.DeletedAt)})
		}
	}
	channels, _, err := c.Service.Store.ListChannelsWithOptions(r.Context(), s.OrgID, store.ListOptions{Trash: true})
	if err == nil {
		for _, v := range channels {
			items = append(items, trashItem{"canal", v.Name, v.ID, daysRemaining(v.DeletedAt)})
		}
	}
	links, _, err := c.Service.Store.ListLinksWithOptions(r.Context(), s.OrgID, store.ListOptions{Trash: true})
	if err == nil {
		for _, v := range links {
			items = append(items, trashItem{"link", v.Title, v.ID, daysRemaining(v.DeletedAt)})
		}
	}
	if err != nil {
		http.Error(w, "erro interno", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = webtmpl.Render(w, "admin", "admin/trash", struct {
		CSRF  string
		Items []trashItem
	}{s.CSRF, items})
}
func daysRemaining(at *time.Time) int {
	if at == nil {
		return 0
	}
	n := int(time.Until(at.Add(30*24*time.Hour)).Hours() / 24)
	if n < 0 {
		return 0
	}
	return n
}
