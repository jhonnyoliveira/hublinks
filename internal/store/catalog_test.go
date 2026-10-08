package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/domain"
	"github.com/hublinks/hublinks/internal/store"
	"github.com/hublinks/hublinks/internal/store/testdb"
)

func catalogOrg(t *testing.T, p *store.Catalog) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := p.Pool.Exec(context.Background(), "INSERT INTO organizations(id,name,slug,created_at) VALUES($1,$2,$3,$4)", id, "Organização", id.String(), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestCatalogScopesAndEffectivePolicy(t *testing.T) {
	p := testdb.New(t)
	c := store.NewCatalog(p)
	orgA, orgB := catalogOrg(t, c), catalogOrg(t, c)
	mp, err := c.CreateMarketplace(context.Background(), orgA, "Loja", domain.PolicyDirect)
	if err != nil {
		t.Fatal(err)
	}
	link, err := c.CreateLink(context.Background(), domain.AffiliateLink{OrgID: orgA, MarketplaceID: mp.ID, Title: "Produto", DestinationURL: "https://example.com/produto", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	var inUse domain.MarketplaceInUseError
	if err = c.SoftDeleteMarketplace(context.Background(), orgA, mp.ID); !errors.As(err, &inUse) || len(inUse.Links) != 1 || inUse.Links[0].ID != link.ID {
		t.Fatalf("conflito de marketplace sem links bloqueadores: %#v", err)
	}
	if link.EffectivePolicy() != domain.PolicyDirect || link.Trackable() {
		t.Fatalf("política efetiva incorreta: %#v", link)
	}
	if _, err = c.GetLink(context.Background(), orgB, link.ID, false); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("outra org recebeu link: %v", err)
	}
	if err = c.UpdateMarketplace(context.Background(), orgB, mp.ID, nil, nil); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("outra org alterou marketplace: %v", err)
	}
	if err = c.SoftDeleteLink(context.Background(), orgB, link.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("outra org excluiu link: %v", err)
	}
}

func TestCatalogTrashAndUniqueChannel(t *testing.T) {
	p := testdb.New(t)
	c := store.NewCatalog(p)
	org := catalogOrg(t, c)
	mp, err := c.CreateMarketplace(context.Background(), org, "Loja", domain.PolicyShorten)
	if err != nil {
		t.Fatal(err)
	}
	ch, err := c.CreateChannel(context.Background(), org, "WhatsApp", "wapp")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.CreateChannel(context.Background(), org, "Duplicado", "wapp"); err == nil {
		t.Fatal("segmento duplicado aceito")
	}
	if err = c.SoftDeleteChannel(context.Background(), org, ch.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = c.CreateChannel(context.Background(), org, "Ainda duplicado", "wapp"); err == nil {
		t.Fatal("segmento de canal na lixeira foi liberado")
	}
	links, err := c.ListChannels(context.Background(), org)
	if err != nil || len(links) != 0 {
		t.Fatalf("canal excluído ainda ativo: %d, %v", len(links), err)
	}
	trash, total, err := c.ListChannelsWithOptions(context.Background(), org, store.ListOptions{Trash: true, Page: 1, PerPage: 20})
	if err != nil || total != 1 || len(trash) != 1 {
		t.Fatalf("lixeira de canais: %d/%d, %v", len(trash), total, err)
	}
	if err = c.RestoreChannel(context.Background(), org, ch.ID); err != nil {
		t.Fatal(err)
	}
	link, err := c.CreateLink(context.Background(), domain.AffiliateLink{OrgID: org, MarketplaceID: mp.ID, Title: "Produto", DestinationURL: "https://example.com", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = c.SoftDeleteLink(context.Background(), org, link.ID); err != nil {
		t.Fatal(err)
	}
	if err = c.SoftDeleteMarketplace(context.Background(), org, mp.ID); err != nil {
		t.Fatal(err)
	}
	if err = c.RestoreLink(context.Background(), org, link.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("restaurou link sem marketplace ativo: %v", err)
	}
	if err = c.RestoreMarketplace(context.Background(), org, mp.ID); err != nil {
		t.Fatal(err)
	}
	if err = c.RestoreLink(context.Background(), org, link.ID); err != nil {
		t.Fatal(err)
	}
}
