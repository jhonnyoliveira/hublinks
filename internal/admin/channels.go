package admin

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/domain"
)

type channelList struct {
	CSRF  string
	Items []domain.Channel
	Q     string
	Pager Pager
}

type channelForm struct {
	CSRF, ID, Name, Segment string
	Errors                  map[string]string
	Error                   string
}

type channelDelete struct {
	CSRF, Name, Segment string
	ID                  uuid.UUID
	ActiveLinks         int
	RetentionDays       int
}

func (c Catalog) channelListData(r *http.Request) (channelList, error) {
	s := session(r)
	q := currentQuery(r)
	opts, page := listOptions(q)
	items, total, err := c.Service.Store.ListChannelsWithOptions(r.Context(), s.OrgID, opts)
	if err != nil {
		return channelList{}, err
	}
	return channelList{CSRF: s.CSRF, Items: items, Q: opts.Query, Pager: newPager("/admin/channels", q, page, total, "#channels-list")}, nil
}

func (c Catalog) Channels(w http.ResponseWriter, r *http.Request) {
	data, err := c.channelListData(r)
	if err != nil {
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	if isHX(r) && r.Header.Get("HX-Target") == "channels-list" {
		render(w, http.StatusOK, "", "admin/channels", "channels_list", data)
		return
	}
	render(w, http.StatusOK, "admin", "admin/channels", "", data)
}

func (c Catalog) channelSaved(w http.ResponseWriter, r *http.Request, message string) {
	if !isHX(r) {
		http.Redirect(w, r, "/admin/channels", http.StatusSeeOther)
		return
	}
	data, err := c.channelListData(r)
	if err != nil {
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	swapInto(w, "#channels-list")
	trigger(w, message, "success", true)
	render(w, http.StatusOK, "", "admin/channels", "channels_list", data)
}

func (c Catalog) channelGone(w http.ResponseWriter, r *http.Request, err error) {
	if !errors.Is(err, domain.ErrNotFound) {
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	data, lerr := c.channelListData(r)
	if lerr != nil {
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	swapInto(w, "#channels-list")
	trigger(w, "Canal não encontrado ou já excluído.", "error", true)
	render(w, http.StatusOK, "", "admin/channels", "channels_list", data)
}

func (c Catalog) ChannelNew(w http.ResponseWriter, r *http.Request) {
	render(w, http.StatusOK, "", "admin/channels", "channel_form", channelForm{CSRF: session(r).CSRF})
}

func (c Catalog) ChannelEdit(w http.ResponseWriter, r *http.Request) {
	s := session(r)
	id, err := pathID(r)
	if err == nil {
		var ch domain.Channel
		if ch, _, err = c.Service.Store.GetChannel(r.Context(), s.OrgID, id, false); err == nil {
			render(w, http.StatusOK, "", "admin/channels", "channel_form", channelForm{CSRF: s.CSRF, ID: ch.ID.String(), Name: ch.Name, Segment: ch.Segment})
			return
		}
	}
	c.channelGone(w, r, err)
}

func (c Catalog) ChannelCreate(w http.ResponseWriter, r *http.Request) {
	s := session(r)
	form := channelForm{CSRF: s.CSRF, Name: r.FormValue("name"), Segment: r.FormValue("segment")}
	if _, err := c.Service.CreateChannel(r.Context(), s.OrgID, form.Name, form.Segment); err != nil {
		c.channelFormError(w, form, err)
		return
	}
	c.channelSaved(w, r, "Canal criado.")
}

func (c Catalog) ChannelUpdate(w http.ResponseWriter, r *http.Request) {
	s := session(r)
	id, err := pathID(r)
	if err != nil {
		c.channelGone(w, r, err)
		return
	}
	form := channelForm{CSRF: s.CSRF, ID: id.String(), Name: r.FormValue("name"), Segment: r.FormValue("segment")}
	if err = c.Service.UpdateChannel(r.Context(), s.OrgID, id, &form.Name, &form.Segment); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			c.channelGone(w, r, err)
			return
		}
		c.channelFormError(w, form, err)
		return
	}
	c.channelSaved(w, r, "Canal atualizado.")
}

func (c Catalog) channelFormError(w http.ResponseWriter, form channelForm, err error) {
	fields, general := formErrors(err)
	if fields == nil && general == "" {
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	if errors.Is(err, domain.ErrConflict) {
		fields = map[string]string{"segment": "Este segmento já está em uso (inclusive por um canal na lixeira)."}
		general = ""
	}
	form.Errors, form.Error = fields, general
	render(w, http.StatusUnprocessableEntity, "", "admin/channels", "channel_form", form)
}

func (c Catalog) ChannelConfirmDelete(w http.ResponseWriter, r *http.Request) {
	s := session(r)
	id, err := pathID(r)
	if err == nil {
		var (
			ch    domain.Channel
			count int
		)
		if ch, count, err = c.Service.Store.GetChannel(r.Context(), s.OrgID, id, false); err == nil {
			render(w, http.StatusOK, "", "admin/channels", "channel_confirm_delete", channelDelete{CSRF: s.CSRF, ID: ch.ID, Name: ch.Name, Segment: ch.Segment, ActiveLinks: count, RetentionDays: c.retention()})
			return
		}
	}
	c.channelGone(w, r, err)
}

func (c Catalog) ChannelDelete(w http.ResponseWriter, r *http.Request) {
	s := session(r)
	id, err := pathID(r)
	if err == nil {
		err = c.Service.DeleteChannel(r.Context(), s.OrgID, id)
	}
	switch {
	case err == nil:
		c.channelSaved(w, r, "Canal enviado para a lixeira.")
	case errors.Is(err, domain.ErrNotFound):
		c.channelGone(w, r, err)
	default:
		http.Error(w, "erro interno", http.StatusInternalServerError)
	}
}
