package handler

import (
	"log/slog"
	"net/http"

	"github.com/InCrowd/unified-qual-api/internal/validate"
)

// ──────────────────────────────────────────────
// LS Timeslot handlers
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Timeslot Sub-resources (moderators, observers)
// ──────────────────────────────────────────────

// GetTimeslotModerators returns moderators assigned to a timeslot.
// Contract-identical with legacy InCrowdAPI: GET /v1/time_slot/:tsId/moderators
// Response: flat array of moderator objects
func (h *LSHandler) GetTimeslotModerators(w http.ResponseWriter, r *http.Request) {
	tsID, err := validate.ParseIDParam(r, "tsId")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	source := resolveSource(r)

	if source == "iris" && h.irisSurveyRepo != nil {
		mods, err := h.irisSurveyRepo.GetModeratorsForTimeSlot(r.Context(), tsID)
		if err != nil {
			slog.Error("get ts mods failed", "error", err)
		}
		writeJSON(w, http.StatusOK, mods)
		return
	}
	if h.qsTimeSlotRepo != nil {
		mods, err := h.qsTimeSlotRepo.GetModerators(r.Context(), tsID)
		if err != nil {
			slog.Error("get qs ts mods failed", "error", err)
		}
		writeJSON(w, http.StatusOK, mods)
		return
	}
	writeJSON(w, http.StatusOK, []any{})
}

// GetTimeslotModeratorOptionsExt returns possible moderators for a timeslot (extended).
// Contract-identical with legacy InCrowdAPI: GET /v1/time_slot/:tsId/moderator_options
// Response: flat array of moderator option objects
func (h *LSHandler) GetTimeslotModeratorOptionsExt(w http.ResponseWriter, r *http.Request) {
	tsID, err := validate.ParseIDParam(r, "tsId")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	source := resolveSource(r)

	if source == "iris" && h.irisSurveyRepo != nil {
		mods, err := h.irisSurveyRepo.GetPossibleModeratorsForTimeSlot(r.Context(), tsID)
		if err != nil {
			slog.Error("get possible mods failed", "error", err)
		}
		writeJSON(w, http.StatusOK, mods)
		return
	}
	if h.qsUserRepo != nil {
		allMods, err := h.qsUserRepo.GetModerators(r.Context())
		if err != nil {
			slog.Error("get qs moderators failed", "error", err)
		}
		assigned := map[int64]bool{}
		if h.qsTimeSlotRepo != nil {
			tsMods, _ := h.qsTimeSlotRepo.GetModerators(r.Context(), tsID)
			for _, m := range tsMods {
				assigned[m.ModeratorID] = true
			}
		}
		result := make([]map[string]any, 0)
		for _, m := range allMods {
			result = append(result, map[string]any{
				"id": m.ID, "firstName": m.FirstName.String, "lastName": m.LastName.String,
				"isAssigned": assigned[m.ID],
			})
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	writeJSON(w, http.StatusOK, []any{})
}

// AssignTimeslotModerator assigns a moderator to a timeslot.
// Contract-identical with legacy InCrowdAPI: POST /v1/time_slot/:tsId/moderators
// Response: flat array of moderator objects (200, not 201)
func (h *LSHandler) AssignTimeslotModerator(w http.ResponseWriter, r *http.Request) {
	tsID, err := validate.ParseIDParam(r, "tsId")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	var req struct {
		ModeratorID int64 `json:"moderatorId"`
		IsHost      bool  `json:"isHost"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}
	source := resolveSource(r)

	if source == "iris" && h.irisSurveyRepo != nil {
		_, err := h.irisSurveyRepo.AssignModeratorToTimeSlot(r.Context(), tsID, req.ModeratorID, req.IsHost)
		if err != nil {
			slog.Error("assign mod failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "assign failed"})
			return
		}
		// Return updated moderators list (legacy returns array)
		mods, err := h.irisSurveyRepo.GetModeratorsForTimeSlot(r.Context(), tsID)
		if err != nil {
			slog.Error("get mods after assign failed", "error", err)
		}
		writeJSON(w, http.StatusOK, mods)
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "not available"})
}

// UnassignTimeslotModerator removes a moderator from a timeslot.
// Contract-identical with legacy InCrowdAPI: DELETE /v1/time_slot/:tsId/moderators/:modId
// Response: flat array of remaining moderator objects
func (h *LSHandler) UnassignTimeslotModerator(w http.ResponseWriter, r *http.Request) {
	tsID, _ := validate.ParseIDParam(r, "tsId")
	modID, _ := validate.ParseIDParam(r, "modId")
	source := resolveSource(r)

	if source == "iris" && h.irisSurveyRepo != nil {
		if err := h.irisSurveyRepo.RemoveModeratorFromTimeSlot(r.Context(), tsID, modID); err != nil {
			slog.Error("unassign mod failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "unassign failed"})
			return
		}
		// Return remaining moderators (legacy returns array)
		mods, err := h.irisSurveyRepo.GetModeratorsForTimeSlot(r.Context(), tsID)
		if err != nil {
			slog.Error("get mods after unassign failed", "error", err)
		}
		writeJSON(w, http.StatusOK, mods)
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "not available"})
}

// GetTimeslotObservers returns observers for a timeslot.
// Contract-identical with legacy InCrowdAPI: GET /v1/time_slot/:tsId/observers
// Response: {"observers": [...]}
func (h *LSHandler) GetTimeslotObservers(w http.ResponseWriter, r *http.Request) {
	tsID, err := validate.ParseIDParam(r, "tsId")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.irisSurveyRepo != nil {
		observers, err := h.irisSurveyRepo.ListObserversForTimeSlot(r.Context(), tsID)
		if err != nil {
			slog.Error("get ts observers failed", "error", err)
		}
		result := make([]map[string]any, 0, len(observers))
		for _, o := range observers {
			result = append(result, map[string]any{
				"id": o.ID, "email": o.Email, "timeSlotId": nullInt64(o.TimeSlotID),
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"observers": result})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"observers": []any{}})
}

// UpdateTimeslotObservers adds/removes observers for a timeslot.
// Contract-identical with legacy InCrowdAPI: PUT /v1/time_slot/:tsId/observers
// Response: {"observers": [...]}
func (h *LSHandler) UpdateTimeslotObservers(w http.ResponseWriter, r *http.Request) {
	tsID, err := validate.ParseIDParam(r, "tsId")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	var req struct {
		ProjectID int64    `json:"projectId"`
		ToAdd     []string `json:"toAdd"`
		ToDelete  []string `json:"toDelete"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	if h.irisSurveyRepo != nil {
		if err := h.irisSurveyRepo.PutObserversForTimeSlot(r.Context(), req.ProjectID, tsID, req.ToAdd, req.ToDelete); err != nil {
			slog.Error("update observers failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
			return
		}
		// Return updated observers list (legacy returns {"observers": [...]})
		observers, err := h.irisSurveyRepo.ListObserversForTimeSlot(r.Context(), tsID)
		if err != nil {
			slog.Error("get observers after update failed", "error", err)
		}
		result := make([]map[string]any, 0, len(observers))
		for _, o := range observers {
			result = append(result, map[string]any{
				"id": o.ID, "email": o.Email, "timeSlotId": nullInt64(o.TimeSlotID),
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"observers": result})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "not available"})
}

// ──────────────────────────────────────────────
// Conference/Meeting extended
// ──────────────────────────────────────────────
