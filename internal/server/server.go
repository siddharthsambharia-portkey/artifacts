package server

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/siddharthsambharia-portkey/artifacts/internal/admin"
	"github.com/siddharthsambharia-portkey/artifacts/internal/ai"
	"github.com/siddharthsambharia-portkey/artifacts/internal/auth"
	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/siddharthsambharia-portkey/artifacts/internal/db"
	"github.com/siddharthsambharia-portkey/artifacts/internal/files"
	"github.com/siddharthsambharia-portkey/artifacts/internal/governance"
	"github.com/siddharthsambharia-portkey/artifacts/internal/notify"
	"github.com/siddharthsambharia-portkey/artifacts/internal/ratelimit"
	"github.com/siddharthsambharia-portkey/artifacts/internal/realtime"
	"github.com/siddharthsambharia-portkey/artifacts/internal/sites"
	"github.com/siddharthsambharia-portkey/artifacts/internal/storage"
	"github.com/siddharthsambharia-portkey/artifacts/internal/warehouse"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/nats-io/nats.go"
)

//go:embed static/*
var staticFS embed.FS

const ServerVersion = "0.1.0"

type Server struct {
	cfg      *config.Config
	logger   *slog.Logger
	http     *http.Server
	store    storage.Store
	db       *db.DB
	auth     auth.Authenticator
	deployer *sites.Deployer
	hub      *realtime.Hub
	cache    *sites.DeployCache
	nats     *nats.Conn
}

func New(cfg *config.Config, logger *slog.Logger) (*Server, error) {
	if err := ensureDataDir(cfg); err != nil {
		return nil, err
	}
	store, err := storage.New(cfg)
	if err != nil {
		return nil, err
	}
	database, err := db.Open(cfg)
	if err != nil {
		return nil, err
	}
	if err := database.Migrate(context.Background()); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}
	authenticator, err := auth.NewAuthenticator(cfg, database)
	if err != nil {
		return nil, err
	}
	gov := governance.New(cfg)
	cache := sites.NewDeployCache(store, 512)
	deployer := sites.NewDeployer(cfg, store, database, gov, cache)
	hub := realtime.NewHub(cfg)

	s := &Server{
		cfg: cfg, logger: logger, store: store, db: database,
		auth: authenticator, deployer: deployer, hub: hub, cache: cache,
	}
	nc, err := realtime.ConnectNATS(cfg, hub, logger)
	if err != nil {
		return nil, fmt.Errorf("NATS connect: %w", err)
	}
	s.nats = nc

	s.http = &http.Server{
		Addr:         cfg.Listen,
		Handler:      s.routes(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	return s, nil
}

func (s *Server) routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(s.loggingMiddleware)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Artifact-Version", ServerVersion)
		w.Write([]byte("ok"))
	})

	staticHandler := sites.NewStaticHandler(s.cfg, s.store, s.cache)
	gov := governance.New(s.cfg)
	api := NewAPI(s.cfg, s.db, gov, s.hub)
	fileHandler := files.NewHandler(s.cfg, s.store, s.db)
	aiHandler := ai.NewHandler(s.cfg, s.db)
	whHandler, _ := warehouse.NewHandler(s.cfg, s.db)
	notifyHandler := notify.NewHandler(s.cfg, s.db)
	adminHandler := admin.NewHandler(s.cfg, s.db)

	limiter := ratelimit.New(20, 50)
	aiLimiter := ratelimit.New(5, 10)
	whLimiter := ratelimit.New(2, 5)

	r.Group(func(r chi.Router) {
		r.Use(s.auth.Middleware)
		r.Get("/login", s.auth.LoginHandler)
		if cb, ok := s.auth.(auth.CallbackCapable); ok {
			r.Get("/auth/callback", cb.CallbackHandler)
		}
		r.Get("/artifact.js", s.serveSDK)
		r.Get("/ui.css", s.serveUICSS)
		r.Get("/api/v1/sites", s.listSites)

		r.Group(func(r chi.Router) {
			r.Use(auth.RequireUser)
			r.Use(ratelimit.Middleware(limiter, rateKey))
			r.Mount("/api/v1", api.Routes())
			r.Get("/api/v1/files", fileHandler.List)
			r.Post("/api/v1/files", fileHandler.Upload)
			r.Get("/api/v1/files/{id}", fileHandler.Serve)
			r.With(ratelimit.Middleware(aiLimiter, rateKey)).Post("/api/v1/ai/chat", aiHandler.Chat)
			r.With(ratelimit.Middleware(aiLimiter, rateKey)).Post("/api/v1/ai/image", aiHandler.Image)
			if whHandler != nil {
				r.With(ratelimit.Middleware(whLimiter, rateKey)).Post("/api/v1/warehouse/query", whHandler.Query)
			}
			r.Post("/api/v1/notify/slack", notifyHandler.Slack)
			r.Get("/ws", s.hub.ServeWS)
			r.Get("/api/v1/admin/audit", adminHandler.Audit)
			r.Get("/api/v1/admin/usage", adminHandler.Usage)
			r.Get("/api/v1/admin/config", adminHandler.Config)
			r.Get("/api/v1/admin/stats", adminHandler.Stats)
		})

		r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			if s.cfg.IsApexHost(r.Host) {
				s.serveHome(w, r)
				return
			}
			if s.cfg.IsAdminHost(r.Host) {
				s.serveAdmin(w, r)
				return
			}
			staticHandler.ServeHTTP(w, r)
		})
	})
	return r
}

