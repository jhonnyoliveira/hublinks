package server

import (
	"context"
	"github.com/hublinks/hublinks/internal/admin"
	"github.com/hublinks/hublinks/internal/api"
	"github.com/hublinks/hublinks/internal/assets"
	"github.com/hublinks/hublinks/internal/auth"
	"github.com/hublinks/hublinks/internal/config"
	"github.com/hublinks/hublinks/internal/events"
	"github.com/hublinks/hublinks/internal/httpx"
	"github.com/hublinks/hublinks/internal/maintenance"
	"github.com/hublinks/hublinks/internal/redirect"
	"github.com/hublinks/hublinks/internal/service"
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
	mux.Handle("GET /static/", assets.Handler())
	mux.Handle("GET /privacidade", web.Privacy(c))
	sessions := auth.Manager{Pool: pool}
	login := admin.Login{Pool: pool, Sessions: sessions, Limiter: auth.NewLoginLimiter(c.Pepper, c.LoginMaxFailures, c.LoginWindow, c.LoginLockout)}
	mux.Handle("GET /admin/login", login)
	mux.Handle("POST /admin/login", login)
	mux.Handle("POST /admin/logout", sessions.Require(auth.RequireCSRF(admin.Logout(sessions)), false))
	mux.Handle("GET /admin", sessions.Require(http.HandlerFunc(admin.Dashboard), false))
	if c.AppEnv == "development" {
		mux.Handle("GET /admin/ui", sessions.Require(http.HandlerFunc(admin.UI), false))
	}
	resolutionCache := redirect.NewCache()
	catalogService := service.Catalog{Store: store.NewCatalog(pool), Cache: resolutionCache, BaseURL: c.BaseURL}
	adminCatalog := admin.Catalog{Service: catalogService, TrashRetentionDays: c.TrashRetentionDays}
	// view exige sessão; write exige sessão e token CSRF (e origem igual ao site).
	view := func(h http.HandlerFunc) http.Handler { return sessions.Require(h, false) }
	write := func(h http.HandlerFunc) http.Handler { return sessions.Require(auth.RequireCSRF(h), false) }
	mux.Handle("GET /admin/marketplaces", view(adminCatalog.Marketplaces))
	mux.Handle("GET /admin/marketplaces/new", view(adminCatalog.MarketplaceNew))
	mux.Handle("POST /admin/marketplaces", write(adminCatalog.MarketplaceCreate))
	mux.Handle("GET /admin/marketplaces/{id}/edit", view(adminCatalog.MarketplaceEdit))
	mux.Handle("POST /admin/marketplaces/{id}", write(adminCatalog.MarketplaceUpdate))
	mux.Handle("GET /admin/marketplaces/{id}/delete", view(adminCatalog.MarketplaceConfirmDelete))
	mux.Handle("POST /admin/marketplaces/{id}/delete", write(adminCatalog.MarketplaceDelete))
	mux.Handle("POST /admin/marketplaces/{id}/restore", write(adminCatalog.MarketplaceRestore))
	mux.Handle("GET /admin/channels", view(adminCatalog.Channels))
	mux.Handle("GET /admin/channels/new", view(adminCatalog.ChannelNew))
	mux.Handle("POST /admin/channels", write(adminCatalog.ChannelCreate))
	mux.Handle("GET /admin/channels/{id}/edit", view(adminCatalog.ChannelEdit))
	mux.Handle("POST /admin/channels/{id}", write(adminCatalog.ChannelUpdate))
	mux.Handle("GET /admin/channels/{id}/delete", view(adminCatalog.ChannelConfirmDelete))
	mux.Handle("POST /admin/channels/{id}/delete", write(adminCatalog.ChannelDelete))
	mux.Handle("POST /admin/channels/{id}/restore", write(adminCatalog.ChannelRestore))
	mux.Handle("GET /admin/links", view(adminCatalog.Links))
	mux.Handle("GET /admin/links/new", view(adminCatalog.LinkNew))
	mux.Handle("POST /admin/links", write(adminCatalog.LinkCreate))
	mux.Handle("GET /admin/links/{id}", view(adminCatalog.LinkShow))
	mux.Handle("GET /admin/links/{id}/edit", view(adminCatalog.LinkEdit))
	mux.Handle("POST /admin/links/{id}", write(adminCatalog.LinkUpdate))
	mux.Handle("POST /admin/links/{id}/toggle", write(adminCatalog.LinkToggle))
	mux.Handle("GET /admin/links/{id}/delete", view(adminCatalog.LinkConfirmDelete))
	mux.Handle("POST /admin/links/{id}/delete", write(adminCatalog.LinkDelete))
	mux.Handle("POST /admin/links/{id}/restore", write(adminCatalog.LinkRestore))
	mux.Handle("GET /admin/lixeira", view(adminCatalog.Trash))
	catalogAPI := api.Catalog{Service: catalogService}
	mux.Handle("POST /api/v1/marketplaces", sessions.Require(auth.RequireCSRF(http.HandlerFunc(catalogAPI.Marketplace)), true))
	mux.Handle("GET /api/v1/marketplaces", sessions.Require(http.HandlerFunc(catalogAPI.Marketplace), true))
	mux.Handle("GET /api/v1/marketplaces/{id}", sessions.Require(http.HandlerFunc(catalogAPI.MarketplaceItem), true))
	mux.Handle("DELETE /api/v1/marketplaces/{id}", sessions.Require(auth.RequireCSRF(http.HandlerFunc(catalogAPI.MarketplaceItem)), true))
	mux.Handle("PATCH /api/v1/marketplaces/{id}", sessions.Require(auth.RequireCSRF(http.HandlerFunc(catalogAPI.MarketplaceItem)), true))
	mux.Handle("POST /api/v1/marketplaces/{id}/restore", sessions.Require(auth.RequireCSRF(http.HandlerFunc(catalogAPI.MarketplaceRestore)), true))
	mux.Handle("POST /api/v1/channels", sessions.Require(auth.RequireCSRF(http.HandlerFunc(catalogAPI.Channel)), true))
	mux.Handle("GET /api/v1/channels", sessions.Require(http.HandlerFunc(catalogAPI.Channel), true))
	mux.Handle("GET /api/v1/channels/{id}", sessions.Require(http.HandlerFunc(catalogAPI.ChannelItem), true))
	mux.Handle("DELETE /api/v1/channels/{id}", sessions.Require(auth.RequireCSRF(http.HandlerFunc(catalogAPI.ChannelItem)), true))
	mux.Handle("PATCH /api/v1/channels/{id}", sessions.Require(auth.RequireCSRF(http.HandlerFunc(catalogAPI.ChannelItem)), true))
	mux.Handle("POST /api/v1/channels/{id}/restore", sessions.Require(auth.RequireCSRF(http.HandlerFunc(catalogAPI.ChannelRestore)), true))
	mux.Handle("POST /api/v1/affiliate-links", sessions.Require(auth.RequireCSRF(http.HandlerFunc(catalogAPI.Link)), true))
	mux.Handle("GET /api/v1/affiliate-links", sessions.Require(http.HandlerFunc(catalogAPI.Link), true))
	mux.Handle("GET /api/v1/affiliate-links/{id}", sessions.Require(http.HandlerFunc(catalogAPI.LinkItem), true))
	mux.Handle("DELETE /api/v1/affiliate-links/{id}", sessions.Require(auth.RequireCSRF(http.HandlerFunc(catalogAPI.LinkItem)), true))
	mux.Handle("PATCH /api/v1/affiliate-links/{id}", sessions.Require(auth.RequireCSRF(http.HandlerFunc(catalogAPI.LinkItem)), true))
	mux.Handle("POST /api/v1/affiliate-links/{id}/restore", sessions.Require(auth.RequireCSRF(http.HandlerFunc(catalogAPI.LinkRestore)), true))
	mux.Handle("GET /api/v1/stats/summary", sessions.Require(http.HandlerFunc(api.Stats{Service: stats.Service{Pool: pool, TZ: c.ReportTZ}}.Summary), true))
	// Caminhos desconhecidos sob /admin/ e /api/v1/ nunca chegam ao redirecionamento
	// público: exigem sessão e respondem "não encontrado" no formato da área.
	mux.Handle("GET /admin/", sessions.Require(http.NotFoundHandler(), false))
	mux.Handle("GET /api/v1/", sessions.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpx.APIError(w, http.StatusNotFound, "not_found", "não encontrado", nil)
	}), true))
	queue := events.NewQueue(c.EventQueueSize, c.EventBatchSize, c.EventFlushInterval, events.DBWriter{Pool: pool})
	go queue.Run(context.Background())
	recorder := redirect.EventRecorder{Queue: queue, Pepper: c.Pepper, TrustedProxies: c.TrustedProxies, Bots: redirect.NewBotDetector(c.BotDistinctCodes, c.BotWindow)}
	public := &redirect.Handler{Catalog: store.NewCatalog(pool), Cache: resolutionCache, BaseURL: c.BaseURL, Recorder: recorder}
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
	if err = (maintenance.Manager{Pool: pool, Config: c}).Start(ctx); err != nil {
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
