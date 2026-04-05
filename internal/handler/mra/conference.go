package mra

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/InCrowd/unified-qual-api/internal/handler/httputil"

	"github.com/InCrowd/unified-qual-api/internal/dto"
)

// ──────────────────────────────────────────────
// MRA Conference handlers
// ──────────────────────────────────────────────

// AddConferenceLinkMRA handles POST /add-conference-link/project/{project_id}/participant_group/{participant_group_id} (MRA).
// Contract-identical with legacy: inserts meeting info per language, inserts conference_invitation,
// updates project.modified_on. All side effects fully implemented.
func (h *Handler) AddConferenceLinkMRA(w http.ResponseWriter, r *http.Request) {
	if !h.ConferenceService.Available() {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "conference repository not available",
			"errorMessage": "conference repository not available",
		})
		return
	}

	projectID, err := dto.ParseIDParam(r, "project_id")
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	participantGroupID, err := dto.ParseIDParam(r, "participant_group_id")
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	var body struct {
		ConferenceLink     string  `json:"conferenceLink"`
		MeetingInformation [][]any `json:"meetingInformation"`
		UserID             any     `json:"userId"`
	}
	if errs := dto.DecodeAndValidate(r, &body); errs != nil {
		dto.WriteError(w, errs)
		return
	}

	var userID int64
	switch v := body.UserID.(type) {
	case float64:
		userID = int64(v)
	case string:
		userID, _ = strconv.ParseInt(v, 10, 64)
	}

	result, err := h.ConferenceService.AddConferenceLinkMRA(
		r.Context(), projectID, participantGroupID, userID, body.ConferenceLink, body.MeetingInformation,
	)
	if err != nil {
		slog.Error("add conference link failed", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": err.Error(),
		})
		return
	}

	// Legacy returns the result of the last addMeetingInfo transaction call
	httputil.WriteJSON(w, http.StatusOK, result)
}

// UpdateConferenceLinkMRA handles PUT /update-conference-link/project/{project_id}/participant-group/{participant_group_id} (MRA).
// Contract-identical with legacy: upserts meeting info per language, updates conference_invitation,
// updates project.modified_on. Side effects fully implemented.
// Legacy also updates pending timeslot calendar events (external Lambda call) — logged but requires integration.
func (h *Handler) UpdateConferenceLinkMRA(w http.ResponseWriter, r *http.Request) {
	if !h.ConferenceService.Available() {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "conference repository not available",
			"errorMessage": "conference repository not available",
		})
		return
	}

	projectID, err := dto.ParseIDParam(r, "project_id")
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	participantGroupID, err := dto.ParseIDParam(r, "participant_group_id")
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	var body struct {
		ConferenceLink     string  `json:"conferenceLink"`
		MeetingInformation [][]any `json:"meetingInformation"`
		UserID             any     `json:"userId"`
	}
	if errs := dto.DecodeAndValidate(r, &body); errs != nil {
		dto.WriteError(w, errs)
		return
	}

	var userID int64
	switch v := body.UserID.(type) {
	case float64:
		userID = int64(v)
	case string:
		userID, _ = strconv.ParseInt(v, 10, 64)
	}

	// Legacy checks which languages already have meeting info to decide UPDATE vs INSERT
	existingLangs, err := h.ConferenceService.GetExistingMeetingLanguagesMRA(r.Context(), projectID)
	if err != nil {
		slog.Error("get existing meeting languages failed", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": err.Error(),
		})
		return
	}

	// Perform the upserts: conference_invitation + project_meeting_translation + project.modified_on
	if err := h.ConferenceService.UpdateConferenceLinkMRA(
		r.Context(), projectID, participantGroupID, userID, body.ConferenceLink, body.MeetingInformation, existingLangs,
	); err != nil {
		slog.Error("update conference link failed", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": err.Error(),
		})
		return
	}

	// Legacy also fetches pending timeslots and updates calendar event communications.
	// This is a post-DB side effect involving external Lambda calls (updateEventCommunicationService).
	// Log the pending count for observability; full calendar event update requires integration.
	pendingCount, _ := h.ConferenceService.GetPendingTimeSlotsCountMRA(r.Context(), projectID)
	if pendingCount > 0 {
		slog.Info("UpdateConferenceLinkMRA: pending timeslots need communication update",
			"projectId", projectID, "pendingCount", pendingCount)
	}

	// Legacy returns JSON.stringify("Done") which serializes as the string "Done"
	httputil.WriteJSON(w, http.StatusOK, "Done")
}

// GetConferenceLinkMRA handles GET /get-conference-link/{participant_group_id} (MRA).
// Contract-identical with legacy: fetches conference link + multi-language meeting information.
// NOTE: Legacy passes participant_group_id but actually uses it as project_id (per code comment).
// Response: {conference_link, meetingInformation: [["en_us","info"],["fr_fr","info"]]}
func (h *Handler) GetConferenceLinkMRA(w http.ResponseWriter, r *http.Request) {
	if !h.ConferenceService.Available() {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "conference repository not available",
			"errorMessage": "conference repository not available",
		})
		return
	}

	// Legacy: participant_group_id from path but used as project_id in query
	projectID, err := dto.ParseIDParam(r, "participant_group_id")
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	records, err := h.ConferenceService.GetConferenceLinkByProjectMRA(r.Context(), projectID)
	if err != nil {
		slog.Error("get conference link failed", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": err.Error(),
		})
		return
	}

	// Build response matching legacy exactly:
	// resObj.conference_link = records[i].conference_link
	// resObj.meetingInformation = [[langCode, meeting_information], ...]
	resObj := map[string]any{}
	var meetingInformation [][]string
	for _, rec := range records {
		langCode, _ := rec["langCode_countryCode"].(string)
		meetingInfo, _ := rec["meeting_information"].(string)
		meetingInformation = append(meetingInformation, []string{langCode, meetingInfo})
		resObj["conference_link"] = rec["conference_link"]
	}
	resObj["meetingInformation"] = meetingInformation

	httputil.WriteJSON(w, http.StatusOK, resObj)
}

// GetConfLinkByTimeSlotMRA handles GET /get-conf-link-time-slot-id/{timeslot_id} (MRA).
// Contract-identical with legacy: returns {conferenceLink} for a timeslot_id.
// No side effects — pure read API.
func (h *Handler) GetConfLinkByTimeSlotMRA(w http.ResponseWriter, r *http.Request) {
	if !h.ConferenceService.Available() {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "conference repository not available",
			"errorMessage": "An error occured while getting conference link by timeslot id",
		})
		return
	}

	tsID, err := dto.ParseIDParam(r, "timeslot_id")
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	result, err := h.ConferenceService.GetConferenceLinkByTimeSlotMRA(r.Context(), tsID)
	if err != nil {
		slog.Error("get conference link by timeslot failed", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while getting conference link by timeslot id",
		})
		return
	}

	// Legacy returns records[0] which is {conferenceLink: "..."}
	// If no records, records[0] would be undefined → JSON.stringify(undefined) = undefined
	// But legacy would throw at .records[0] access, caught → 500
	if result == nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "conference link not found",
			"errorMessage": "An error occured while getting conference link by timeslot id",
		})
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result)
}
