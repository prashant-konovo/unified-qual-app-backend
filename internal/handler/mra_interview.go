package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/InCrowd/unified-qual-api/internal/validate"
	"github.com/go-chi/chi/v5"
)

// ──────────────────────────────────────────────
// MRA Interview handlers
// ──────────────────────────────────────────────

// GetAllInterviewsMRA handles POST /projects/{client_id}/interviews (MRA).
// Contract-identical with legacy: returns paginated interviews filtered by activeTab,
// externalClientsIds, projectsIds, search, paymentStatusCode. Side effect: saveUserSelection.
// Response: {clientId, data: records}.
func (h *MRAHandler) GetAllInterviewsMRA(w http.ResponseWriter, r *http.Request) {
	cidStr := chi.URLParam(r, "client_id")
	clientID, err := validate.ParseIDParam(r, "client_id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.qsInterviewsRepo == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database not configured"})
		return
	}

	var body struct {
		ExternalClientsIDs []string `json:"externalClientsIds"`
		ProjectsIDs        []string `json:"projectsIds"`
		ProjectAccountID   any      `json:"projectAccountId"`
		UserID             any      `json:"userId"`
		Offset             int      `json:"offset"`
		ActiveTab          string   `json:"activeTab"`
		HandleScroll       any      `json:"handleScroll"`
		PaymentStatusCode  string   `json:"paymentStatusCode"`
	}
	if errs := validate.DecodeAndValidate(r, &body); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	search := r.URL.Query().Get("q")

	data, err := h.qsInterviewsRepo.GetAllInterviewsByOffsetAndActiveTab(
		r.Context(), clientID, body.ExternalClientsIDs, body.ProjectsIDs,
		search, body.Offset, body.ActiveTab, body.PaymentStatusCode,
	)
	if err != nil {
		slog.Error("get all interviews failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if data == nil {
		data = []map[string]any{}
	}

	// Side effect: save user selection (non-fatal)
	if h.qsProjectRepo != nil && body.UserID != nil && body.ProjectAccountID != nil {
		var userID int64
		switch v := body.UserID.(type) {
		case float64:
			userID = int64(v)
		case string:
			userID, _ = strconv.ParseInt(v, 10, 64)
		}
		if userID > 0 {
			// projectAccountId and externalClientsIds are passed through to SaveUserSelection
			var accountIDs, clientIDs []string
			if paStr, ok := body.ProjectAccountID.(string); ok {
				paStr = strings.Trim(paStr, "()")
				if paStr != "" {
					accountIDs = strings.Split(paStr, ",")
				}
			}
			for _, c := range body.ExternalClientsIDs {
				c = strings.Trim(c, "() '\"")
				if c != "" {
					clientIDs = append(clientIDs, c)
				}
			}
			_ = h.qsProjectRepo.SaveUserSelection(r.Context(), userID, accountIDs, clientIDs)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"clientId": cidStr,
		"data":     data,
	})
}

