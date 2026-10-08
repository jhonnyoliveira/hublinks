package server

import (
	"context"
	"github.com/hublinks/hublinks/internal/admin"
	"github.com/hublinks/hublinks/internal/api"
	"github.com/hublinks/hublinks/internal/auth"
	"github.com/hublinks/hublinks/internal/config"
	"github.com/hublinks/hublinks/internal/events"
	"github.com/hublinks/hublinks/internal/httpx"
	"github.com/hublinks/hublinks/internal/maintenance"
	"github.com/hublinks/hublinks/internal/redirect"
	"github.com/hublinks/hublinks/internal/stats"
	"github.com/hublinks/hublinks/internal/store"
	"github.com/hublinks/hublinks/internal/web"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"net/http"
	"os"
)

func New(pool *pgxpool.Pool, c config.Config) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /healthz", httpx.Health(pool))
	mux.Handle("GET /privacidade", web.Privacy(c))
	sessions := auth.Manager{Pool: pool}
	mux.Handle("GET /admin/login", admin.Login{Pool: pool, Sessions: sessions})
	mux.Handle("POST /admin/login", admin.Login{Pool: pool, Sessions: sessions})
	mux.Handle("POST /admin/logout", sessions.Require(auth.RequireCSRF(admin.Logout(sessions)), false))
	mux.Handle("GET /admin", sessions.Require(http.HandlerFunc(admin.Dashboard), false))
	adminCatalog := admin.Catalog{Store: store.NewCatalog(pool)}
	mux.Handle("GET /admin/marketplaces", sessions.Require(http.HandlerFunc(adminCatalog.Marketplaces), false))
	mux.Handle("POST /admin/marketplaces", sessions.Require(auth.RequireCSRF(http.HandlerFunc(adminCatalog.Marketplaces)), false))
	mux.Handle("GET /admin/channels", sessions.Require(http.HandlerFunc(adminCatalog.Channels), false))
	mux.Handle("POST /admin/channels", sessions.Require(auth.RequireCSRF(http.HandlerFunc(adminCatalog.Channels)), false))
	mux.Handle("GET /admin/links", sessions.Require(http.HandlerFunc(adminCatalog.Links), false))
	mux.Handle("POST /admin/links", sessions.Require(auth.RequireCSRF(http.HandlerFunc(adminCatalog.Links)), false))
	catalogAPI := api.Catalog{Store: store.NewCatalog(pool)}
	mux.Handle("POST /api/v1/marketplaces", sessions.Require(auth.RequireCSRF(http.HandlerFunc(catalogAPI.Marketplace)), true))
	mux.Handle("GET /api/v1/marketplaces", sessions.Require(http.HandlerFunc(catalogAPI.Marketplace), true))
	mux.Handle("DELETE /api/v1/marketplaces/{id}", sessions.Require(auth.RequireCSRF(http.HandlerFunc(catalogAPI.MarketplaceItem)), true))
	mux.Handle("PATCH /api/v1/marketplaces/{id}", sessions.Require(auth.RequireCSRF(http.HandlerFunc(catalogAPI.MarketplaceItem)), true))
	mux.Handle("POST /api/v1/marketplaces/{id}/restore", sessions.Require(auth.RequireCSRF(http.HandlerFunc(catalogAPI.MarketplaceRestore)), true))
	mux.Handle("POST /api/v1/channels", sessions.Require(auth.RequireCSRF(http.HandlerFunc(catalogAPI.Channel)), true))
	mux.Handle("GET /api/v1/channels", sessions.Require(http.HandlerFunc(catalogAPI.Channel), true))
	mux.Handle("DELETE /api/v1/channels/{id}", sessions.Require(auth.RequireCSRF(http.HandlerFunc(catalogAPI.ChannelItem)), true))
	mux.Handle("PATCH /api/v1/channels/{id}", sessions.Require(auth.RequireCSRF(http.HandlerFunc(catalogAPI.ChannelItem)), true))
	mux.Handle("POST /api/v1/channels/{id}/restore", sessions.Require(auth.RequireCSRF(http.HandlerFunc(catalogAPI.ChannelRestore)), true))
	mux.Handle("POST /api/v1/affiliate-links", sessions.Require(auth.RequireCSRF(http.HandlerFunc(catalogAPI.Link)), true))
	mux.Handle("GET /api/v1/affiliate-links", sessions.Require(http.HandlerFunc(catalogAPI.Link), true))
	mux.Handle("DELETE /api/v1/affiliate-links/{id}", sessions.Require(auth.RequireCSRF(http.HandlerFunc(catalogAPI.LinkItem)), true))
	mux.Handle("POST /api/v1/affiliate-links/{id}/restore", sessions.Require(auth.RequireCSRF(http.HandlerFunc(catalogAPI.LinkRestore)), true))
	mux.Handle("GET /api/v1/stats/summary", sessions.Require(http.HandlerFunc(api.Stats{Service: stats.Service{Pool: pool, TZ: c.ReportTZ}}.Summary), true))
	queue := events.NewQueue(c.EventQueueSize, c.EventBatchSize, c.EventFlushInterval, events.DBWriter{Pool: pool})
	go queue.Run(context.Background())
	public := &redirect.Handler{Catalog: store.NewCatalog(pool), Cache: redirect.NewCache(), Queue: queue, Pepper: c.Pepper, BaseURL: c.BaseURL, TrustedProxies: c.TrustedProxies, Bots: redirect.NewBotDetector(c.BotDistinctCodes, c.BotWindow)}
	mux.Handle("GET /", httpx.NewRateLimiter(c.Pepper, c.PublicRateLimit).Middleware(public))
	return httpx.Log(mux, slog.New(slog.NewJSONHandler(os.Stdout, nil)))
}
func Run(ctx context.Context, c config.Config) error {
	pool, err := store.Open(ctx, c.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err = store.Up(ctx, pool); err != nil {
		return err
	}
	if err = store.Bootstrap(ctx, pool, c.OrgName, c.AdminEmail, c.AdminPassword); err != nil {
		return err
	}
	if err = (maintenance.Manager{Pool: pool, Config: c}).CheckTimezone(ctx); err != nil {
		return err
	}
	s := &http.Server{Addr: c.HTTPAddr, Handler: New(pool, c)}
	go func() { <-ctx.Done(); _ = s.Shutdown(context.Background()) }()
	err = s.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}
