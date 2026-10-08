package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/hublinks/hublinks/internal/domain"
	"github.com/hublinks/hublinks/internal/store"
	"github.com/hublinks/hublinks/internal/store/testdb"
)

func names(items []domain.Marketplace) []string {
	out := make([]string, len(items))
	for i, m := range items {
		out[i] = m.Name
	}
	return out
}

func TestListMarketplacesSearchSortAndPagination(t *testing.T) {
	ctx := context.Background()
	c := store.NewCatalog(testdb.New(t))
	org := catalogOrg(t, c)
	for _, n := range []string{"Beta", "alfa", "Gama", "100% Off", "a_b"} {
		if _, err := c.CreateMarketplace(ctx, org, n, domain.PolicyShorten); err != nil {
			t.Fatal(err)
		}
	}

	items, total, err := c.ListMarketplacesWithOptions(ctx, org, store.ListOptions{})
	if err != nil || total != 5 {
		t.Fatal(total, err)
	}
	if got := names(items); got[0] != "100% Off" || got[1] != "a_b" || got[2] != "alfa" {
		t.Fatalf("ordem padrão por nome sem diferenciar caixa: %v", got)
	}
	items, _, _ = c.ListMarketplacesWithOptions(ctx, org, store.ListOptions{Sort: "-name"})
	if got := names(items); got[0] != "Gama" {
		t.Fatalf("ordem descendente: %v", got)
	}
	items, _, _ = c.ListMarketplacesWithOptions(ctx, org, store.ListOptions{Sort: "name; DROP TABLE marketplaces"})
	if len(items) != 5 || names(items)[0] != "100% Off" {
		t.Fatalf("campo de ordenação desconhecido deve cair na ordem padrão: %v", names(items))
	}

	// curingas digitados pelo usuário são literais
	for q, want := range map[string]int{"%": 1, "_": 1, "a_b": 1, "ALFA": 1, "a": 4, "zzz": 0} {
		got, n, err := c.ListMarketplacesWithOptions(ctx, org, store.ListOptions{Query: q})
		if err != nil || n != want || len(got) != want {
			t.Fatalf("busca %q: total=%d itens=%d (esperado %d) err=%v", q, n, len(got), want, err)
		}
	}

	// paginação: o total ignora a página
	page2, total, err := c.ListMarketplacesWithOptions(ctx, org, store.ListOptions{Page: 2, PerPage: 2})
	if err != nil || total != 5 || len(page2) != 2 {
		t.Fatalf("página 2: total=%d itens=%d err=%v", total, len(page2), err)
	}
	last, _, _ := c.ListMarketplacesWithOptions(ctx, org, store.ListOptions{Page: 3, PerPage: 2})
	if len(last) != 1 {
		t.Fatalf("última página: %d itens", len(last))
	}
}

