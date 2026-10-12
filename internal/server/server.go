package server

import (
	"context"
	"errors"
	"github.com/hublinks/hublinks/internal/admin"
	"github.com/hublinks/hublinks/internal/api"
	"github.com/hublinks/hublinks/internal/assets"
	"github.com/hublinks/hublinks/internal/auth"
	"github.com/hublinks/hublinks/internal/config"
	"github.com/hublinks/hublinks/internal/events"
	"github.com/hublinks/hublinks/internal/httpx"
	"github.com/hublinks/hublinks/internal/maintenance"
	"github.com/hublinks/hublinks/internal/metrics"
	"github.com/hublinks/hublinks/internal/redirect"
	"github.com/hublinks/hublinks/internal/service"
	"github.com/hublinks/hublinks/internal/stats"
	"github.com/hublinks/hublinks/internal/store"
	"github.com/hublinks/hublinks/internal/web"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// New monta o handler sem ciclo de vida: o worker de eventos só para com o
// processo. Use Build quando for preciso drená-lo no desligamento.
func New(pool *pgxpool.Pool, c config.Config) http.Handler {
	return Build(context.Background(), pool, c).Handler
}

// App é o servidor montado: o handler HTTP, as métricas e um canal fechado
// quando o worker de eventos termina de drenar a fila (depois que ctx cancela).
type App struct {
	Handler http.Handler
	Metrics *metrics.Metrics
	Drained <-chan struct{}
}

func Build(ctx context.Context, pool *pgxpool.Pool, c config.Config) App {
	mux := http.NewServeMux()
	mux.Handle("GET /healthz", httpx.Health(pool))
	mux.Handle("GET /static/", assets.Handler())
	mux.Handle("GET /privacidade", web.Privacy(c))
	sessions := auth.Manager{Pool: pool}
	login := admin.Login{Pool: pool, Sessions: sessions, Limiter: auth.NewLoginLimiter(c.Pepper, c.LoginMaxFailures, c.LoginWindow, c.LoginLockout).WithTrustedProxies(c.TrustedProxies)}
	mux.Handle("GET /admin/login", login)
	mux.Handle("POST /admin/login", login)
	mux.Handle("POST /admin/logout", sessions.Require(auth.RequireCSRF(admin.Logout(sessions)), false))
	statsService := stats.Service{Pool: pool, TZ: c.ReportTZ}
	mux.Handle("GET /admin", sessions.Require(http.HandlerFunc(admin.Dashboard{Stats: statsService}.Show), false))
	if c.AppEnv == "development" {
		mux.Handle("GET /admin/ui", sessions.Require(http.HandlerFunc(admin.UI), false))
	}
	resolutionCache := redirect.NewCache()
	catalogService := service.Catalog{Store: store.NewCatalog(pool), Cache: resolutionCache, BaseURL: c.BaseURL}
	adminCatalog := admin.Catalog{Service: catalogService, TrashRetentionDays: c.TrashRetentionDays, Stats: statsService}
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
	statsAPI := api.Stats{Service: statsService, BaseURL: c.BaseURL}
	mux.Handle("GET /api/v1/stats/summary", sessions.Require(http.HandlerFunc(statsAPI.Summary), true))
	mux.Handle("GET /api/v1/stats/links", sessions.Require(http.HandlerFunc(statsAPI.Links), true))
	mux.Handle("GET /api/v1/stats/links/{id}", sessions.Require(http.HandlerFunc(statsAPI.Link), true))
	mux.Handle("GET /api/v1/stats/channels", sessions.Require(http.HandlerFunc(statsAPI.Channels), true))
	// Caminhos desconhecidos sob /admin/ e /api/v1/ nunca chegam ao redirecionamento
	// público: exigem sessão e respondem "não encontrado" no formato da área.
	mux.Handle("GET /admin/", sessions.Require(http.NotFoundHandler(), false))
	mux.Handle("GET /api/v1/", sessions.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpx.APIError(w, http.StatusNotFound, "not_found", "não encontrado", nil)
	}), true))
	m := metrics.New()
	queue := events.NewQueue(c.EventQueueSize, c.EventBatchSize, c.EventFlushInterval, events.DBWriter{Pool: pool}, m)
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		queue.Run(ctx)
	}()
	recorder := redirect.EventRecorder{Queue: queue, Pepper: c.Pepper, TrustedProxies: c.TrustedProxies, Bots: redirect.NewBotDetector(c.BotDistinctCodes, c.BotWindow)}
	public := &redirect.Handler{Catalog: store.NewCatalog(pool), Cache: resolutionCache, BaseURL: c.BaseURL, Recorder: recorder}
	mux.Handle("GET /", httpx.NewRateLimiter(c.Pepper, c.PublicRateLimit).WithTrustedProxies(c.TrustedProxies).Middleware(public))
	logger := newLogger(c.LogLevel)
	return App{Handler: httpx.Recover(httpx.Log(crossOrigin(mux), logger), logger), Metrics: m, Drained: drained}
}

// MetricsHandler serve /metrics (formato Prometheus) em porta separada da
// aplicação pública: as métricas nunca ficam expostas pelo proxy do site.
func MetricsHandler(m *metrics.Metrics) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", m.Handler())
	return mux
}

func newLogger(level string) *slog.Logger {
	var l slog.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		l = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l}))
}

// crossOrigin recusa requisições de escrita iniciadas por outro site (por
// Origin e Sec-Fetch-Site) em /admin e /api/v1. As rotas públicas ficam de fora
// de propósito: o beacon e o redirecionamento precisam aceitar outros sites.
func crossOrigin(next http.Handler) http.Handler {
	protected := http.NewCrossOriginProtection().Handler(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p := r.URL.Path; p == "/admin" || strings.HasPrefix(p, "/admin/") || strings.HasPrefix(p, "/api/v1/") {
			protected.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
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
	ln, err := net.Listen("tcp", c.HTTPAddr)
	if err != nil {
		return err
	}
	app := Build(ctx, pool, c)
	if c.MetricsAddr != "" {
		mln, merr := net.Listen("tcp", c.MetricsAddr)
		if merr != nil {
			return merr
		}
		go func() {
			if merr := serve(ctx, mln, &http.Server{Handler: MetricsHandler(app.Metrics), ReadHeaderTimeout: 10 * time.Second}, 5*time.Second); merr != nil {
				slog.Error("servidor de métricas encerrou com erro", "error", merr)
			}
		}()
	}
	err = serve(ctx, ln, &http.Server{
		Handler:           app.Handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}, 15*time.Second)
	// O pool só fecha (defer acima) depois que o worker gravar o que restou.
	<-app.Drained
	return err
}

// serve atende até o contexto ser cancelado e então encerra com Shutdown,
// esperando as requisições em andamento por até grace. Só retorna depois que o
// Shutdown termina, para o processo não sair no meio de uma resposta.
func serve(ctx context.Context, ln net.Listener, s *http.Server, grace time.Duration) error {
	shutdownDone := make(chan error, 1)
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), grace)
		defer cancel()
		shutdownDone <- s.Shutdown(sctx)
	}()
	err := s.Serve(ln)
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return <-shutdownDone
}
