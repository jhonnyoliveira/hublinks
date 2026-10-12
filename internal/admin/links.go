package admin

import (
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/domain"
	"github.com/hublinks/hublinks/internal/service"
	"github.com/hublinks/hublinks/internal/stats"
	webtmpl "github.com/hublinks/hublinks/web"
)

// linkRow é uma linha da listagem: o link e seus contadores no período.
// Clicks e Unique são nil para links não rastreáveis (nunca zero).
type linkRow struct {
	service.LinkView
	HasStats bool
	Clicks   *int
	Unique   *int
	History  *stats.Counts
}

type linkList struct {
	CSRF          string
	Items         []linkRow
	Period        periodView
	StatsError    bool
	Marketplaces  []domain.Marketplace
	Q             string
	MarketplaceID string
	Active        string
	Filtered      bool
	Pager         Pager
}

type linkForm struct {
	CSRF, ID, Title, DestinationURL, ImageURL, MarketplaceID, Policy string
	Active                                                           bool
	Marketplaces                                                     []domain.Marketplace
	Errors                                                           map[string]string
	Error                                                            string
}

type linkShow struct {
	CSRF  string
	Link  service.LinkView
	Flash string
	// Estatísticas do período (nil quando indisponíveis).
	Stats      *linkStats
	StatsError bool
	Period     periodView
}

// linkStats é o detalhe de cliques do link para a tela.
type linkStats struct {
	Trackable bool
	Totals    *stats.Counts
	History   *stats.Counts
	Channels  []channelRow
	Chart     template.JS
	DayRows   [][]string
}

type channelRow struct {
	Name, URL      string
	Clicks, Unique int
}

type linkDelete struct {
	CSRF, Title   string
	ID            uuid.UUID
	RetentionDays int
}

// flashes são as mensagens aceitas em ?ok=, usadas após redirecionamentos.
var flashes = map[string]string{"created": "Link criado.", "updated": "Link atualizado."}

func (c Catalog) linkListData(r *http.Request) (linkList, error) {
	s := session(r)
	q := currentQuery(r)
	opts, page := listOptions(q)
	data := linkList{CSRF: s.CSRF, Q: opts.Query, Filtered: opts.Query != ""}
	if id, err := uuid.Parse(q.Get("marketplace_id")); err == nil {
		opts.MarketplaceID = id
		data.MarketplaceID = id.String()
		data.Filtered = true
	}
	switch q.Get("active") {
	case "true", "false":
		v := q.Get("active") == "true"
		opts.Active = &v
		data.Active = q.Get("active")
		data.Filtered = true
	}
	items, total, err := c.Service.Links(r.Context(), s.OrgID, opts)
	if err != nil {
		return data, err
	}
	if data.Marketplaces, err = c.Service.Store.ListMarketplaces(r.Context(), s.OrgID); err != nil {
		return data, err
	}
	var p stats.Period
	p, data.Period = resolvePeriod(c.Stats, q, "/admin/links", "#links-list")
	data.Items, data.StatsError = c.rows(r, s.OrgID, p, items)
	data.Pager = newPager("/admin/links", q, page, total, "#links-list")
	return data, nil
}

// rows junta os contadores do período às linhas. Sem estatísticas (pool nulo ou
// erro de consulta) as linhas seguem sem contadores e o erro é sinalizado.
func (c Catalog) rows(r *http.Request, org uuid.UUID, p stats.Period, items []service.LinkView) ([]linkRow, bool) {
	rows := make([]linkRow, len(items))
	for i, v := range items {
		rows[i].LinkView = v
	}
	if c.Stats.Pool == nil || len(items) == 0 {
		return rows, false
	}
	ids := make([]uuid.UUID, len(items))
	for i, v := range items {
		ids[i] = v.ID
	}
	totals, err := c.Stats.Totals(r.Context(), org, p, ids)
	if err != nil {
		slog.Error("falha ao consultar cliques dos links", "error", err)
		return rows, true
	}
	for i := range rows {
		rows[i].HasStats = true
		t := totals[rows[i].ID]
		if rows[i].Trackable {
			clicks, unique := t.Clicks, t.Unique
			rows[i].Clicks, rows[i].Unique = &clicks, &unique
		} else if t.Clicks > 0 {
			h := t
			rows[i].History = &h
		}
	}
	return rows, false
}