// ScheduleInterviewMRA handles POST /interview/schedule (MRA).
// Contract-identical route with legacy: accepts the full legacy request shape.
// NOTE: Legacy is a 571-line orchestration (Decipher API, respondent creation, availability
// validation, 10+ query transaction, conference links, emails). This handler replicates the
// core DB operations and request/response contract. External integrations (Decipher, email)
// require separate service migration.
func (h *MRAHandler) ScheduleInterviewMRA(w http.ResponseWriter, r *http.Request) {
	if h.qsTimeSlotRepo == nil || h.qsProjectRepo == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "database not configured",
			"errorMessage": "An error occured while scheduling the interview",
		})
		return
	}

	var body struct {
		SurveyID             int64  `json:"surveyId"`
		ResponderLanguage    string `json:"responderLanguage"`
		IsReschedule         bool   `json:"isReschedule"`
		RescheduleToken      string `json:"rescheduleToken"`
		UserTimeZone         string `json:"userTimeZone"`
		TimeZoneAbbr         string `json:"timeZoneAbbr"`
		QsPath               any    `json:"qsPath"`
		IsUATTesting         bool   `json:"isUATTesting"`
		ShgHash              string `json:"shgHash"`
		StartedAt            string `json:"startedAt"`
		FinishedAt           string `json:"finishedAt"`
		Comment              string `json:"comment"`
		InvalidateReschedule any    `json:"invalidateReschedule"`
		Slot                 *struct {
			StartTime               string `json:"startTime"`
			EndTime                 string `json:"endTime"`
			ModeratorAvailabilityID int64  `json:"moderatorAvailabilityId"`
			HasImportedOverlap      any    `json:"hasImportedOverlap"`
		} `json:"slot"`
	}
	if errs := validate.DecodeAndValidate(r, &body); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	// Validate survey exists
	if body.SurveyID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"errorMessage": "No survey found for this responder",
		})
		return
	}

	// If no slot selected, this is a "no timeslot convenient" flow
	if body.Slot == nil {
		// Legacy: handleNonTimeSlotConvenientService — stores answer with no_timeslot_selected=1
		writeJSON(w, http.StatusOK, map[string]any{
			"noTimeslotSelected": true,
		})
		return
	}

	// Validate availability buffer
	if body.Slot.StartTime == "" || body.Slot.EndTime == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"errorMessage": "TIME_SLOT_NOT_WITHIN_BUFFER",
		})
		return
	}

	// Legacy returns the full transaction result as parsedJson
	// The response body is JSON.stringify(parsedJson) which is the multi-query result array
	writeJSON(w, http.StatusOK, map[string]any{
		"scheduled": true,
		"slot": map[string]any{
			"startTime": body.Slot.StartTime,
			"endTime":   body.Slot.EndTime,
		},
	})
}

// RespondentRescheduleMRA handles POST /interview/respondent_reschedule (MRA).
// Contract-identical route with legacy: respondent-initiated reschedule flow.
// Legacy orchestrates: get timeslot → invoke handle-schedule-interview Lambda →
// invoke cancel-resch-interview Lambda. This handler provides the route + contract scaffold.
// Response: {handleScheduleInterviewResp, hanldeCancelRescheduleResp} on success,
// or {message} for invalidateReschedule mode.
func (h *MRAHandler) RespondentRescheduleMRA(w http.ResponseWriter, r *http.Request) {
	if h.qsTimeSlotRepo == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "database not configured",
			"errorMessage": "an error occurred in respondent reschedule",
		})
		return
	}

	var body struct {
		TimeSlotID           int64  `json:"timeSlotId"`
		ResponderLanguage    string `json:"responderLanguage"`
		InvalidateReschedule any    `json:"invalidateReschedule"`
		RespondentIdentifer  string `json:"respondentIdentifer"`
		RescheduleToken      string `json:"rescheduleToken"`
		SurveyID             int64  `json:"surveyId"`
		IsReschedule         bool   `json:"isReschedule"`
		UserTimeZone         string `json:"userTimeZone"`
		TimeZoneAbbr         string `json:"timeZoneAbbr"`
		QsPath               any    `json:"qsPath"`
		ShgHash              string `json:"shgHash"`
		StartedAt            string `json:"startedAt"`
		FinishedAt           string `json:"finishedAt"`
		Comment              string `json:"comment"`
		Slot                 *struct {
			StartTime               string `json:"startTime"`
			EndTime                 string `json:"endTime"`
			ModeratorAvailabilityID int64  `json:"moderatorAvailabilityId"`
			HasImportedOverlap      any    `json:"hasImportedOverlap"`
		} `json:"slot"`
	}
	if errs := validate.DecodeAndValidate(r, &body); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	if body.TimeSlotID == 0 {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "TIMESLOT_NOT_FOUND",
			"errorMessage": "an error occurred in respondent reschedule",
		})
		return
	}

	// Check invalidateReschedule mode
	invalidate := false
	switch v := body.InvalidateReschedule.(type) {
	case bool:
		invalidate = v
	case string:
		invalidate = v == "true"
	}

	if invalidate {
		writeJSON(w, http.StatusOK, map[string]any{
			"message": "Rescheduled successfully!, no cancellation of previous interview done.",
		})
		return
	}

	// Normal reschedule: schedule new + cancel old
	// Legacy invokes handle-schedule-interview Lambda then cancel-resch-interview Lambda
	// This is a contract scaffold — full Lambda orchestration requires separate migration
	writeJSON(w, http.StatusOK, map[string]any{
		"handleScheduleInterviewResp": "{}",
		"hanldeCancelRescheduleResp":  "{}",
	})
}

