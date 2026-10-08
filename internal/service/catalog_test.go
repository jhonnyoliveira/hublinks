package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/hublinks/hublinks/internal/domain"
	"github.com/hublinks/hublinks/internal/store"
)

type spyCache struct {
	mu    sync.Mutex
	codes []string
	all   int
}

func (c *spyCache) Invalidate(code string) {
	c.mu.Lock()
	c.codes = append(c.codes, code)
	c.mu.Unlock()
}
func (c *spyCache) InvalidateAll() { c.mu.Lock(); c.all++; c.mu.Unlock() }
func (c *spyCache) reset() (codes []string, all int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	codes, all = c.codes, c.all
	c.codes, c.all = nil, 0
	return
}

func TestLinkURLsUseBaseURLAndOnlyActiveChannels(t *testing.T) {
	ctx := context.Background()
	s, _, org := trashService(t)
	s.BaseURL = "https://hub.example/" // barra final não duplica
	mp, _ := s.CreateMarketplace(ctx, org, "Loja", domain.PolicyShorten)
	wapp, _ := s.CreateChannel(ctx, org, "WhatsApp", "wapp")
	tg, _ := s.CreateChannel(ctx, org, "Telegram", "tg")
	v, err := s.CreateLink(ctx, domain.AffiliateLink{OrgID: org, MarketplaceID: mp.ID, Title: "P", DestinationURL: "https://example.com", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	if v.Marketplace.ID != mp.ID || v.Marketplace.Name != "Loja" {
		t.Fatalf("marketplace deve ser {id,name}: %+v", v.Marketplace)
	}
	if len(v.URLs) != 3 || v.URLs[0].URL != "https://hub.example/"+v.Code || v.URLs[0].Label != "curta" {
		t.Fatalf("URLs: %+v", v.URLs)
	}
	got := map[string]bool{}
	for _, u := range v.URLs[1:] {
		got[u.URL] = true
	}
	if !got["https://hub.example/wapp/"+v.Code] || !got["https://hub.example/tg/"+v.Code] {
		t.Fatalf("URLs por canal: %+v", v.URLs)
	}

	// canal na lixeira deixa de gerar URL; a listagem usa os mesmos canais
	if err = s.DeleteChannel(ctx, org, tg.ID); err != nil {
		t.Fatal(err)
	}
	items, total, err := s.Links(ctx, org, store.ListOptions{})
	if err != nil || total != 1 || len(items[0].URLs) != 2 || items[0].URLs[1].Channel.ID != wapp.ID {
		t.Fatalf("listagem: %d %v %+v", total, err, items)
	}

	// política efetiva direct: só a URL original, sem canais
	direct := domain.PolicyDirect
	d, err := s.CreateLink(ctx, domain.AffiliateLink{OrgID: org, MarketplaceID: mp.ID, Title: "D", DestinationURL: "https://example.com/d", ShortenPolicyOverride: &direct, Active: true})
	if err != nil || d.Trackable || len(d.URLs) != 1 || d.URLs[0].Label != "original" || d.EffectivePolicy != domain.PolicyDirect {
		t.Fatalf("link direct: %+v %v", d, err)
	}
}

func TestEveryWriteInvalidatesResolutionCache(t *testing.T) {
	ctx := context.Background()
	s, _, org := trashService(t)
	spy := &spyCache{}
	s.Cache = spy

	mp, _ := s.CreateMarketplace(ctx, org, "Loja", domain.PolicyShorten)
	ch, _ := s.CreateChannel(ctx, org, "WhatsApp", "wapp")
	if _, all := spy.reset(); all != 2 {
		t.Fatalf("criar marketplace e canal invalida tudo: %d", all)
	}
	v, err := s.CreateLink(ctx, domain.AffiliateLink{OrgID: org, MarketplaceID: mp.ID, Title: "P", DestinationURL: "https://example.com", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	title, off := "Novo", false
	steps := []struct {
		name     string
		do       func() error
		wantCode bool
	}{
		{"criar link", func() error { return nil }, true},
		{"editar link", func() error { return s.UpdateLink(ctx, org, v.ID, &title, nil, nil, nil, nil, &off) }, true},
		{"excluir link", func() error { return s.DeleteLink(ctx, org, v.ID) }, true},
		{"restaurar link", func() error { return s.RestoreLink(ctx, org, v.ID) }, true},
		{"editar canal", func() error { n := "Zap"; return s.UpdateChannel(ctx, org, ch.ID, &n, nil) }, false},
		{"excluir canal", func() error { return s.DeleteChannel(ctx, org, ch.ID) }, false},
		{"restaurar canal", func() error { return s.RestoreChannel(ctx, org, ch.ID) }, false},
		{"editar marketplace", func() error { p := domain.PolicyDirect; return s.UpdateMarketplace(ctx, org, mp.ID, nil, &p) }, false},
	}
	for _, st := range steps {
		if st.name == "criar link" {
			continue // já executado acima; o cache foi consumido ao criar
		}
		spy.reset()
		if err := st.do(); err != nil {
			t.Fatalf("%s: %v", st.name, err)
		}
		codes, all := spy.reset()
		if st.wantCode && (len(codes) == 0 || codes[0] != v.Code) {
			t.Fatalf("%s: o código %q deveria ser invalidado: %v", st.name, v.Code, codes)
		}
		if !st.wantCode && all == 0 {
			t.Fatalf("%s: deveria invalidar todo o cache", st.name)
		}
	}

	// escrita que falha não invalida nada
	spy.reset()
	if err := s.UpdateChannel(ctx, org, uuid.New(), &title, nil); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal(err)
	}
	if codes, all := spy.reset(); len(codes) != 0 || all != 0 {
		t.Fatalf("falha não deve invalidar: %v %d", codes, all)
	}
}

// Criar um link e excluir o marketplace ao mesmo tempo nunca pode deixar um
// link fora da lixeira apontando para um marketplace na lixeira (FR-008c).
func TestMarketplaceDeleteRacesWithLinkCreation(t *testing.T) {
	ctx := context.Background()
	s, c, org := trashService(t)
	for round := 0; round < 15; round++ {
		mp, err := s.CreateMarketplace(ctx, org, "Loja "+uuid.NewString()[:8], domain.PolicyShorten)
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		var createErr, deleteErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, createErr = s.CreateLink(ctx, domain.AffiliateLink{OrgID: org, MarketplaceID: mp.ID, Title: "P", DestinationURL: "https://example.com", Active: true})
		}()
		go func() { defer wg.Done(); deleteErr = s.DeleteMarketplace(ctx, org, mp.ID) }()
		wg.Wait()

		_, linksNow, _ := c.ListLinksWithOptions(ctx, org, store.ListOptions{MarketplaceID: mp.ID})
		_, err = c.GetMarketplace(ctx, org, mp.ID, false)
		inTrash := errors.Is(err, domain.ErrNotFound)
		if inTrash && linksNow > 0 {
			t.Fatalf("rodada %d: marketplace na lixeira com %d link(s) ativos (create=%v delete=%v)", round, linksNow, createErr, deleteErr)
		}
		if createErr == nil && deleteErr == nil {
			t.Fatalf("rodada %d: as duas operações não podem ter vencido", round)
		}
	}
}
