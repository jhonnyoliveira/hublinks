package admin

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"github.com/hublinks/hublinks/internal/auth"
	"github.com/hublinks/hublinks/internal/domain"
	"github.com/hublinks/hublinks/internal/store"
	webtmpl "github.com/hublinks/hublinks/web"
)

const perPage = 20

func isHX(r *http.Request) bool { return r.Header.Get("HX-Request") == "true" }

// render escreve uma página completa (layout "admin") ou, com layout vazio, um
// fragmento HTMX. O status só é escrito depois de a renderização terminar sem
// erro, para nunca enviar HTML parcial.
func render(w http.ResponseWriter, status int, layout, dir, name string, data any) {
	var buf bufferedWriter
	if err := webtmpl.RenderDir(&buf, layout, dir, name, data); err != nil {
		slog.Error("falha ao renderizar template do painel", "dir", dir, "name", name, "error", err)
		http.Error(w, "erro ao renderizar página", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.b)
}

type bufferedWriter struct{ b []byte }

func (w *bufferedWriter) Write(p []byte) (int, error) { w.b = append(w.b, p...); return len(p), nil }

// Pager descreve a paginação de uma listagem. Target é o seletor do contêiner
// que o HTMX substitui ao navegar.
type Pager struct {
	Page, Pages, Total int
	PrevURL, NextURL   string
	Target             string
}

func newPager(base string, q url.Values, page, total int, target string) Pager {
	pages := (total + perPage - 1) / perPage
	if pages < 1 {
		pages = 1
	}
	p := Pager{Page: page, Pages: pages, Total: total, Target: target}
	link := func(n int) string {
		v := url.Values{}
		for k, vals := range q {
			if k != "page" {
				v[k] = vals
			}
		}
		v.Set("page", strconv.Itoa(n))
		return base + "?" + v.Encode()
	}
	if page > 1 {
		p.PrevURL = link(page - 1)
	}
	if page < pages {
		p.NextURL = link(page + 1)
	}
	return p
}

// currentQuery devolve a query string da página que o usuário está vendo. Em
// ações de escrita disparadas de um modal, a requisição não carrega q/page; o
// HTMX informa a URL da página em HX-Current-URL.
func currentQuery(r *http.Request) url.Values {
	// Só ações de escrita (modal) herdam a busca da página; um GET (busca,
	// paginação) já traz os próprios parâmetros.
	if isHX(r) && r.Method != http.MethodGet {
		if raw := r.Header.Get("HX-Current-URL"); raw != "" {
			if u, err := url.Parse(raw); err == nil {
				return u.Query()
			}
		}
	}
	return r.URL.Query()
}

func listOptions(q url.Values) (store.ListOptions, int) {
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	return store.ListOptions{Query: q.Get("q"), Sort: q.Get("sort"), Page: page, PerPage: perPage}, page
}

// trigger define o HX-Trigger com toast e, opcionalmente, o fechamento do modal.
func trigger(w http.ResponseWriter, message, kind string, closeModal bool) {
	ev := map[string]any{}
	if message != "" {
		ev["toast"] = map[string]string{"message": message, "kind": kind}
	}
	if closeModal {
		ev["modal-close"] = true
	}
	if len(ev) == 0 {
		return
	}
	b, _ := json.Marshal(ev)
	w.Header().Set("HX-Trigger", string(b))
}

// swapInto redireciona a resposta HTMX para outro contêiner (por exemplo, da
// lista após salvar um formulário que está no modal).
func swapInto(w http.ResponseWriter, selector string) {
	w.Header().Set("HX-Retarget", selector)
	w.Header().Set("HX-Reswap", "outerHTML")
}

// formErrors separa erros de validação por campo do restante. O segundo valor é
// uma mensagem geral para erros que não pertencem a um campo.
func formErrors(err error) (fields map[string]string, general string) {
	var ve domain.ValidationError
	switch {
	case errors.As(err, &ve):
		return ve.Fields, ""
	case errors.Is(err, domain.ErrConflict):
		return nil, "Já existe um item com este valor."
	case errors.Is(err, domain.ErrNotFound):
		return nil, "Item não encontrado ou já excluído."
	}
	return nil, ""
}

// pathID lê {id} da URL; um id malformado equivale a um item inexistente.
func pathID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return uuid.Nil, domain.ErrNotFound
	}
	return id, nil
}

func session(r *http.Request) auth.Session {
	s, _ := auth.FromContext(r.Context())
	return s
}
