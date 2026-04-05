package router

import (
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/InCrowd/unified-qual-api/internal/handler"
	"github.com/InCrowd/unified-qual-api/internal/logger"
	"github.com/InCrowd/unified-qual-api/internal/middleware"
)

// Role presets — reusable middleware closures
var (
	adminOnly       = middleware.RequireRoles("admin")
	adminManager    = middleware.RequireRoles("admin", "manager")
	adminManagerMod = middleware.RequireRoles("admin", "manager", "moderator")
)

func New(hs *handler.Handlers, jwtAuth *middleware.JWTAuth) *chi.Mux {
	r := chi.NewRouter()

	// Middleware
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(logger.Middleware) // structured JSON logging with request ID
	r.Use(chimw.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID", "IC-Auth", "CognitoToken"},
		ExposedHeaders:   []string{"X-Request-ID", "X-Brand"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check (public)
	r.Get("/health", hs.Handler.Health)

	// API v1 routes — matches the frontend axios baseURL suffix /v1
	r.Route("/v1", func(r chi.Router) {
		// Public routes (no JWT required)
		registerPublicAuthRoutes(r, hs)
		registerPublicSurveyRoutes(r, hs)
		registerPublicTimeslotRoutes(r, hs)
		registerPublicConferenceRoutes(r, hs)

		// Authenticated routes (JWT required)
		r.Group(func(r chi.Router) {
			r.Use(jwtAuth.Middleware)
			registerAuthRoutes(r, hs)
			registerProjectRoutes(r, hs)
			registerSurveyRoutes(r, hs)
			registerTimeslotRoutes(r, hs)
			registerModeratorRoutes(r, hs)
			registerParticipantRoutes(r, hs)
			registerBookingRoutes(r, hs)
			registerSubscriptionRoutes(r, hs)
			registerConferenceRoutes(r, hs)
			registerAdminRoutes(r, hs)
			registerUserRoutes(r, hs)
			registerPaymentRoutes(r, hs)
			registerNotificationRoutes(r, hs)
			registerMarketRoutes(r, hs)
		})
	})

	return r
}