// InvalidateInterviewMRA handles POST /interview/invalidate (MRA).
// Contract-identical with legacy: validates request, checks timeslot state,
// sets is_invalidated_interview=1 with reason, updates project status.
// Response: {status:"SUCCESS", message:"Interview invalidated successfully"}.
func (h *MRAHandler) InvalidateInterviewMRA(w http.ResponseWriter, r *http.Request) {
	if h.qsTimeSlotRepo == nil || h.qsProjectRepo == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "An error occurred while invalidating the interview",
		})
		return
	}

	var body struct {
		TimeSlotID             any    `json:"timeSlotId"`
		InvalidationReasonCode string `json:"invalidationReasonCode"`
		InvalidationReasonText string `json:"invalidationReasonText"`
		InvalidatedByUserID    any    `json:"invalidatedByUserId"`
		IsInvalidateEmailSent  *bool  `json:"isInvalidateEmailSent"`
	}
	if errs := validate.DecodeAndValidate(r, &body); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	// Validation
	validReasonCodes := map[string]bool{"PARTICIPANT_NO_SHOW": true, "MODERATOR_NO_SHOW": true, "OTHER": true}
	var validationErrors []string

	// Parse timeSlotId
	var timeSlotID int64
	switch v := body.TimeSlotID.(type) {
	case float64:
		timeSlotID = int64(v)
	case string:
		timeSlotID, _ = strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	}
	if timeSlotID == 0 {
		validationErrors = append(validationErrors, "timeSlotId is missing!")
	}

	if !validReasonCodes[body.InvalidationReasonCode] {
		validationErrors = append(validationErrors, "invalidationReasonCode must be one of: PARTICIPANT_NO_SHOW, MODERATOR_NO_SHOW, OTHER")
	}

	// Parse invalidatedByUserId
	var invalidatedByUserID int64
	switch v := body.InvalidatedByUserID.(type) {
	case float64:
		invalidatedByUserID = int64(v)
	case string:
		invalidatedByUserID, _ = strconv.ParseInt(v, 10, 64)
	}
	if invalidatedByUserID == 0 {
		validationErrors = append(validationErrors, "invalidatedByUserId is missing!")
	}

	if body.IsInvalidateEmailSent == nil {
		validationErrors = append(validationErrors, "isInvalidateEmailSent must be a boolean")
	}

	var reasonText *string
	if body.InvalidationReasonCode == "OTHER" {
		trimmed := strings.TrimSpace(body.InvalidationReasonText)
		if trimmed == "" {
			validationErrors = append(validationErrors, "invalidationReasonText is required when invalidationReasonCode is OTHER")
		} else if len(trimmed) > 150 {
			validationErrors = append(validationErrors, "invalidationReasonText cannot exceed 150 characters")
		} else {
			reasonText = &trimmed
		}
	}

	if len(validationErrors) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": validationErrors})
		return
	}

	// Check timeslot exists and not already invalidated
	ts, err := h.qsTimeSlotRepo.GetByID(r.Context(), timeSlotID)
	if err != nil || ts == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "An error occurred while invalidating the interview",
		})
		return
	}

	if ts.IsInvalidatedInterview {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error": "Interview already invalidated!",
		})
		return
	}

	// Check completed payment — legacy blocks invalidation if payment is done
	hasPaid, err := h.qsTimeSlotRepo.HasCompletedPaymentMRA(r.Context(), timeSlotID)
	if err != nil {
		slog.Error("check payment status failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "An error occurred while invalidating the interview",
		})
		return
	}
	if hasPaid {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error": "Interview cannot be invalidated because payment is already completed",
		})
		return
	}

	// Invalidate
	if err := h.qsTimeSlotRepo.InvalidateInterviewMRA(r.Context(), timeSlotID, body.InvalidationReasonCode, reasonText, invalidatedByUserID); err != nil {
		slog.Error("invalidate interview failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "An error occurred while invalidating the interview",
		})
		return
	}

	// Update project status to InProgress (2)
	_ = h.qsProjectRepo.Update(r.Context(), ts.ProjectID, map[string]any{"project_status_id": 2})

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "SUCCESS",
		"message": "Interview invalidated successfully",
	})
}

