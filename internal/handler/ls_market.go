package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/InCrowd/unified-qual-api/internal/validate"
)

// ──────────────────────────────────────────────
// LS Market handlers
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Market Domain
// ──────────────────────────────────────────────

// ListMarkets returns markets with legacy-compatible filtering, pagination, and adminJson.
// Legacy contract: {markets: [adminJson], limit, offset, totalCount}
// Query params: brandId, subscriptionId, accountId, includeAnyProfession, lang, limit, offset
func (h *LSHandler) ListMarkets(w http.ResponseWriter, r *http.Request) {
	if h.irisSurveyRepo == nil {
		writeJSON(w, http.StatusOK, map[string]any{"markets": []any{}, "limit": nil, "offset": nil, "totalCount": 0})
		return
	}

	q := r.URL.Query()
	lang := q.Get("lang")
	if lang == "" {
		lang = "en_us"
	}

	filter := &iris.MarketFilter{
		BrandID: 1,
		Lang:    lang,
	}

	if bid := q.Get("brandId"); bid != "" {
		if v, err := strconv.ParseInt(bid, 10, 64); err == nil {
			filter.BrandID = v
		}
	}

	// subscriptionId takes precedence; accountId resolves to subscription
	if sid := q.Get("subscriptionId"); sid != "" {
		if v, err := strconv.ParseInt(sid, 10, 64); err == nil {
			filter.SubscriptionID = &v
		}
	} else if aid := q.Get("accountId"); aid != "" {
		if v, err := strconv.ParseInt(aid, 10, 64); err == nil {
			if v == 0 {
				filter.SubscriptionID = &v
			} else if subID := h.irisSurveyRepo.GetSubscriptionIDForAccount(r.Context(), v); subID != nil {
				filter.SubscriptionID = subID
			}
		}
	}

	if q.Get("includeAnyProfession") == "true" {
		filter.IncludeAnyProfession = true
		filter.AnyProfessionID = 26 // Constants.marketsIds.anyProfession
	}

	var limitVal, offsetVal any
	if lim := q.Get("limit"); lim != "" {
		if v, err := strconv.Atoi(lim); err == nil {
			filter.Limit = &v
			limitVal = v
		}
	}
	if off := q.Get("offset"); off != "" {
		if v, err := strconv.Atoi(off); err == nil {
			filter.Offset = &v
			offsetVal = v
		}
	}

	markets, totalCount, err := h.irisSurveyRepo.ListMarkets(r.Context(), filter)
	if err != nil {
		slog.Error("list markets failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
		return
	}

	ctx := r.Context()
	result := make([]map[string]any, 0, len(markets))
	for _, m := range markets {
		// Translations
		name := m.Name
		if lang != "en_us" {
			if translated := h.irisSurveyRepo.GetMarketNameTranslation(ctx, m.ID, lang); translated != "" {
				name = translated
			}
		}
		rollup := nullStr(m.Rollup)
		if lang != "en_us" {
			if translated := h.irisSurveyRepo.GetMarketRollupTranslation(ctx, m.ID, lang); translated != "" {
				rollup = translated
			}
		}
		if rollup == nil {
			rollup = ""
		}

		result = append(result, map[string]any{
			"id":                   m.ID,
			"name":                 name,
			"canRegister":          m.CanRegister,
			"exemptFromValidation": m.ExemptFromValidation,
			"rollup":               rollup,
			"isInternal":           m.IsInternal,
			"rewards":              m.Rewards,
			"canInterview":         m.CanInterview,
			"medproValidation":     m.MedproValidation,
			"requiredLicensure":    m.RequiredLicensure,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"markets":    result,
		"limit":      limitVal,
		"offset":     offsetVal,
		"totalCount": totalCount,
	})
}

// ListMarketsNPI returns markets with NPI.
// ListMarketsNPI returns markets with NPI data.
// Contract-identical with legacy InCrowdAPI: GET /v1/markets/npi
// Response: {"markets": [...]}
func (h *LSHandler) ListMarketsNPI(w http.ResponseWriter, r *http.Request) {
	if h.irisSurveyRepo != nil {
		markets, err := h.irisSurveyRepo.ListMarketsWithNPI(r.Context())
		if err != nil {
			slog.Error("list npi markets failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		result := make([]map[string]any, 0, len(markets))
		for _, m := range markets {
			result = append(result, map[string]any{
				"id": m.ID, "name": m.Name, "canInterview": m.CanInterview,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"markets": result})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"markets": []any{}})
}

// GetCrowdableAttributes returns crowdable attributes for a market.
// Contract-identical with legacy InCrowdAPI: GET /v1/market/:id/crowdable_attributes
// Response: {"attributes": [...]}
func (h *LSHandler) GetCrowdableAttributes(w http.ResponseWriter, r *http.Request) {
	marketID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.irisSurveyRepo != nil {
		attrs, err := h.irisSurveyRepo.GetCrowdableAttributes(r.Context(), marketID)
		if err != nil {
			slog.Error("crowdable attrs failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"attributes": attrs})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"attributes": []any{}})
}

// ──────────────────────────────────────────────
// Timeslot Sub-resources (moderators, observers)
// ──────────────────────────────────────────────
