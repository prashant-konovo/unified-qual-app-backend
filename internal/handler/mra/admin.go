package mra

import (
	"github.com/InCrowd/unified-qual-api/internal/dto"
	"github.com/InCrowd/unified-qual-api/internal/httpkit"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"


	"github.com/go-chi/chi/v5"
)

// ──────────────────────────────────────────────
// MRA Admin handlers
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Admin Get All Users (MRA #17)
// ──────────────────────────────────────────────

// ListAdminUsersMRA returns all QS users with comm prefs using legacy JOIN query.
// Contract-identical with legacy QS Tool: GET /qstoolAdmin/get-all-users
// Response: flat array of user rows (result.records from data-api-client)
func (h *Handler) ListAdminUsersMRA(w http.ResponseWriter, r *http.Request) {
	if !h.AdminService.QsAvailable() {
		httpkit.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "no database available",
			"errorMessage": "Error while getting all users",
		})
		return
	}

	cognitoUserID := r.URL.Query().Get("cognitoUserId")

	records, err := h.AdminService.GetAllUsersAdmin(r.Context(), cognitoUserID)
	if err != nil {
		slog.Error("get all users admin failed", "error", err)
		httpkit.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "Error while getting all users",
		})
		return
	}

	// Legacy returns result.records (flat array)
	httpkit.WriteJSON(w, http.StatusOK, records)
}

// GetAllProjectManagersMRA handles GET /project_manager/client/{client_id} (MRA).
// Contract-identical: returns [{firstName, lastName, id}]
// Note: Legacy SQL has client_id=1 hardcoded, ignoring the path parameter.
func (h *Handler) GetAllProjectManagersMRA(w http.ResponseWriter, r *http.Request) {
	if !h.AdminService.QsAvailable() {
		httpkit.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "repository not available"})
		return
	}

	records, err := h.AdminService.GetAllProjectManagersListMRA(r.Context())
	if err != nil {
		slog.Error("get all project managers mra failed", "error", err)
		httpkit.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	httpkit.WriteJSON(w, http.StatusOK, records)
}

// ──────────────────────────────────────────────
// PM Timeslots & Availabilities (MRA #55, #56)
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// PM Timeslots & Availabilities (MRA #55, #56)
// ──────────────────────────────────────────────

// GetPMTimeslotsMRA handles POST /project_manager/get/time_slots/client/{client_id} (MRA).
// Contract-identical: returns timeslots array, or moderatorId-grouped map when pending=true.
func (h *Handler) GetPMTimeslotsMRA(w http.ResponseWriter, r *http.Request) {
	clientIDStr := chi.URLParam(r, "client_id")
	clientID, _ := strconv.ParseInt(clientIDStr, 10, 64)

	if !h.AdminService.TimeSlotAvailable() {
		httpkit.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "repository not available"})
		return
	}

	var body dto.MraGetPMTimeslotsRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}

	ctx := r.Context()

	if body.Pending {
		records, err := h.AdminService.GetAllPendingInterviewsPerProjectMRA(ctx, body.ProjectID, clientID)
		if err != nil {
			slog.Error("get pending interviews per project mra failed", "error", err)
			httpkit.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		// Group by moderatorId
		grouped := map[string]any{}
		for _, rec := range records {
			modID := fmt.Sprintf("%v", rec["moderatorId"])
			arr, ok := grouped[modID].([]map[string]any)
			if !ok {
				arr = []map[string]any{}
			}
			grouped[modID] = append(arr, rec)
		}
		httpkit.WriteJSON(w, http.StatusOK, grouped)
		return
	}

	var records []map[string]any
	var err error
	if body.FilterBy == "Projects" && len(body.FilteredItems) > 0 {
		records, err = h.AdminService.GetPMTimeSlotsByClientIdWithProjectFilterMRA(ctx, clientID, body.FilteredItems)
	} else if body.FilterBy == "Moderators" && len(body.FilteredItems) > 0 {
		records, err = h.AdminService.GetPMTimeSlotsByClientIdWithModeratorFilterMRA(ctx, clientID, body.FilteredItems)
	} else {
		records, err = h.AdminService.GetPMTimeSlotsByClientIdMRA(ctx, clientID)
	}
	if err != nil {
		slog.Error("get pm timeslots mra failed", "error", err)
		httpkit.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	httpkit.WriteJSON(w, http.StatusOK, records)
}

// GetAvailabilitiesForPMMRA handles POST /project_manager/get/availabilities/client/{client_id} (MRA).
// Contract-identical: returns availabilities array with moderator names.
func (h *Handler) GetAvailabilitiesForPMMRA(w http.ResponseWriter, r *http.Request) {
	clientIDStr := chi.URLParam(r, "client_id")
	clientID, _ := strconv.ParseInt(clientIDStr, 10, 64)

	if !h.AdminService.QsAvailable() {
		httpkit.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "repository not available"})
		return
	}

	var body dto.MraGetAvailabilitiesForPMRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}

	ctx := r.Context()

	var records []map[string]any
	var err error
	if body.FilterBy == "Projects" && len(body.FilteredItems) > 0 {
		records, err = h.AdminService.GetAllModeratorsAvailabilityForPMWithProjectFilterMRA(ctx, clientID, body.FilteredItems)
	} else if body.FilterBy == "Moderators" && len(body.FilteredItems) > 0 {
		records, err = h.AdminService.GetAllModeratorsAvailabilityForPMWithModeratorFilterMRA(ctx, clientID, body.FilteredItems)
	} else {
		records, err = h.AdminService.GetAllModeratorsAvailabilityForPMMRA(ctx, clientID)
	}
	if err != nil {
		slog.Error("get availabilities for pm mra failed", "error", err)
		httpkit.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	httpkit.WriteJSON(w, http.StatusOK, records)
}
