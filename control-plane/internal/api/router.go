package api

import (
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/MichelRolandOkoubi/idp-platform/control-plane/internal/auth"
	"github.com/MichelRolandOkoubi/idp-platform/control-plane/internal/orchestrator"
	"github.com/MichelRolandOkoubi/idp-platform/control-plane/internal/repository"
)

type Config struct {
	Logger       *zap.Logger
	Auth         *auth.Service
	Orchestrator *orchestrator.Orchestrator
	DB           *repository.DB
	MLEngineURL  string
}

type Handler struct {
	Config
}

func NewHandler(cfg Config) *Handler {
	return &Handler{Config: cfg}
}

func NewRouter(h *Handler) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))
	r.Use(ZapLogger(h.Logger))
	r.Use(OtelMiddleware())
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Public routes
	r.Get("/health", h.Health)
	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	r.Route("/api/v1", func(r chi.Router) {
		// Auth (public)
		r.Post("/auth/login", h.Login)
		r.Post("/auth/register", h.Register)

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(h.AuthMiddleware)
			r.Use(httprate.LimitByIP(100, 1*time.Minute))

			// Deploy
			r.Post("/deploy", h.Deploy)

			// Applications
			r.Get("/applications", h.ListApplications)
			r.Get("/applications/{name}", h.GetApplication)
			r.Delete("/applications/{name}", h.DeleteApplication)
			r.Post("/applications/{name}/scale", h.Scale)
			r.Get("/applications/{name}/logs", h.GetLogs)

			// Env vars
			r.Get("/applications/{name}/env", h.ListEnv)
			r.Put("/applications/{name}/env", h.SetEnv)
			r.Delete("/applications/{name}/env/{key}", h.DeleteEnv)

			// Cost
			r.Post("/cost/estimate", h.EstimateCost)
			r.Get("/cost/history", h.GetCostHistory)
			r.Get("/cost/anomalies", h.GetAnomalies)

			// Status
			r.Get("/status", h.PlatformStatus)
		})
	})

	return r
}
