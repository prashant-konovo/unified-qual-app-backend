package router

import (
	"github.com/go-chi/chi/v5"

	"github.com/InCrowd/unified-qual-api/internal/handler"
)

// ══════════════════════════════════════════════════════════════
// Market routes — markets, locales, third-party, eligibility
// ══════════════════════════════════════════════════════════════

func registerMarketRoutes(r chi.Router, hs *handler.Handlers) {
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Get("/markets", hs.LS.ListMarkets)
		r.Get("/markets/npi", hs.LS.ListMarketsNPI)
		r.Get("/market/{id}/crowdable_attributes", hs.LS.GetCrowdableAttributes)
		r.Get("/translations/locales", hs.LS.GetLocalesReal)
		r.Post("/third-party-integrate", hs.LS.ThirdPartyIntegrate)
		r.Post("/qual/eligibility", hs.LS.CheckQualEligibility)
	})
}
