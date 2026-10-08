package admin

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/domain"
)

type marketplaceList struct {
	CSRF  string
	Items []domain.Marketplace
	Q     string
	Pager Pager
}

type marketplaceForm struct {
	CSRF, ID, Name, Policy string
	Errors                 map[string]string
	Error                  string
}

type marketplaceDelete struct {
	CSRF, Name    string
	ID            uuid.UUID
	Blocking      []domain.LinkRef
	RetentionDays int
}

func (c Catalog) marketplaceListData(r *http.Request) (marketplaceList, error) {
	s := session(r)
	q := currentQuery(r)
	opts, page := listOptions(q)
	items, total, err := c.Service.Store.ListMarketplacesWithOptions(r.Context(), s.OrgID, opts)
	if err != nil {
		return marketplaceList{}, err
	}
	return marketplaceList{CSRF: s.CSRF, Items: items, Q: opts.Query, Pager: newPager("/admin/marketplaces", q, page, total, "#marketplaces-list")}, nil
}

// Marketplaces lista (página completa ou só o fragmento da lista, para busca e
// paginação via HTMX).
func (c Catalog) Marketplaces(w http.ResponseWriter, r *http.Request) {
	data, err := c.marketplaceListData(r)
	if err != nil {
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	if isHX(r) && r.Header.Get("HX-Target") == "marketplaces-list" {
		render(w, http.StatusOK, "", "admin/marketplaces", "marketplaces_list", data)
		return
	}
	render(w, http.StatusOK, "admin", "admin/marketplaces", "", data)
}

// afterWrite responde a uma gravação bem-sucedida: com HTMX devolve a lista
// atualizada no lugar da lista, fecha o modal e mostra o toast; sem HTMX,
// redireciona para a lista.
func (c Catalog) marketplaceSaved(w http.ResponseWriter, r *http.Request, message string) {
	if !isHX(r) {
		http.Redirect(w, r, "/admin/marketplaces", http.StatusSeeOther)
		return
	}
	data, err := c.marketplaceListData(r)
	if err != nil {
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	swapInto(w, "#marketplaces-list")
	trigger(w, message, "success", true)
	render(w, http.StatusOK, "", "admin/marketplaces", "marketplaces_list", data)
}

func (c Catalog) MarketplaceNew(w http.ResponseWriter, r *http.Request) {
	render(w, http.StatusOK, "", "admin/marketplaces", "marketplace_form", marketplaceForm{CSRF: session(r).CSRF, Policy: string(domain.PolicyShorten)})
}

func (c Catalog) MarketplaceEdit(w http.ResponseWriter, r *http.Request) {
	s := session(r)
	id, err := pathID(r)
	if err == nil {
		var m domain.Marketplace
		if m, err = c.Service.Store.GetMarketplace(r.Context(), s.OrgID, id, false); err == nil {
			render(w, http.StatusOK, "", "admin/marketplaces", "marketplace_form", marketplaceForm{CSRF: s.CSRF, ID: m.ID.String(), Name: m.Name, Policy: string(m.ShortenPolicy)})
			return
		}
	}
	c.marketplaceGone(w, r, err)
}

// marketplaceGone trata item inexistente (por exemplo, excluído em outra aba):
// atualiza a lista e avisa, em vez de deixar o modal vazio.
func (c Catalog) marketplaceGone(w http.ResponseWriter, r *http.Request, err error) {
	if !errors.Is(err, domain.ErrNotFound) {
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	data, lerr := c.marketplaceListData(r)
	if lerr != nil {
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	swapInto(w, "#marketplaces-list")
	trigger(w, "Marketplace não encontrado ou já excluído.", "error", true)
	render(w, http.StatusOK, "", "admin/marketplaces", "marketplaces_list", data)
}

func (c Catalog) MarketplaceCreate(w http.ResponseWriter, r *http.Request) {
	s := session(r)
	form := marketplaceForm{CSRF: s.CSRF, Name: r.FormValue("name"), Policy: r.FormValue("shorten_policy")}
	_, err := c.Service.CreateMarketplace(r.Context(), s.OrgID, form.Name, domain.Policy(form.Policy))
	if err == nil {
		c.marketplaceSaved(w, r, "Marketplace criado.")
		return
	}
	c.marketplaceFormError(w, form, err)
}

func (c Catalog) MarketplaceUpdate(w http.ResponseWriter, r *http.Request) {
	s := session(r)
	id, err := pathID(r)
	if err != nil {
		c.marketplaceGone(w, r, err)
		return
	}
	form := marketplaceForm{CSRF: s.CSRF, ID: id.String(), Name: r.FormValue("name"), Policy: r.FormValue("shorten_policy")}
	policy := domain.Policy(form.Policy)
	if err = c.Service.UpdateMarketplace(r.Context(), s.OrgID, id, &form.Name, &policy); err == nil {
		c.marketplaceSaved(w, r, "Marketplace atualizado.")
		return
	}
	if errors.Is(err, domain.ErrNotFound) {
		c.marketplaceGone(w, r, err)
		return
	}
	c.marketplaceFormError(w, form, err)
}

func (c Catalog) marketplaceFormError(w http.ResponseWriter, form marketplaceForm, err error) {
	fields, general := formErrors(err)
	if fields == nil && general == "" {
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	if errors.Is(err, domain.ErrConflict) {
		fields = map[string]string{"name": "Já existe um marketplace com este nome."}
		general = ""
	}
	form.Errors, form.Error = fields, general
	render(w, http.StatusUnprocessableEntity, "", "admin/marketplaces", "marketplace_form", form)
}

func (c Catalog) MarketplaceConfirmDelete(w http.ResponseWriter, r *http.Request) {
	s := session(r)
	id, err := pathID(r)
	if err == nil {
		var m domain.Marketplace
		if m, err = c.Service.Store.GetMarketplace(r.Context(), s.OrgID, id, false); err == nil {
			render(w, http.StatusOK, "", "admin/marketplaces", "marketplace_confirm_delete", marketplaceDelete{CSRF: s.CSRF, ID: m.ID, Name: m.Name, RetentionDays: c.retention()})
			return
		}
	}
	c.marketplaceGone(w, r, err)
}

func (c Catalog) MarketplaceDelete(w http.ResponseWriter, r *http.Request) {
	s := session(r)
	id, err := pathID(r)
	if err != nil {
		c.marketplaceGone(w, r, err)
		return
	}
	err = c.Service.DeleteMarketplace(r.Context(), s.OrgID, id)
	var inUse domain.MarketplaceInUseError
	switch {
	case err == nil:
		c.marketplaceSaved(w, r, "Marketplace enviado para a lixeira.")
	case errors.As(err, &inUse):
		name := ""
		if m, gerr := c.Service.Store.GetMarketplace(r.Context(), s.OrgID, id, false); gerr == nil {
			name = m.Name
		}
		render(w, http.StatusConflict, "", "admin/marketplaces", "marketplace_confirm_delete", marketplaceDelete{CSRF: s.CSRF, ID: id, Name: name, Blocking: inUse.Links, RetentionDays: c.retention()})
	case errors.Is(err, domain.ErrNotFound):
		c.marketplaceGone(w, r, err)
	default:
		http.Error(w, "erro interno", http.StatusInternalServerError)
	}
}
