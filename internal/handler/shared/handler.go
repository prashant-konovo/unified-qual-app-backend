package shared

import (
	"context"
	"net/http"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/service"
	"github.com/InCrowd/unified-qual-api/internal/handler/httputil"
)

// Handler holds the remaining (non-auth) handler methods.
// It embeds *Deps so all promoted fields (cfg, db, repos, etc.) are accessible.
type Handler struct{ *service.Deps }

// ──────────────────────────────────────────────
// Health
// ──────────────────────────────────────────────

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	dbChecks := h.DB.HealthCheck(ctx)

	status := "healthy"
	httpCode := http.StatusOK
	for _, v := range dbChecks {
		if v != "ok" && v != "not_configured" {
			status = "degraded"
			httpCode = http.StatusServiceUnavailable
			break
		}
	}

	httputil.WriteJSON(w, httpCode, map[string]any{
		"status":      status,
		"version":     "1.2.0",
		"environment": h.Cfg.Environment,
		"checks":      dbChecks,
	})
}
