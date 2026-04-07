package shared

import (
	"context"
	"net/http"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/utilities"
)

// ──────────────────────────────────────────────
// Health
// ──────────────────────────────────────────────

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	dbChecks := h.HealthService.Check(ctx)

	status := "healthy"
	httpCode := http.StatusOK
	for _, v := range dbChecks {
		if v != "ok" && v != "not_configured" {
			status = "degraded"
			httpCode = http.StatusServiceUnavailable
			break
		}
	}

	utilities.WriteJSON(w, httpCode, map[string]any{
		"status":      status,
		"version":     "1.2.0",
		"environment": h.Cfg.Environment,
		"checks":      dbChecks,
	})
}