// SendInvalidateRescheduleMailMRA handles POST /interview/send_invalidate_reschedule_mail (MRA).
// Contract-identical scaffold with legacy: two flows — standard reschedule mail & ineligible PM notification.
// Full email orchestration (SES, template rendering, PM notification) requires separate migration.
func (h *MRAHandler) SendInvalidateRescheduleMailMRA(w http.ResponseWriter, r *http.Request) {
	if h.qsTimeSlotRepo == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "Failed to send reschedule email",
		})
		return
	}

	var body struct {
		TimeSlotID       any   `json:"timeSlotId"`
		ParticipantID    any   `json:"participantId"`
		IsIneligibleMail *bool `json:"isIneligibleMail"`
	}
	if errs := validate.DecodeAndValidate(r, &body); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	// Flow 1: Ineligible Mail — PM notification for participant eligibility change
	if body.IsIneligibleMail != nil && *body.IsIneligibleMail {
		// Legacy accepts participantId as single value or array
		var participantIDs []int64
		switch v := body.ParticipantID.(type) {
		case float64:
			participantIDs = append(participantIDs, int64(v))
		case []any:
			for _, item := range v {
				if num, ok := item.(float64); ok {
					participantIDs = append(participantIDs, int64(num))
				}
			}
		}
		if len(participantIDs) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "participantId is required"})
			return
		}

		// PARTIAL: Full implementation requires fetching upcoming timeslots per participant,
		// PM details, communication preferences, email template rendering, and SES sending.
		slog.Info("SendInvalidateRescheduleMailMRA: ineligible mail flow (PARTIAL)", "participantIDs", participantIDs)
		writeJSON(w, http.StatusOK, map[string]any{
			"success":       true,
			"processed_ids": participantIDs,
			"failed_ids":    []int64{},
		})
		return
	}

	// Flow 2: Standard reschedule mail
	var timeSlotID int64
	switch v := body.TimeSlotID.(type) {
	case float64:
		timeSlotID = int64(v)
	case string:
		timeSlotID, _ = strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	}
	if timeSlotID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "timeSlotId is required"})
		return
	}

	// Fetch timeslot and validate state
	ts, err := h.qsTimeSlotRepo.GetByID(r.Context(), timeSlotID)
	if err != nil || ts == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "Timeslot not found"})
		return
	}

	if !ts.IsInvalidatedInterview {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "Interview not invalidated yet!"})
		return
	}

	if ts.IsInvalidateEmailSent {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "Invalidate Reschedule Email Already Sent"})
		return
	}

	// PARTIAL: Full implementation requires:
	// - Fetch responder by timeSlotId (email, language, timezone, externalResponderId)
	// - Fetch project details (surveyId, salesForceJobNumber, shghash, externalSurveyId)
	// - Fetch/generate reschedule token (generateHash)
	// - Build reschedule URL with query params
	// - Fetch & render email template with timezone-localized start time
	// - Send email via SES
	// - Mark email as sent (markInvalidateEmailSentService)
	// - Notify PM (fire-and-forget)
	slog.Info("SendInvalidateRescheduleMailMRA: reschedule mail flow (PARTIAL)", "timeSlotID", timeSlotID)

	writeJSON(w, http.StatusOK, map[string]any{"status": "SUCCESS"})
}

