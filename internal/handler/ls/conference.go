package ls

import (
	"log/slog"
	"net/http"
	"strconv"


	"github.com/InCrowd/unified-qual-api/internal/dto"
	"github.com/InCrowd/unified-qual-api/internal/httpkit"
	"github.com/go-chi/chi/v5"
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
	if errs := httpkit.DecodeAndValidate(r, &req); errs != nil {
		httpkit.WriteError(w, errs)
		return
	}
	if req.ConferenceHash == "" {
		req.ConferenceHash = httpkit.ID()[:12]
	}

	if h.ConferenceService.Available() {
		linkID, err := h.ConferenceService.CreateConferenceLink(r.Context(), req.TimeSlotID, projectID, req.ConferenceHash)
		if err != nil {
			slog.Error("create conf link failed", "error", err)
			httpkit.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "create failed"})
			return
		}
		httpkit.WriteJSON(w, http.StatusCreated, map[string]any{
			"id": linkID, "projectId": projectID,
			"timeSlotId": req.TimeSlotID, "conferenceHash": req.ConferenceHash,
		})
		return
	}
	httpkit.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

func (h *Handler) UpdateConferenceLinkHandler(w http.ResponseWriter, r *http.Request) {
	var req dto.LsUpdateConferenceLinkRequest
	if errs := httpkit.DecodeAndValidate(r, &req); errs != nil {
		httpkit.WriteError(w, errs)
		return
	}

	if h.ConferenceService.Available() {
		if err := h.ConferenceService.UpdateConferenceLink(r.Context(), req.TimeSlotID, req.ConferenceHash, req.Pin); err != nil {
			slog.Error("update conf link failed", "error", err)
			httpkit.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
			return
		}
		httpkit.WriteJSON(w, http.StatusOK, map[string]any{"updated": true, "timeSlotId": req.TimeSlotID})
		return
	}
	httpkit.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

func (h *Handler) GetConferenceLinkByTimeSlot(w http.ResponseWriter, r *http.Request) {
	tsID, err := httpkit.ParseIDParam(r, "timeslotId")
	if err != nil {
		httpkit.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.ConferenceService.Available() {
		link, err := h.ConferenceService.GetConferenceLinkByTimeSlotID(r.Context(), tsID)
		if err != nil {
			slog.Error("get conf link failed", "error", err)
			httpkit.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed"})
			return
		}
		if link == nil {
			httpkit.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "conference link not found"})
			return
		}
		httpkit.WriteJSON(w, http.StatusOK, link)
		return
	}
	httpkit.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "conference link not found"})
}
