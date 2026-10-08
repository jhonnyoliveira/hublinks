package admin

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/domain"
	"github.com/hublinks/hublinks/internal/store"
	webtmpl "github.com/hublinks/hublinks/web"
)

type trashItem struct {
	Type, Path, Name string
	ID               uuid.UUID
	Days             int
	// BlockedBy é o nome do marketplace na lixeira que impede restaurar um link.
	BlockedBy string
	deletedAt time.Time
}

type trashData struct {
	CSRF          string
	Items         []trashItem
	RetentionDays int
}

func (c Catalog) trashData(r *http.Request) (trashData, error) {
	s := session(r)
	data := trashData{CSRF: s.CSRF, RetentionDays: c.retention()}
	opts := store.ListOptions{Trash: true}
	now := time.Now()
	add := func(typ, path, name string, id uuid.UUID, at *time.Time, blocked string) {
		it := trashItem{Type: typ, Path: path, Name: name, ID: id, BlockedBy: blocked}
		if at != nil {
			it.deletedAt = *at
			it.Days = daysRemaining(*at, c.retention(), now)
		}
		data.Items = append(data.Items, it)
	}
	marketplaces, _, err := c.Service.Store.ListMarketplacesWithOptions(r.Context(), s.OrgID, opts)
	if err != nil {
		return data, err
	}
	for _, v := range marketplaces {
		add("marketplace", "marketplaces", v.Name, v.ID, v.DeletedAt, "")
	}
	channels, _, err := c.Service.Store.ListChannelsWithOptions(r.Context(), s.OrgID, opts)
	if err != nil {
		return data, err
	}
	for _, v := range channels {
		add("canal", "channels", v.Name, v.ID, v.DeletedAt, "")
	}
	links, _, err := c.Service.Store.ListLinksWithOptions(r.Context(), s.OrgID, opts)
	if err != nil {
		return data, err
	}
	for _, v := range links {
		blocked := ""
		if v.Marketplace.DeletedAt != nil {
			blocked = v.Marketplace.Name
		}
		add("link", "links", v.Title, v.ID, v.DeletedAt, blocked)
	}
	// os mais recentes primeiro; empate por nome mantém a ordem estável
	sort.SliceStable(data.Items, func(i, j int) bool { return data.Items[i].deletedAt.After(data.Items[j].deletedAt) })
	return data, nil
}

// daysRemaining conta os dias inteiros que faltam até o fim do prazo de
// restauração, arredondando para cima (no último dia ainda há restauração).
func daysRemaining(deletedAt time.Time, retentionDays int, now time.Time) int {
	left := deletedAt.Add(time.Duration(retentionDays) * 24 * time.Hour).Sub(now)
	if left <= 0 {
		return 0
	}
	return int((left + 24*time.Hour - 1) / (24 * time.Hour))
}

var trashFiles = []string{"admin/trash.html", "admin/trash_list.html"}

func (c Catalog) Trash(w http.ResponseWriter, r *http.Request) {
	data, err := c.trashData(r)
	if err != nil {
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	renderPage(w, http.StatusOK, data, trashFiles...)
}

func (c Catalog) renderTrashList(w http.ResponseWriter, r *http.Request) {
	data, err := c.trashData(r)
	if err != nil {
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	var buf bufferedWriter
	if err := webtmpl.RenderFragment(&buf, "trash_list", data, trashFiles...); err != nil {
		http.Error(w, "erro ao renderizar página", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.b)
}

func (c Catalog) MarketplaceRestore(w http.ResponseWriter, r *http.Request) {
	c.restore(w, r, "Marketplace restaurado.", c.Service.RestoreMarketplace)
}
func (c Catalog) ChannelRestore(w http.ResponseWriter, r *http.Request) {
	c.restore(w, r, "Canal restaurado.", c.Service.RestoreChannel)
}
func (c Catalog) LinkRestore(w http.ResponseWriter, r *http.Request) {
	c.restore(w, r, "Link restaurado.", c.Service.RestoreLink)
}

// restore restaura um item. Falhas esperadas (já restaurado, prazo vencido,
// marketplace ainda na lixeira) viram aviso e a lista é atualizada.
func (c Catalog) restore(w http.ResponseWriter, r *http.Request, message string, fn func(ctx context.Context, org, id uuid.UUID) error) {
	s := session(r)
	id, err := pathID(r)
	if err == nil {
		err = fn(r.Context(), s.OrgID, id)
	}
	kind, text := "success", message
	switch {
	case err == nil:
	case errors.Is(err, domain.ErrNotFound):
		kind, text = "error", "Item não encontrado, já restaurado ou com o prazo de restauração vencido."
	case errors.Is(err, domain.ErrConflict):
		kind, text = "error", "Restaure antes o marketplace deste link."
	default:
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	if !isHX(r) {
		http.Redirect(w, r, "/admin/lixeira", http.StatusSeeOther)
		return
	}
	trigger(w, text, kind, false)
	c.renderTrashList(w, r)
}
