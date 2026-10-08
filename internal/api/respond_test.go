package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/domain"
	"github.com/hublinks/hublinks/internal/httpx"
)

type envelope struct {
	Error struct {
		Code    string
		Message string
		Fields  map[string]string
	}
	Links []map[string]any
}

func decode(t *testing.T, w *httptest.ResponseRecorder) envelope {
	t.Helper()
	var e envelope
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatalf("corpo não é JSON: %v: %s", err, w.Body.String())
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type: %q", w.Header().Get("Content-Type"))
	}
	return e
}

func TestCatalogErrorMapsDomainErrorsToContractCodes(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"validação", domain.ValidationError{Fields: map[string]string{"name": "obrigatório"}}, 422, "validation_failed"},
		{"validação embrulhada", fmt.Errorf("ctx: %w", domain.ValidationError{Fields: map[string]string{"a": "b"}}), 422, "validation_failed"},
		{"não encontrado", domain.ErrNotFound, 404, "not_found"},
		{"não encontrado embrulhado", fmt.Errorf("x: %w", domain.ErrNotFound), 404, "not_found"},
		{"conflito", domain.ErrConflict, 409, "conflict"},
		{"marketplace em uso", domain.MarketplaceInUseError{Links: []domain.LinkRef{{ID: uuid.New(), Title: "L"}}}, 409, "conflict"},
		{"erro inesperado", errors.New("boom: detalhe interno"), 500, "internal_error"},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		catalogError(w, tc.err)
		e := decode(t, w)
		if w.Code != tc.status || e.Error.Code != tc.code || e.Error.Message == "" {
			t.Errorf("%s: %d %+v", tc.name, w.Code, e)
		}
	}
	// detalhes internos não vazam na resposta 500
	w := httptest.NewRecorder()
	catalogError(w, errors.New("pq: senha do banco"))
	if got := w.Body.String(); json.Valid([]byte(got)) && decode(t, w).Error.Message == "pq: senha do banco" {
		t.Fatal("mensagem interna exposta")
	}
	// campos e links vão no corpo
	w = httptest.NewRecorder()
	catalogError(w, domain.ValidationError{Fields: map[string]string{"segment": "já em uso"}})
	if f := decode(t, w).Error.Fields; f["segment"] != "já em uso" {
		t.Fatalf("fields: %v", f)
	}
	w = httptest.NewRecorder()
	catalogError(w, domain.MarketplaceInUseError{Links: []domain.LinkRef{{ID: uuid.New(), Title: "Produto"}}})
	if l := decode(t, w).Links; len(l) != 1 || l[0]["title"] != "Produto" {
		t.Fatalf("links: %v", l)
	}
}

func TestEnvelopeCodesOfTheContract(t *testing.T) {
	for _, c := range []struct {
		status int
		code   string
	}{{401, "unauthorized"}, {403, "forbidden"}, {422, "validation_failed"}, {429, "rate_limited"}} {
		w := httptest.NewRecorder()
		httpx.APIError(w, c.status, c.code, "mensagem", nil)
		e := decode(t, w)
		if w.Code != c.status || e.Error.Code != c.code || e.Error.Fields != nil {
			t.Errorf("%d %s: %+v", c.status, c.code, e)
		}
	}
}

func TestListOptionsPaginationBounds(t *testing.T) {
	cases := []struct {
		query         string
		page, perPage int
	}{
		{"", 1, 20},
		{"page=3&per_page=50", 3, 50},
		{"per_page=100", 1, 100},
		{"per_page=101", 1, 100},
		{"per_page=9999", 1, 100},
		{"per_page=0", 1, 20},
		{"per_page=-5", 1, 20},
		{"page=0", 1, 20},
		{"page=-2", 1, 20},
		{"page=abc&per_page=xyz", 1, 20},
	}
	for _, c := range cases {
		o := listOptions(httptest.NewRequest(http.MethodGet, "/?"+c.query, nil))
		if o.Page != c.page || o.PerPage != c.perPage {
			t.Errorf("%q: page=%d per_page=%d, esperado %d/%d", c.query, o.Page, o.PerPage, c.page, c.perPage)
		}
	}
	o := listOptions(httptest.NewRequest(http.MethodGet, "/?q=abc&trash=true&active=false&marketplace_id=nao-uuid", nil))
	if o.Query != "abc" || !o.Trash || o.Active == nil || *o.Active || o.MarketplaceID != uuid.Nil {
		t.Fatalf("filtros: %+v", o)
	}
}

func TestListResponseShape(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?page=2&per_page=5", nil)
	got := listResponse(r, []string{"a"}, 11)
	if got["page"] != 2 || got["per_page"] != 5 || got["total"] != 11 || got["items"] == nil {
		t.Fatalf("%v", got)
	}
}