func TestListLinksFiltersTrashAndOrg(t *testing.T) {
	ctx := context.Background()
	c := store.NewCatalog(testdb.New(t))
	orgA, orgB := catalogOrg(t, c), catalogOrg(t, c)
	mpA, _ := c.CreateMarketplace(ctx, orgA, "Loja A", domain.PolicyShorten)
	mpA2, _ := c.CreateMarketplace(ctx, orgA, "Loja A2", domain.PolicyShorten)
	mpB, _ := c.CreateMarketplace(ctx, orgB, "Loja B", domain.PolicyShorten)
	create := func(title string, m domain.Marketplace, active bool) domain.AffiliateLink {
		l, err := c.CreateLink(ctx, domain.AffiliateLink{OrgID: m.OrgID, MarketplaceID: m.ID, Title: title, DestinationURL: "https://example.com/" + title, Active: active})
		if err != nil {
			t.Fatal(err)
		}
		return l
	}
	camisa := create("Camisa", mpA, true)
	create("Calça", mpA2, false)
	create("Camisa B", mpB, true)

	_, n, _ := c.ListLinksWithOptions(ctx, orgA, store.ListOptions{})
	if n != 2 {
		t.Fatalf("a org A só enxerga os próprios links: %d", n)
	}
	if _, n, _ = c.ListLinksWithOptions(ctx, orgA, store.ListOptions{MarketplaceID: mpA2.ID}); n != 1 {
		t.Fatalf("filtro por marketplace: %d", n)
	}
	// marketplace de outra organização não vaza pelo filtro
	if _, n, _ = c.ListLinksWithOptions(ctx, orgA, store.ListOptions{MarketplaceID: mpB.ID}); n != 0 {
		t.Fatalf("filtro com marketplace de outra org: %d", n)
	}
	inactive := false
	if _, n, _ = c.ListLinksWithOptions(ctx, orgA, store.ListOptions{Active: &inactive}); n != 1 {
		t.Fatalf("filtro inativos: %d", n)
	}
	if got, n, _ := c.ListLinksWithOptions(ctx, orgA, store.ListOptions{Query: "camisa"}); n != 1 || got[0].Code == "" || got[0].Marketplace.Name != "Loja A" {
		t.Fatalf("busca por título deve trazer código e marketplace: %d %+v", n, got)
	}
	got, _, err := c.ListLinksWithOptions(ctx, orgA, store.ListOptions{Sort: "title"})
	if err != nil || len(got) != 2 || got[0].Title != "Calça" {
		t.Fatalf("ordenação por título: %v %+v", err, got)
	}

	count, err := c.CountActiveLinksByMarketplace(ctx, orgA, mpA.ID)
	if err != nil || count != 1 {
		t.Fatalf("contagem de links ativos: %d %v", count, err)
	}
	if count, _ = c.CountActiveLinksByMarketplace(ctx, orgA, mpA2.ID); count != 0 {
		t.Fatalf("link inativo não conta como ativo: %d", count)
	}
	if count, _ = c.CountActiveLinksByMarketplace(ctx, orgB, mpA.ID); count != 0 {
		t.Fatalf("a contagem respeita a org: %d", count)
	}

	if err := c.SoftDeleteLink(ctx, orgA, camisa.ID); err != nil {
		t.Fatal(err)
	}
	if _, n, _ = c.ListLinksWithOptions(ctx, orgA, store.ListOptions{}); n != 1 {
		t.Fatalf("a listagem padrão exclui a lixeira: %d", n)
	}
	if _, n, _ = c.ListLinksWithOptions(ctx, orgA, store.ListOptions{Trash: true}); n != 1 {
		t.Fatalf("lixeira: %d", n)
	}
	if err := c.SoftDeleteLink(ctx, orgB, camisa.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("excluir link de outra org deve dar ErrNotFound: %v", err)
	}
}

func TestCreateLinkRollsBackWhenCodesKeepColliding(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	seed := store.NewCatalog(pool)
	org := catalogOrg(t, seed)
	mp, _ := seed.CreateMarketplace(ctx, org, "Loja", domain.PolicyShorten)
	first, err := seed.CreateLink(ctx, domain.AffiliateLink{OrgID: org, MarketplaceID: mp.ID, Title: "Um", DestinationURL: "https://example.com", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	c := store.NewCatalogWithCodeGenerator(pool, func() (string, error) { calls++; return first.Code, nil })
	_, err = c.CreateLink(ctx, domain.AffiliateLink{OrgID: org, MarketplaceID: mp.ID, Title: "Dois", DestinationURL: "https://example.com/2", Active: true})
	if !errors.Is(err, domain.ErrConflict) || calls != 5 {
		t.Fatalf("esperado ErrConflict após 5 tentativas, veio %v após %d", err, calls)
	}
	// a transação desfez o link órfão
	if _, n, _ := seed.ListLinksWithOptions(ctx, org, store.ListOptions{}); n != 1 {
		t.Fatalf("link sem código ficou gravado: %d links", n)
	}
}