func rateKey(r *http.Request) string {
	u := auth.UserFromContext(r.Context())
	if u != nil {
		return u.Email
	}
	return r.RemoteAddr
}

func (s *Server) serveSDK(w http.ResponseWriter, r *http.Request) {
	data, err := staticFS.ReadFile("static/artifact.js")
	if err != nil {
		http.Error(w, "SDK not embedded", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/javascript")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(data)
}

func (s *Server) serveUICSS(w http.ResponseWriter, r *http.Request) {
	data, err := staticFS.ReadFile("static/ui.css")
	if err != nil {
		http.Error(w, "design system not embedded", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/css")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(data)
}

func (s *Server) serveHome(w http.ResponseWriter, r *http.Request) {
	data, err := staticFS.ReadFile("static/home.html")
	if err != nil {
		s.listSites(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	w.Write(data)
}

func (s *Server) serveAdmin(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if !governance.New(s.cfg).IsAdmin(u) {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}
	data, err := staticFS.ReadFile("static/admin.html")
	if err != nil {
		http.Error(w, "Admin console not available", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	w.Write(data)
}

func (s *Server) listSites(w http.ResponseWriter, r *http.Request) {
	siteList, err := s.db.ListSites(r.Context())
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if siteList == nil {
		siteList = []db.SiteRecord{}
	}
	writeJSON(w, siteList)
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		s.logger.Info("request",
			"method", r.Method, "path", r.URL.Path, "status", ww.Status(),
			"duration", time.Since(start), "host", r.Host,
		)
	})
}

func (s *Server) ListenAndServe() error {
	s.logger.Info("artifact starting",
		"version", ServerVersion,
		"listen", s.cfg.Listen,
		"domain", s.cfg.Domain,
		"auth", s.cfg.Auth.Mode,
		"governance", s.cfg.Governance.Mode,
	)
	return s.http.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.nats != nil {
		s.nats.Drain()
	}
	return s.http.Shutdown(ctx)
}

func (s *Server) Config() *config.Config       { return s.cfg }
func (s *Server) DB() *db.DB                   { return s.db }
func (s *Server) Store() storage.Store         { return s.store }
func (s *Server) Deployer() *sites.Deployer    { return s.deployer }
func (s *Server) Hub() *realtime.Hub           { return s.hub }

func ensureDataDir(cfg *config.Config) error {
	if cfg.DataDir == "" {
		cfg.DataDir = ".artifact-data"
	}
	return os.MkdirAll(cfg.DataDir, 0755)
}