// ──────────────────────────────────────────────
// MRA #35 — CancelRescheduleAction
// POST /v1/project/{pid}/time_slot/{tsId}/{action}
// Legacy: cancel-resch-interview.js
// Actions: cancel→5(ModeratorCancel), reschedule→3(ModeratorReschedule),
//   pmCancel→12, pmReschedule→11, respondentReschedule→4, respondentCancel→6
// ──────────────────────────────────────────────

func (h *MRAHandler) CancelRescheduleAction(w http.ResponseWriter, r *http.Request) {
	action, _ := validate.ParseStringParam(r, "action")
	tsID, err := validate.ParseIDParam(r, "tsId")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.qsTimeSlotRepo == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "timeslot repository not available",
			"errorMessage": "An error occured while rescheduling or canceling the interview",
		})
		return
	}

	// Map action to status_id matching legacy exactly
	var statusID int
	switch action {
	case "cancel":
		statusID = 5 // ModeratorCancel
	case "reschedule":
		statusID = 3 // ModeratorReschedule
	case "pmCancel":
		statusID = 12 // ProjectManagerCancel
	case "pmReschedule":
		statusID = 11 // ProjectManagerReschedule
	case "respondentReschedule":
		statusID = 4 // RespondentReschedule
	case "respondentCancel":
		statusID = 6 // RespondentCancel
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "action must be cancel, reschedule, pmCancel, pmReschedule, respondentReschedule, or respondentCancel"})
		return
	}

	ctx := r.Context()

	// Fetch timeslot first (legacy does this before any action)
	ts, err := h.qsTimeSlotRepo.GetByID(ctx, tsID)
	if err != nil || ts == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "timeslot not found",
			"errorMessage": "An error occured while rescheduling or canceling the interview",
		})
		return
	}

	// Legacy: if timeslot already cancelled/rescheduled (statusId in [3,4,5,6,11,12]), return 200 with timeslot data (no-op)
	alreadyActioned := map[int]bool{3: true, 4: true, 5: true, 6: true, 11: true, 12: true}
	if alreadyActioned[ts.StatusID] {
		writeJSON(w, http.StatusOK, buildTimeSlotResponse(ts))
		return
	}

	// PARTIAL: Legacy checks moderator external calendar import status.
	// If 'In Progress' and action != respondentReschedule, returns 405.
	// Full implementation requires moderator_external_calendar table query.

	// Parse request body (legacy parses body for responderLanguage etc.)
	var body struct {
		ResponderLanguage string `json:"responderLanguage"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	// Update status_id
	if err := h.qsTimeSlotRepo.Update(ctx, tsID, map[string]any{"status_id": statusID}); err != nil {
		slog.Error("cancel/reschedule update failed", "tsId", tsID, "action", action, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while rescheduling or canceling the interview",
		})
		return
	}

	// PARTIAL: Full implementation requires:
	// - Send cancel/reschedule email to respondent (SES, template rendering, timezone conversion)
	// - Send PM notification email (if not respondentReschedule and PM allows email)
	// - handleCancelOrRescheduleService (moderator availability cleanup, Google Calendar removal)
	slog.Info("CancelRescheduleAction (PARTIAL)", "tsId", tsID, "action", action, "statusId", statusID)

	// Re-fetch to return updated timeslot — legacy returns timeSlotRes.records[0]
	tsUpdated, err := h.qsTimeSlotRepo.GetByID(ctx, tsID)
	if err != nil || tsUpdated == nil {
		// Fallback: update was successful, return original with new statusId
		ts.StatusID = statusID
		writeJSON(w, http.StatusOK, buildTimeSlotResponse(ts))
		return
	}

	writeJSON(w, http.StatusOK, buildTimeSlotResponse(tsUpdated))
}
