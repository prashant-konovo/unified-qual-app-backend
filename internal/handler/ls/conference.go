package ls

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/InCrowd/unified-qual-api/internal/dto"
	"github.com/InCrowd/unified-qual-api/internal/utilities"
)

// ──────────────────────────────────────────────
// LS Conference handlers
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Conference Link CRUD (MRA #36, #37)
// ──────────────────────────────────────────────

func (h *Handler) AddConferenceLink(w http.ResponseWriter, r *http.Request) {
	pidStr := chi.URLParam(r, "projectId")
	projectID, _ := strconv.ParseInt(pidStr, 10, 64)

	var req dto.LsAddConferenceLinkRequest
	if errs := utilities.DecodeAndValidate(r, &req); errs != nil {
		utilities.WriteError(w, errs)
		return
	}
	if req.ConferenceHash == "" {
		req.ConferenceHash = utilities.ID()[:12]
	}

	if h.ConferenceService.Available() {
		linkID, err := h.ConferenceService.CreateConferenceLink(r.Context(), req.TimeSlotID, projectID, req.ConferenceHash)
		if err != nil {
			slog.Error("create conf link failed", "error", err)
			utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "create failed"})
			return
		}
		utilities.WriteJSON(w, http.StatusCreated, map[string]any{
			"id": linkID, "projectId": projectID,
			"timeSlotId": req.TimeSlotID, "conferenceHash": req.ConferenceHash,
		})
		return
	}
	utilities.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

func (h *Handler) UpdateConferenceLinkHandler(w http.ResponseWriter, r *http.Request) {
	var req dto.LsUpdateConferenceLinkRequest
	if errs := utilities.DecodeAndValidate(r, &req); errs != nil {
		utilities.WriteError(w, errs)
		return
	}

	if h.ConferenceService.Available() {
		if err := h.ConferenceService.UpdateConferenceLink(r.Context(), req.TimeSlotID, req.ConferenceHash, req.Pin); err != nil {
			slog.Error("update conf link failed", "error", err)
			utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
			return
		}
		utilities.WriteJSON(w, http.StatusOK, map[string]any{"updated": true, "timeSlotId": req.TimeSlotID})
		return
	}
	utilities.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

func (h *Handler) GetConferenceLinkByTimeSlot(w http.ResponseWriter, r *http.Request) {
	tsID, err := utilities.ParseIDParam(r, "timeslotId")
	if err != nil {
		utilities.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.ConferenceService.Available() {
		link, err := h.ConferenceService.GetConferenceLinkByTimeSlotID(r.Context(), tsID)
		if err != nil {
			slog.Error("get conf link failed", "error", err)
			utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed"})
			return
		}
		if link == nil {
			utilities.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "conference link not found"})
			return
		}
		utilities.WriteJSON(w, http.StatusOK, link)
		return
	}
	utilities.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "conference link not found"})
}
