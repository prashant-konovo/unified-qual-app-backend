package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/InCrowd/unified-qual-api/internal/validate"
	"github.com/go-chi/chi/v5"
)

// ──────────────────────────────────────────────
// LS Conference handlers
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Conference Link CRUD (MRA #36, #37)
// ──────────────────────────────────────────────

func (h *LSHandler) AddConferenceLink(w http.ResponseWriter, r *http.Request) {
	pidStr := chi.URLParam(r, "projectId")
	projectID, _ := strconv.ParseInt(pidStr, 10, 64)

	var req struct {
		TimeSlotID     int64  `json:"timeSlotId"`
		ConferenceHash string `json:"conferenceHash"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}
	if req.ConferenceHash == "" {
		req.ConferenceHash = id()[:12]
	}

	if h.qsConferenceRepo != nil {
		linkID, err := h.qsConferenceRepo.CreateConferenceLink(r.Context(), req.TimeSlotID, projectID, req.ConferenceHash)
		if err != nil {
			slog.Error("create conf link failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create failed"})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"id": linkID, "projectId": projectID,
			"timeSlotId": req.TimeSlotID, "conferenceHash": req.ConferenceHash,
		})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

func (h *LSHandler) UpdateConferenceLinkHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TimeSlotID     int64  `json:"timeSlotId"`
		ConferenceHash string `json:"conferenceHash"`
		Pin            string `json:"pin"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	if h.qsConferenceRepo != nil {
		if err := h.qsConferenceRepo.UpdateConferenceLink(r.Context(), req.TimeSlotID, req.ConferenceHash, req.Pin); err != nil {
			slog.Error("update conf link failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"updated": true, "timeSlotId": req.TimeSlotID})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

func (h *LSHandler) GetConferenceLinkByTimeSlot(w http.ResponseWriter, r *http.Request) {
	tsID, err := validate.ParseIDParam(r, "timeslotId")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.qsConferenceRepo != nil {
		link, err := h.qsConferenceRepo.GetConferenceLinkByTimeSlotID(r.Context(), tsID)
		if err != nil {
			slog.Error("get conf link failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed"})
			return
		}
		if link == nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "conference link not found"})
			return
		}
		writeJSON(w, http.StatusOK, link)
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "conference link not found"})
}
