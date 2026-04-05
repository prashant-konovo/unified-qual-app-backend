package mra

import (
	"log/slog"
	"net/http"

	"github.com/InCrowd/unified-qual-api/internal/handler/support"

	"github.com/InCrowd/unified-qual-api/internal/validate"
	"github.com/go-chi/chi/v5"
)

// ──────────────────────────────────────────────
// MRA Salesforce handlers
// ──────────────────────────────────────────────

// GetAllAccountsMRA handles GET /salesforce/getAllAccounts (MRA).
// Contract-identical: returns {data: [{id, account_name}]}
func (h *Handler) GetAllAccountsMRA(w http.ResponseWriter, r *http.Request) {
	if !h.ProjectService.QsProjectAvailable() {
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "repository not available"})
		return
	}
	records, err := h.ProjectService.GetAllAccountsMRA(r.Context())
	if err != nil {
		slog.Error("get all accounts mra failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	support.WriteJSON(w, http.StatusOK, map[string]any{"data": records})
}

// GetSalesforceClientsMRA handles GET /salesforce/clients (MRA).
// Contract-identical: returns {data: [{accountId, name}]}
func (h *Handler) GetSalesforceClientsMRA(w http.ResponseWriter, r *http.Request) {
	if !h.ProjectService.QsProjectAvailable() {
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "repository not available"})
		return
	}
	records, err := h.ProjectService.GetSalesforceClientsMRA(r.Context())
	if err != nil {
		slog.Error("get salesforce clients mra failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while getting sales force clients",
		})
		return
	}
	support.WriteJSON(w, http.StatusOK, map[string]any{"data": records})
}

// GetSalesforceProjectsMRA handles GET /salesforce/projects/{salesforce_client_id} (MRA).
// Contract-identical: returns {data: [{salesForceProjectId, salesForceProjectName}]}
func (h *Handler) GetSalesforceProjectsMRA(w http.ResponseWriter, r *http.Request) {
	sfClientID := chi.URLParam(r, "salesforce_client_id")

	if !h.ProjectService.QsProjectAvailable() {
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "repository not available"})
		return
	}
	records, err := h.ProjectService.GetSalesforceProjectsMRA(r.Context(), sfClientID)
	if err != nil {
		slog.Error("get salesforce projects mra failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while getting sales force projects",
		})
		return
	}
	support.WriteJSON(w, http.StatusOK, map[string]any{"data": records})
}

// GetSalesforceClientsWithFilterMRA handles POST /salesforce/getSalesforceClientsWithFilter (MRA).
// Contract-identical: returns {data: [{salesforce_account_id, name}]}
func (h *Handler) GetSalesforceClientsWithFilterMRA(w http.ResponseWriter, r *http.Request) {
	if !h.ProjectService.QsProjectAvailable() {
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "repository not available"})
		return
	}

	var body struct {
		ProjectAccountID int `json:"projectAccountId"`
	}
	if errs := validate.DecodeAndValidate(r, &body); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	records, err := h.ProjectService.GetSalesforceClientsWithFilterMRA(r.Context(), body.ProjectAccountID)
	if err != nil {
		slog.Error("get salesforce clients with filter mra failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while getting sales force clients",
		})
		return
	}
	support.WriteJSON(w, http.StatusOK, map[string]any{"data": records})
}
