package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/domain"
	"github.com/hublinks/hublinks/internal/service"
	"github.com/hublinks/hublinks/internal/store"
	"github.com/hublinks/hublinks/internal/store/testdb"
)

func trashService(t *testing.T) (service.Catalog, *store.Catalog, uuid.UUID) {
	t.Helper()
	p := testdb.New(t)
	org := uuid.New()
	if _, err := p.Exec(context.Background(), "INSERT INTO organizations(id,name,slug,created_at) VALUES($1,$2,$3,$4)", org, "Org", org.String(), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	catalog := store.NewCatalog(p)
	return service.Catalog{Store: catalog, BaseURL: "https://hub.example"}, catalog, org
}

func TestTrashRules(t *testing.T) {
	s, c, org := trashService(t)
	mp, err := s.CreateMarketplace(context.Background(), org, "Loja", domain.PolicyShorten)
	if err != nil {
		t.Fatal(err)
	}
	link, err := s.CreateLink(context.Background(), domain.AffiliateLink{OrgID: org, MarketplaceID: mp.ID, Title: "Produto", DestinationURL: "https://example.com", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	var blocked domain.MarketplaceInUseError
	if err = s.DeleteMarketplace(context.Background(), org, mp.ID); !errors.As(err, &blocked) || len(blocked.Links) != 1 || blocked.Links[0].ID != link.ID {
		t.Fatalf("marketplace deveria listar link bloqueador: %#v", err)
	}
	if err = s.DeleteLink(context.Background(), org, link.ID); err != nil {
		t.Fatal(err)
	}
	trashed, err := c.GetLink(context.Background(), org, link.ID, true)
	if err != nil || trashed.DeletedAt == nil {
		t.Fatalf("link não entrou na lixeira: %#v, %v", trashed, err)
	}
	if err = s.DeleteMarketplace(context.Background(), org, mp.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.RestoreLink(context.Background(), org, link.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("link restaurado sem marketplace: %v", err)
	}
	if err = s.RestoreMarketplace(context.Background(), org, mp.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.RestoreLink(context.Background(), org, link.ID); err != nil {
		t.Fatal(err)
	}
	active, err := c.GetLink(context.Background(), org, link.ID, false)
	if err != nil || active.DeletedAt != nil {
		t.Fatalf("link não foi restaurado: %#v, %v", active, err)
	}
	if _, err = c.Pool.Exec(context.Background(), "UPDATE affiliate_links SET deleted_at=$2,purged_at=$2 WHERE id=$1", link.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err = s.RestoreLink(context.Background(), org, link.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("item purgado foi restaurado: %v", err)
	}
}

func TestChannelTrashReservesSegmentAndReportsAffectedLinks(t *testing.T) {
	s, c, org := trashService(t)
	mp, err := s.CreateMarketplace(context.Background(), org, "Loja", domain.PolicyShorten)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateLink(context.Background(), domain.AffiliateLink{OrgID: org, MarketplaceID: mp.ID, Title: "Produto", DestinationURL: "https://example.com", Active: true}); err != nil {
		t.Fatal(err)
	}
	ch, err := s.CreateChannel(context.Background(), org, "WhatsApp", "wapp")
	if err != nil {
		t.Fatal(err)
	}
	_, count, err := c.GetChannel(context.Background(), org, ch.ID, false)
	if err != nil || count != 1 {
		t.Fatalf("contagem de links afetados: %d, %v", count, err)
	}
	if err = s.DeleteChannel(context.Background(), org, ch.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateChannel(context.Background(), org, "Duplicado", "wapp"); err == nil {
		t.Fatal("segmento foi liberado na lixeira")
	}
}