func (c Catalog) Links(w http.ResponseWriter, r *http.Request) {
	data, err := c.linkListData(r)
	if err != nil {
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	if isHX(r) && r.Header.Get("HX-Target") == "links-list" {
		render(w, http.StatusOK, "", "admin/links", "links_list", data)
		return
	}
	renderPage(w, http.StatusOK, data, "admin/links/index.html", "admin/links/list.html")
}

func renderPage(w http.ResponseWriter, status int, data any, files ...string) {
	var buf bufferedWriter
	if err := webtmpl.RenderPage(&buf, "admin", data, files...); err != nil {
		http.Error(w, "erro ao renderizar página", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.b)
}

func (c Catalog) newLinkForm(r *http.Request) (linkForm, error) {
	s := session(r)
	mps, err := c.Service.Store.ListMarketplaces(r.Context(), s.OrgID)
	return linkForm{CSRF: s.CSRF, Marketplaces: mps, Active: true}, err
}

func (c Catalog) LinkNew(w http.ResponseWriter, r *http.Request) {
	form, err := c.newLinkForm(r)
	if err != nil {
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	if len(form.Marketplaces) == 0 {
		http.Redirect(w, r, "/admin/links", http.StatusSeeOther)
		return
	}
	// ?marketplace_id pré-seleciona o marketplace (atalho a partir da listagem).
	if id, perr := uuid.Parse(r.URL.Query().Get("marketplace_id")); perr == nil {
		form.MarketplaceID = id.String()
	}
	renderPage(w, http.StatusOK, form, "admin/links/form.html")
}

func (c Catalog) LinkEdit(w http.ResponseWriter, r *http.Request) {
	s := session(r)
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	v, err := c.Service.Link(r.Context(), s.OrgID, id, false)
	if err != nil {
		c.linkError(w, r, err)
		return
	}
	form, err := c.newLinkForm(r)
	if err != nil {
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	form.ID, form.Title, form.DestinationURL, form.Active = v.ID.String(), v.Title, v.DestinationURL, v.Active
	form.MarketplaceID = v.Marketplace.ID.String()
	if v.ImageURL != nil {
		form.ImageURL = *v.ImageURL
	}
	if v.ShortenPolicyOverride != nil {
		form.Policy = string(*v.ShortenPolicyOverride)
	}
	renderPage(w, http.StatusOK, form, "admin/links/form.html")
}

// linkError responde a erros de itens (inexistente, sem permissão de org).
func (c Catalog) linkError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, domain.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	http.Error(w, "erro interno", http.StatusInternalServerError)
}

func readLinkForm(r *http.Request, base linkForm) linkForm {
	base.Title = strings.TrimSpace(r.FormValue("title"))
	base.DestinationURL = strings.TrimSpace(r.FormValue("destination_url"))
	base.ImageURL = strings.TrimSpace(r.FormValue("image_url"))
	base.MarketplaceID = r.FormValue("marketplace_id")
	base.Policy = r.FormValue("shorten_policy_override")
	base.Active = r.FormValue("active") != ""
	return base
}

func (c Catalog) LinkCreate(w http.ResponseWriter, r *http.Request) {
	s := session(r)
	form, err := c.newLinkForm(r)
	if err != nil {
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	form = readLinkForm(r, form)
	mpID, perr := uuid.Parse(form.MarketplaceID)
	if perr != nil {
		form.Errors = map[string]string{"marketplace_id": "Selecione um marketplace."}
		renderPage(w, http.StatusUnprocessableEntity, form, "admin/links/form.html")
		return
	}
	link := domain.AffiliateLink{OrgID: s.OrgID, MarketplaceID: mpID, Title: form.Title, DestinationURL: form.DestinationURL, Active: form.Active}
	if form.ImageURL != "" {
		link.ImageURL = &form.ImageURL
	}
	if form.Policy != "" {
		p := domain.Policy(form.Policy)
		link.ShortenPolicyOverride = &p
	}
	created, err := c.Service.CreateLink(r.Context(), link)
	if err != nil {
		c.linkFormError(w, form, err)
		return
	}
	http.Redirect(w, r, "/admin/links/"+created.ID.String()+"?ok=created", http.StatusSeeOther)
}

func (c Catalog) LinkUpdate(w http.ResponseWriter, r *http.Request) {
	s := session(r)
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	form, err := c.newLinkForm(r)
	if err != nil {
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	form.ID = id.String()
	form = readLinkForm(r, form)
	mpID, perr := uuid.Parse(form.MarketplaceID)
	if perr != nil {
		form.Errors = map[string]string{"marketplace_id": "Selecione um marketplace."}
		renderPage(w, http.StatusUnprocessableEntity, form, "admin/links/form.html")
		return
	}
	policy := domain.Policy(form.Policy)
	if err = c.Service.UpdateLink(r.Context(), s.OrgID, id, &form.Title, &form.DestinationURL, &form.ImageURL, &mpID, &policy, &form.Active); err != nil {
		if errors.Is(err, domain.ErrNotFound) && !c.linkExists(r, s.OrgID, id) {
			http.NotFound(w, r)
			return
		}
		c.linkFormError(w, form, err)
		return
	}
	http.Redirect(w, r, "/admin/links/"+id.String()+"?ok=updated", http.StatusSeeOther)
}

func (c Catalog) linkExists(r *http.Request, org, id uuid.UUID) bool {
	_, err := c.Service.Store.GetLink(r.Context(), org, id, false)
	return err == nil
}

func (c Catalog) linkFormError(w http.ResponseWriter, form linkForm, err error) {
	fields, general := formErrors(err)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		// o item existe (verificado antes), então o marketplace escolhido não é válido
		fields = map[string]string{"marketplace_id": "Selecione um marketplace válido."}
		general = ""
	case fields == nil && general == "":
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	form.Errors, form.Error = fields, general
	renderPage(w, http.StatusUnprocessableEntity, form, "admin/links/form.html")
}

func (c Catalog) LinkShow(w http.ResponseWriter, r *http.Request) {
	s := session(r)
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	v, err := c.Service.Link(r.Context(), s.OrgID, id, false)
	if err != nil {
		c.linkError(w, r, err)
		return
	}
	data := linkShow{CSRF: s.CSRF, Link: v, Flash: flashes[r.URL.Query().Get("ok")]}
	var p stats.Period
	p, data.Period = resolvePeriod(c.Stats, r.URL.Query(), "/admin/links/"+v.ID.String(), "#link-stats")
	if c.Stats.Pool != nil {
		if data.Stats, err = c.linkStats(r, s.OrgID, v, p); err != nil {
			slog.Error("falha ao consultar estatísticas do link", "error", err)
			data.StatsError = true
		}
	}
	if isHX(r) && r.Header.Get("HX-Target") == "link-stats" {
		w.Header().Set("Vary", "HX-Request")
		renderFragment(w, "link_stats", data, "admin/links/show.html")
		return
	}
	renderPage(w, http.StatusOK, data, "admin/links/show.html")
}

// LinkToggle liga ou desliga o link (valor explícito em "active", para a ação
// ser idempotente) e devolve a linha atualizada.
func (c Catalog) LinkToggle(w http.ResponseWriter, r *http.Request) {
	s := session(r)
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	active := r.FormValue("active") == "true"
	if err = c.Service.UpdateLink(r.Context(), s.OrgID, id, nil, nil, nil, nil, nil, &active); err != nil {
		c.linkError(w, r, err)
		return
	}
	v, err := c.Service.Link(r.Context(), s.OrgID, id, false)
	if err != nil {
		c.linkError(w, r, err)
		return
	}
	if !isHX(r) {
		http.Redirect(w, r, "/admin/links", http.StatusSeeOther)
		return
	}
	msg := "Link desativado: agora responde “não encontrado”."
	if active {
		msg = "Link ativado."
	}
	trigger(w, msg, "success", false)
	// A linha volta com os contadores do período que o usuário está vendo.
	p, _ := resolvePeriod(c.Stats, currentQuery(r), "/admin/links", "")
	rows, _ := c.rows(r, s.OrgID, p, []service.LinkView{v})
	render(w, http.StatusOK, "", "admin/links", "link_row", rows[0])
}

func (c Catalog) LinkConfirmDelete(w http.ResponseWriter, r *http.Request) {
	s := session(r)
	id, err := pathID(r)
	if err == nil {
		var v service.LinkView
		if v, err = c.Service.Link(r.Context(), s.OrgID, id, false); err == nil {
			render(w, http.StatusOK, "", "admin/links", "link_confirm_delete", linkDelete{CSRF: s.CSRF, ID: v.ID, Title: v.Title, RetentionDays: c.retention()})
			return
		}
	}
	c.linkGone(w, r, err)
}

func (c Catalog) linkGone(w http.ResponseWriter, r *http.Request, err error) {
	if !errors.Is(err, domain.ErrNotFound) {
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	if onDetailPage(r) {
		w.Header().Set("HX-Redirect", "/admin/links")
		return
	}
	data, lerr := c.linkListData(r)
	if lerr != nil {
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	swapInto(w, "#links-list")
	trigger(w, "Link não encontrado ou já excluído.", "error", true)
	render(w, http.StatusOK, "", "admin/links", "links_list", data)
}

// onDetailPage informa se a requisição HTMX partiu da página de um link.
func onDetailPage(r *http.Request) bool {
	raw := r.Header.Get("HX-Current-URL")
	u, err := url.Parse(raw)
	return err == nil && strings.HasPrefix(u.Path, "/admin/links/") && u.Path != "/admin/links/"
}

func (c Catalog) LinkDelete(w http.ResponseWriter, r *http.Request) {
	s := session(r)
	id, err := pathID(r)
	if err == nil {
		err = c.Service.DeleteLink(r.Context(), s.OrgID, id)
	}
	if err != nil {
		c.linkGone(w, r, err)
		return
	}
	if !isHX(r) {
		http.Redirect(w, r, "/admin/links", http.StatusSeeOther)
		return
	}
	if onDetailPage(r) {
		w.Header().Set("HX-Redirect", "/admin/links")
		return
	}
	data, err := c.linkListData(r)
	if err != nil {
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	swapInto(w, "#links-list")
	trigger(w, "Link enviado para a lixeira.", "success", true)
	render(w, http.StatusOK, "", "admin/links", "links_list", data)
}

// linkStats monta o detalhe de cliques: totais, tabela por canal (com a URL de
// cada um) e a série diária.
func (c Catalog) linkStats(r *http.Request, org uuid.UUID, v service.LinkView, p stats.Period) (*linkStats, error) {
	d, err := c.Stats.Link(r.Context(), org, v.ID, p)
	if err != nil {
		return nil, err
	}
	urls := map[uuid.UUID]string{}
	short := ""
	for _, u := range v.URLs {
		if u.Channel != nil {
			urls[u.Channel.ID] = u.URL
		} else if u.Label == "curta" {
			short = u.URL
		}
	}
	out := &linkStats{Trackable: d.Link.Trackable, Totals: d.Totals, History: d.History, Chart: chartData(d.Daily), DayRows: dayRows(d.Daily)}
	for _, ch := range d.ByChannel {
		row := channelRow{Name: "URL curta (sem canal)", URL: short, Clicks: ch.Clicks, Unique: ch.Unique}
		if ch.Channel != nil {
			row.Name, row.URL = ch.Channel.Name, urls[ch.Channel.ID]
		}
		out.Channels = append(out.Channels, row)
	}
	return out, nil
}
