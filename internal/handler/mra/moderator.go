package mra

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/handler/support"

	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
	"github.com/InCrowd/unified-qual-api/internal/validate"
	"github.com/go-chi/chi/v5"
)

// ──────────────────────────────────────────────
// MRA Moderator handlers
// ──────────────────────────────────────────────

// GetAllModeratorsMRA handles GET /user/get_all_moderators/{client_id} (MRA).
// Contract-identical with legacy: returns moderators for a client with interview count.
// Response: [{id, firstName, lastName, interviewCount}]
// No side effects — pure read API.
func (h *Handler) GetAllModeratorsMRA(w http.ResponseWriter, r *http.Request) {
	if !h.ModeratorService.QsUserAvailable() {
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "user repository not available"})
		return
	}

	clientID, err := validate.ParseIDParam(r, "client_id")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	records, err := h.ModeratorService.GetAllModeratorsListMRA(r.Context(), clientID)
	if err != nil {
		slog.Error("get all moderators failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	// Legacy returns result.records directly as array
	support.WriteJSON(w, http.StatusOK, records)
}

// PostModeratorAvailabilityMRA handles POST /moderator/post-availability (MRA).
// Contract-identical with legacy: validates dates, checks external calendar, checks timeslot conflicts,
// merges overlapping availabilities, creates new availability, returns all availabilities.
func (h *Handler) PostModeratorAvailabilityMRA(w http.ResponseWriter, r *http.Request) {
	headers := map[string]string{"Content-Type": "application/json", "Access-Control-Allow-Origin": "*"}
	_ = headers

	if !h.ModeratorService.QsUserAvailable() {
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "user repository not available"})
		return
	}

	var body struct {
		ModeratorID int64  `json:"moderatorId"`
		ClientID    int64  `json:"clientId"`
		StartTime   string `json:"startTime" validate:"required"`
		EndTime     string `json:"endTime" validate:"required"`
	}
	if errs := validate.DecodeAndValidate(r, &body); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	start, err := time.Parse(time.RFC3339, body.StartTime)
	if err != nil {
		start, err = time.Parse("2006-01-02T15:04:05.000Z", body.StartTime)
	}
	end, errEnd := time.Parse(time.RFC3339, body.EndTime)
	if errEnd != nil {
		end, errEnd = time.Parse("2006-01-02T15:04:05.000Z", body.EndTime)
	}
	if err != nil || errEnd != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": "Invalid date format"})
		return
	}

	now := time.Now()
	if !end.After(start) || start.Before(now) {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{
			"error": "End time must be greater than start time and start time cannot be in the past",
		})
		return
	}

	ctx := r.Context()

	// Check external calendar import status
	calStatus, _ := h.ModeratorService.GetModExternalCalendarStatusMRA(ctx, body.ModeratorID)
	if calStatus == "In Progress" {
		support.WriteJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"errorMessage": "There is a running import process for this moderator, try again later",
		})
		return
	}

	// Get moderator buffer (default 15)
	buffer, _ := h.ModeratorService.GetModeratorBufferMRA(ctx, body.ModeratorID)

	// Check for timeslot conflicts
	conflictCount, err := h.ModeratorService.IsValidAvailabilityMRA(ctx, body.ModeratorID, body.StartTime, body.EndTime, buffer)
	if err != nil {
		slog.Error("check availability validity failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if conflictCount > 0 {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{
			"error": "This availability conflicts with an existing timeslot or some other condition",
		})
		return
	}

	// Check for overlapping availabilities and merge
	overlapping, err := h.ModeratorService.OverlappingAvailabilitiesMRA(ctx, body.ModeratorID, body.StartTime, body.EndTime)
	if err != nil {
		slog.Error("get overlapping availabilities failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	newStartTime := start
	newEndTime := end
	skipCreate := false

	for _, oldAv := range overlapping {
		oldST, _ := time.Parse("2006-01-02 15:04:05", oldAv["startTime"].(string))
		oldET, _ := time.Parse("2006-01-02 15:04:05", oldAv["endTime"].(string))
		avID := oldAv["id"].(int64)

		if (oldST.Equal(newStartTime) || oldST.After(newStartTime)) && (oldET.Equal(newEndTime) || oldET.Before(newEndTime)) {
			// Old is fully contained in new → delete old
			_ = h.ModeratorService.DeleteModeratorAvailability(ctx, avID)
		} else if oldST.Before(newStartTime) && oldET.After(newEndTime) {
			// New is wrapped inside old → don't create
			skipCreate = true
		} else if oldST.Before(newStartTime) || oldET.Equal(newStartTime) {
			// Old starts before new → extend newStartTime
			newStartTime = oldST
			_ = h.ModeratorService.DeleteModeratorAvailability(ctx, avID)
		} else if oldET.After(newEndTime) || oldST.Equal(newEndTime) {
			// Old ends after new → extend newEndTime
			newEndTime = oldET
			_ = h.ModeratorService.DeleteModeratorAvailability(ctx, avID)
		}
	}

	// Create the merged availability
	if !skipCreate {
		_, err = h.ModeratorService.CreateModeratorAvailability(ctx, body.ModeratorID, body.ClientID, newStartTime, newEndTime)
		if err != nil {
			slog.Error("create moderator availability failed", "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
	}

	// Return all availabilities with imported overlap resolution (matching legacy getAllModeratorAvailabilityWithImportedService)
	result := h.getAllModeratorAvailabilityWithImported(ctx, body.ModeratorID, body.ClientID)
	support.WriteJSON(w, http.StatusOK, result)
}

// getAllModeratorAvailabilityWithImported replicates the legacy getAllModeratorAvailabilityWithImportedService.
// It fetches 4 sets of availabilities and merges them with overlap resolution.
func (h *Handler) getAllModeratorAvailabilityWithImported(ctx context.Context, moderatorID, clientID int64) []map[string]any {
	nonOverlapManual, err1 := h.ModeratorService.GetNonOverlappingManualAvailabilityMRA(ctx, moderatorID, clientID)
	nonOverlapImported, err2 := h.ModeratorService.GetNonOverlappingImportedAvailabilityMRA(ctx, moderatorID, clientID)
	overlapManual, err3 := h.ModeratorService.GetAllOverlappingManualAvailabilityMRA(ctx, moderatorID, clientID)
	overlapImported, err4 := h.ModeratorService.GetAllOverlappingImportedAvailabilityMRA(ctx, moderatorID, clientID)

	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		slog.Error("availability query errors", "err1", err1, "err2", err2, "err3", err3, "err4", err4)
	}

	var result []map[string]any
	t := 0

	// Non-overlapping manual
	for _, d := range nonOverlapManual {
		d["isImported"] = false
		d["availabilityId"] = d["id"]
		d["id"] = t
		t++
		result = append(result, d)
	}

	// Non-overlapping imported
	for _, d := range nonOverlapImported {
		d["isImported"] = true
		d["availabilityId"] = d["id"]
		d["id"] = t
		t++
		result = append(result, d)
	}

	frontEndId := t + 1

	// For each overlapping manual, find its overlapping imported counterparts
	for i := range overlapManual {
		manAv := overlapManual[i]
		manST := parseTimeFlexible(fmt.Sprint(manAv["startTime"]))
		manET := parseTimeFlexible(fmt.Sprint(manAv["endTime"]))

		// Filter imported that overlap with this manual availability
		var overlappingImp []map[string]any
		for _, imp := range overlapImported {
			impST := parseTimeFlexible(fmt.Sprint(imp["startTime"]))
			impET := parseTimeFlexible(fmt.Sprint(imp["endTime"]))
			if (impST.Equal(manST) || impST.After(manST)) && impST.Before(manET) ||
				impET.After(manST) && (impET.Equal(manET) || impET.Before(manET)) ||
				impST.Before(manST) && impET.After(manET) {
				overlappingImp = append(overlappingImp, imp)
			}
		}

		manAv["isImported"] = false
		manAv["availabilityId"] = manAv["id"]

		if len(overlappingImp) == 0 {
			manAv["id"] = frontEndId
			frontEndId++
			result = append(result, manAv)
		} else {
			for j := range overlappingImp {
				imp := overlappingImp[j]
				impST := parseTimeFlexible(fmt.Sprint(imp["startTime"]))
				impET := parseTimeFlexible(fmt.Sprint(imp["endTime"]))

				imp["isImported"] = true
				imp["availabilityId"] = imp["id"]

				if impST.After(manST) && impET.Before(manET) {
					// Imported fully inside manual → split manual
					manualHalf := map[string]any{
						"id":             frontEndId,
						"moderatorId":    manAv["moderatorId"],
						"firstName":      manAv["firstName"],
						"lastName":       manAv["lastName"],
						"clientId":       manAv["clientId"],
						"startTime":      manAv["startTime"],
						"endTime":        fmt.Sprint(imp["startTime"]),
						"isImported":     false,
						"availabilityId": manAv["availabilityId"],
					}
					frontEndId++
					manAv["startTime"] = fmt.Sprint(imp["endTime"])
					result = append(result, manualHalf)
					imp["id"] = frontEndId
					frontEndId++
					result = append(result, imp)
				} else if (impST.Equal(manST) || impST.Before(manST)) && (impET.Equal(manET) || impET.After(manET)) {
					// Imported wraps manual → replace with imported
					imp["id"] = frontEndId
					frontEndId++
					result = append(result, imp)
				} else if impST.Before(manET) && (impET.Equal(manET) || impET.After(manET)) {
					// Imported extends past manual end → truncate manual, add imported
					manAv["endTime"] = fmt.Sprint(imp["startTime"])
					manAv["id"] = frontEndId
					frontEndId++
					result = append(result, manAv)
					imp["id"] = frontEndId
					frontEndId++
					result = append(result, imp)
				} else if impET.After(manST) && (impST.Equal(manST) || impST.Before(manST)) {
					// Imported extends past manual start → add imported, truncate manual start
					imp["id"] = frontEndId
					frontEndId++
					result = append(result, imp)
					manAv["startTime"] = fmt.Sprint(imp["endTime"])
					manAv["id"] = frontEndId
					frontEndId++
					result = append(result, manAv)
				} else {
					imp["id"] = frontEndId
					frontEndId++
					result = append(result, imp)
				}
			}
		}
	}

	if result == nil {
		result = []map[string]any{}
	}
	return result
}

// parseTimeFlexible parses a time string in multiple formats.
func parseTimeFlexible(s string) time.Time {
	for _, layout := range []string{
		"2006-01-02 15:04:05",
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// UpdateModeratorAvailabilityMRA handles PUT /moderator/{availability_id}/update-availability (MRA).
// Contract-identical with legacy: validates against timeslots with moderator buffer, updates availability,
// returns all moderator availabilities with user info.
func (h *Handler) UpdateModeratorAvailabilityMRA(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "availability_id")
	avID, _ := strconv.ParseInt(idStr, 10, 64)

	if !h.ModeratorService.QsUserAvailable() {
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "user repository not available"})
		return
	}

	ctx := r.Context()

	// Step 1: Find existing availability
	oldAv, err := h.ModeratorService.FindModeratorAvailabilityByIdMRA(ctx, avID)
	if err != nil {
		slog.Error("find availability failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if oldAv == nil {
		support.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "Availability not found"})
		return
	}

	// Decode request body
	var req struct {
		StartTime string `json:"startTime"`
		EndTime   string `json:"endTime"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	// Step 2: Get moderator info with buffer
	modInfo, err := h.ModeratorService.GetModeratorsInfoByAvailabilityIdMRA(ctx, avID)
	if err != nil {
		slog.Error("get moderator info failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	moderatorBuffer := 15
	if modInfo != nil {
		if buf, ok := modInfo["moderatorBuffer"].(int); ok {
			moderatorBuffer = buf
		}
	}

	// Step 3: Check validity against existing timeslots (using OLD availability data, as legacy does)
	modID := oldAv["moderatorId"].(int64)
	clientID := oldAv["clientId"].(int64)
	oldST := fmt.Sprint(oldAv["startTime"])
	oldET := fmt.Sprint(oldAv["endTime"])

	conflictCount, err := h.ModeratorService.IsValidAvailabilityMRA(ctx, modID, oldST, oldET, moderatorBuffer)
	if err != nil {
		slog.Error("check availability validity failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if conflictCount > 0 {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{
			"error": "This availability conflicts with an existing timeslot or some other condition",
		})
		return
	}

	// Step 4: Update the availability
	startTime, _ := time.Parse(time.RFC3339, req.StartTime)
	if startTime.IsZero() {
		startTime, _ = time.Parse("2006-01-02T15:04:05.000Z", req.StartTime)
	}
	endTime, _ := time.Parse(time.RFC3339, req.EndTime)
	if endTime.IsZero() {
		endTime, _ = time.Parse("2006-01-02T15:04:05.000Z", req.EndTime)
	}

	if err := h.ModeratorService.UpdateModeratorAvailability(ctx, avID, startTime, endTime); err != nil {
		slog.Error("update availability failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	// Step 5: Return all moderator availabilities (matching legacy getAllModeratorAvailabilityService)
	result, err := h.ModeratorService.GetAllModeratorAvailabilityWithUserMRA(ctx, modID, clientID)
	if err != nil {
		slog.Error("get all availability failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	support.WriteJSON(w, http.StatusOK, result)
}

// DeleteModeratorAvailabilityMRA handles DELETE /moderator/{availability_id}/delete-availability (MRA).
// Contract-identical with legacy: checks external calendar, re-creates overlapping imported avails as manual,
// deletes the availability, returns merged availability list with imported overlap resolution.
func (h *Handler) DeleteModeratorAvailabilityMRA(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "availability_id")
	avID, _ := strconv.ParseInt(idStr, 10, 64)

	if !h.ModeratorService.QsUserAvailable() {
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "user repository not available"})
		return
	}

	ctx := r.Context()

	// Step 1: Find existing availability
	oldAv, err := h.ModeratorService.FindModeratorAvailabilityByIdMRA(ctx, avID)
	if err != nil {
		slog.Error("find availability failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if oldAv == nil {
		support.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "availability not found"})
		return
	}

	modID := oldAv["moderatorId"].(int64)
	clientID := oldAv["clientId"].(int64)

	// Step 2: Check external calendar status
	calStatus, _ := h.ModeratorService.GetModExternalCalendarStatusMRA(ctx, modID)
	if calStatus == "In Progress" {
		support.WriteJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"errorMessage": "There is a running import process for this moderator, try again later",
		})
		return
	}

	// Step 3: Before deleting, check for overlapping imported avails and re-create them as manual
	oldST := fmt.Sprint(oldAv["startTime"])
	oldET := fmt.Sprint(oldAv["endTime"])
	overlappingImported, err := h.ModeratorService.GetOverlappingImportedAvailabilityMRA(ctx, modID, clientID, oldST, oldET)
	if err != nil {
		slog.Error("get overlapping imported failed", "error", err)
	}
	for _, imp := range overlappingImported {
		impModID, _ := imp["moderatorId"].(int64)
		impClientID, _ := imp["clientId"].(int64)
		impST := fmt.Sprint(imp["startTime"])
		impET := fmt.Sprint(imp["endTime"])
		_ = h.ModeratorService.AddModeratorAvailabilityFromImportedMRA(ctx, impModID, impClientID, impST, impET)
	}

	// Step 4: Delete the availability
	if err := h.ModeratorService.DeleteModeratorAvailability(ctx, avID); err != nil {
		slog.Error("delete availability failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	// Step 5: Return all availabilities with imported overlap resolution
	result := h.getAllModeratorAvailabilityWithImported(ctx, modID, clientID)
	support.WriteJSON(w, http.StatusOK, result)
}

// ──────────────────────────────────────────────
// MRA #44 — Get Moderators Availability
// GET /get_moderators_availability/{qs_path}/survey/{survey_id}
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// MRA #44 — Get Moderators Availability
// GET /get_moderators_availability/{qs_path}/survey/{survey_id}
// ──────────────────────────────────────────────

// GetModeratorsAvailabilityMRA handles GET /get_moderators_availability/{qs_path}/survey/{survey_id}.
// Contract-identical with legacy getModeratorsAvailability Lambda handler.
// Returns moderator availability slots split into 15-min intervals grouped by date.
func (h *Handler) GetModeratorsAvailabilityMRA(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	surveyID, err := validate.ParseIDParam(r, "survey_id")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	respondentIdentifier := r.URL.Query().Get("respondentIdentifer")
	rescheduleToken := r.URL.Query().Get("rescheduleToken")

	if h.QsSurveyRepo == nil || !h.ProjectService.QsProjectAvailable() || !h.ModeratorService.QsUserAvailable() || !h.ModeratorService.TimeSlotAvailable() || h.QsRespondentRepo == nil {
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":           "database not configured",
			"errorMessage":    "an error occurred while getting moderators availability",
			"customErrorCode": "DB_NOT_CONFIGURED",
		})
		return
	}

	// Step 1: Get survey → clientId, projectId
	survey, err := h.SurveyService.GetSurveyByIdMRA(ctx, surveyID)
	if err != nil {
		slog.Error("get survey failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":           err.Error(),
			"errorMessage":    "an error occurred while getting moderators availability",
			"customErrorCode": err.Error(),
		})
		return
	}
	if survey == nil {
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":           "survey not found",
			"errorMessage":    "an error occurred while getting moderators availability",
			"customErrorCode": "SURVEY_NOT_FOUND",
		})
		return
	}
	clientID, _ := survey["client_id"].(int64)
	projectID, _ := survey["project_id"].(int64)

	// Step 2: Get project details
	project, err := h.ProjectService.GetProjectDetailsMRA(ctx, projectID)
	if err != nil {
		slog.Error("get project details failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":           err.Error(),
			"errorMessage":    "an error occurred while getting moderators availability",
			"customErrorCode": err.Error(),
		})
		return
	}
	if project == nil {
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":           "project not found",
			"errorMessage":    "an error occurred while getting moderators availability",
			"customErrorCode": "PROJECT_NOT_FOUND",
		})
		return
	}

	sampleSize, _ := project["sampleSize"].(int64)
	scheduled, _ := project["scheduled"].(int64)
	completed, _ := project["completed"].(int64)
	interviewLength, _ := project["interviewLength"].(int64)
	postScreeninBufferStr, _ := project["postScreeninBuffer"].(string)
	postScreeninBuffer := 0.0
	if postScreeninBufferStr != "" {
		postScreeninBuffer, _ = strconv.ParseFloat(postScreeninBufferStr, 64)
	}

	// Step 3: Check quota
	var isOverquota bool
	if rescheduleToken != "" {
		isOverquota = sampleSize == (scheduled-1)+completed
	} else {
		isOverquota = sampleSize == scheduled+completed
	}
	if isOverquota {
		slog.Error("quota reached", "surveyId", surveyID, "projectId", projectID)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":           "Quota Reached",
			"errorMessage":    "an error occurred while getting moderators availability",
			"customErrorCode": "QUOTA_REACHED",
		})
		return
	}

	// Step 4: Validate respondent
	if rescheduleToken == "" {
		// Regular schedule or moderator reschedule
		statusRecords, err := h.ModeratorService.GetInvalidTimeSlotStatusMRA(ctx, respondentIdentifier, projectID)
		if err != nil {
			slog.Error("get invalid timeslot status failed", "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":           err.Error(),
				"errorMessage":    "an error occurred while getting moderators availability",
				"customErrorCode": err.Error(),
			})
			return
		}
		if len(statusRecords) > 0 {
			slog.Error("respondent already scheduled", "respondentIdentifier", respondentIdentifier, "projectId", projectID)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":           "Respondent already scheduled an interview, interview cancelled or completed",
				"errorMessage":    "an error occurred while getting moderators availability",
				"customErrorCode": "QUOTA_REACHED",
			})
			return
		}
	} else {
		// Respondent reschedule — validate token
		statusRecords, err := h.ModeratorService.GetInvalidTimeSlotStatusForRespRescMRA(ctx, respondentIdentifier, projectID)
		if err != nil {
			slog.Error("get invalid timeslot status for resp resc failed", "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":           err.Error(),
				"errorMessage":    "an error occurred while getting moderators availability",
				"customErrorCode": err.Error(),
			})
			return
		}
		if len(statusRecords) > 0 {
			slog.Error("interview cancelled or completed", "respondentIdentifier", respondentIdentifier, "projectId", projectID)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":           "Interview cancelled or completed",
				"errorMessage":    "an error occurred while getting moderators availability",
				"customErrorCode": "QUOTA_REACHED",
			})
			return
		}

		// Get respondent ID
		respRecords, err := h.ParticipantService.GetRespondentByExternalIdMRA(ctx, respondentIdentifier, projectID)
		if err != nil {
			slog.Error("get respondent by external id failed", "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":           err.Error(),
				"errorMessage":    "an error occurred while getting moderators availability",
				"customErrorCode": err.Error(),
			})
			return
		}
		if len(respRecords) == 0 {
			slog.Error("respondent not found", "respondentIdentifier", respondentIdentifier)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":           "Respondent not found",
				"errorMessage":    "an error occurred while getting moderators availability",
				"customErrorCode": "Respondent not found",
			})
			return
		}
		responderID, _ := respRecords[0]["responderId"].(int64)

		// Check pending timeslot for token expiry
		pendingSlots, err := h.ModeratorService.GetPendingTimeslotByProjectAndResponderMRA(ctx, projectID, responderID)
		if err != nil {
			slog.Error("get pending timeslot failed", "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":           err.Error(),
				"errorMessage":    "an error occurred while getting moderators availability",
				"customErrorCode": err.Error(),
			})
			return
		}

		if len(pendingSlots) == 0 {
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":           "timeslot not found for this respondent",
				"errorMessage":    "an error occurred while getting moderators availability",
				"customErrorCode": "timeslot not found for this respondent",
			})
			return
		}

		timeSlotStartTimeStr, _ := pendingSlots[0]["start_time"].(string)
		isInvalidatedInterview, _ := pendingSlots[0]["is_invalidated_interview"].(bool)

		if timeSlotStartTimeStr != "" {
			timeSlotStartTime, parseErr := time.Parse("2006-01-02 15:04:05", timeSlotStartTimeStr)
			if parseErr == nil {
				bufferDuration := time.Duration(postScreeninBuffer * float64(time.Hour))
				tokenExpired := timeSlotStartTime.Before(time.Now().UTC().Add(bufferDuration))
				if tokenExpired && !isInvalidatedInterview {
					slog.Error("token expired", "respondentIdentifier", respondentIdentifier)
					support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
						"error":           "token expired",
						"errorMessage":    "an error occurred while getting moderators availability",
						"customErrorCode": "token expired",
					})
					return
				}
			}
		}

		// Validate reschedule token
		existingToken, err := h.ParticipantService.GetRescheduleTokenMRA(ctx, projectID, responderID)
		if err != nil {
			slog.Error("get reschedule token failed", "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":           err.Error(),
				"errorMessage":    "an error occurred while getting moderators availability",
				"customErrorCode": err.Error(),
			})
			return
		}
		if existingToken == "" {
			slog.Error("token not found", "respondentIdentifier", respondentIdentifier)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":           "token not found",
				"errorMessage":    "an error occurred while getting moderators availability",
				"customErrorCode": "token not found",
			})
			return
		}
		if existingToken != rescheduleToken {
			slog.Error("token mismatch", "respondentIdentifier", respondentIdentifier)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":           "token mismatch",
				"errorMessage":    "an error occurred while getting moderators availability",
				"customErrorCode": "token mismatch",
			})
			return
		}
	}

	// Step 5: Get moderator time ranges and build map
	moderatorsTimeRange, err := h.ProjectService.GetModeratorsTimeRangePerProjectMRA(ctx, projectID)
	if err != nil {
		slog.Error("get moderators time range failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":           err.Error(),
			"errorMessage":    "an error occurred while getting moderators availability",
			"customErrorCode": err.Error(),
		})
		return
	}

	type modTimeRange struct {
		startTime string
		endTime   string
		timezone  string
	}
	moderatorsWithTimeRangeMap := make(map[int64]modTimeRange)
	for _, tr := range moderatorsTimeRange {
		modID, _ := tr["moderator_id"].(int64)
		moderatorsWithTimeRangeMap[modID] = modTimeRange{
			startTime: fmt.Sprint(tr["start_time"]),
			endTime:   fmt.Sprint(tr["end_time"]),
			timezone:  fmt.Sprint(tr["timezone"]),
		}
	}

	// Step 6: Get all moderator availabilities
	moderatorsAvailability, err := h.ModeratorService.GetAllModeratorsAvailabilityPerClientMRA(ctx, clientID, projectID)
	if err != nil {
		slog.Error("get all moderators availability failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":           err.Error(),
			"errorMessage":    "an error occurred while getting moderators availability",
			"customErrorCode": err.Error(),
		})
		return
	}

	// Step 7: Split availabilities into 15-min intervals
	var dividedAvailabilities []map[string]any
	currentTime := time.Now().UTC()

	for _, av := range moderatorsAvailability {
		avID, _ := av["id"].(int64)
		modID, _ := av["moderatorId"].(int64)
		avClientID, _ := av["clientId"].(int64)
		stStr, _ := av["startTime"].(string)
		etStr, _ := av["endTime"].(string)

		startTime, err := time.Parse("2006-01-02 15:04:05", stStr)
		if err != nil {
			continue
		}
		endTime, err := time.Parse("2006-01-02 15:04:05", etStr)
		if err != nil {
			continue
		}

		// Get overlapping imported availability for this availability window
		overlappingImported, err := h.ModeratorService.GetOverlappingImportedAvailabilityMRA(ctx, modID, avClientID, stStr, etStr)
		if err != nil {
			slog.Error("get overlapping imported failed", "error", err)
			overlappingImported = []map[string]any{}
		}

		minutes := int(endTime.Sub(startTime).Minutes())
		for i := 1; i <= minutes/15; i++ {
			offset := time.Duration((i-1)*15) * time.Minute
			newStartTime := startTime.Add(offset)
			newEndTime := newStartTime.Add(time.Duration(interviewLength) * time.Minute)

			// Check imported overlap
			hasImportedOverlap := false
			for _, impAv := range overlappingImported {
				impSTStr := fmt.Sprint(impAv["startTime"])
				impETStr := fmt.Sprint(impAv["endTime"])
				impST, e1 := time.Parse("2006-01-02 15:04:05", impSTStr)
				impET, e2 := time.Parse("2006-01-02 15:04:05", impETStr)
				if e1 != nil || e2 != nil {
					continue
				}
				if (impST.After(newStartTime) && impST.Before(newEndTime)) ||
					(impET.After(newStartTime) && impET.Before(newEndTime)) ||
					(!impST.After(newStartTime) && !impET.Before(newEndTime)) {
					hasImportedOverlap = true
					break
				}
			}

			// Check time buffer
			bufferDuration := time.Duration(postScreeninBuffer * float64(time.Hour))
			respectsTimeBuffer := !newStartTime.Before(time.Now().UTC().Add(bufferDuration))

			// Check moderator working hours
			isWithinTimeRangeCondition := true
			if tr, ok := moderatorsWithTimeRangeMap[modID]; ok {
				isWithinTimeRangeCondition = isWithinModeratorTimeRange(tr.startTime, tr.endTime, tr.timezone, newStartTime, newEndTime)
			}

			if !newStartTime.Before(currentTime) && !newEndTime.After(endTime) && respectsTimeBuffer && isWithinTimeRangeCondition {
				dividedAvailabilities = append(dividedAvailabilities, map[string]any{
					"moderatorAvailabilityId": avID,
					"clientId":                avClientID,
					"startTime":               newStartTime.Format("2006-01-02T15:04:05.000") + "Z",
					"endTime":                 newEndTime.Format("2006-01-02T15:04:05.000") + "Z",
					"moderatorId":             modID,
					"hasImportedOverlap":      hasImportedOverlap,
				})
			}
		}
	}

	// Step 8: Add sequential id field
	for j := range dividedAvailabilities {
		dividedAvailabilities[j]["id"] = j
	}

	// Step 9: Group by date "YYYY-MM-DD"
	grouped := make(map[string][]map[string]any)
	for _, av := range dividedAvailabilities {
		stStr, _ := av["startTime"].(string)
		t, err := time.Parse("2006-01-02T15:04:05.000Z", stStr)
		if err != nil {
			continue
		}
		key := t.Format("2006-01-02")
		grouped[key] = append(grouped[key], av)
	}

	support.WriteJSON(w, http.StatusOK, grouped)
}

// isWithinModeratorTimeRange checks if a time interval falls within the moderator's working hours
// in the moderator's timezone. Mirrors legacy isWithinTimeRange helper.
func isWithinModeratorTimeRange(rangeStart, rangeEnd, timezone string, newStart, newEnd time.Time) bool {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return true // if timezone invalid, don't filter
	}
	// Convert UTC times to moderator timezone
	localStart := newStart.In(loc)
	localEnd := newEnd.In(loc)
	// Format as HHmm for numeric comparison
	startHHMM := localStart.Hour()*100 + localStart.Minute()
	endHHMM := localEnd.Hour()*100 + localEnd.Minute()
	// 00:00 means end of day (24:00)
	if endHHMM == 0 {
		endHHMM = 2400
	}
	// Parse range values (stored as "HHmm" strings like "0900", "1700")
	var rangeStartInt, rangeEndInt int
	if _, err := fmt.Sscanf(rangeStart, "%d", &rangeStartInt); err != nil {
		return true
	}
	if _, err := fmt.Sscanf(rangeEnd, "%d", &rangeEndInt); err != nil {
		return true
	}
	return rangeStartInt <= startHHMM && rangeEndInt >= endHHMM
}

// GetModeratorTimeslotsMRA handles POST /moderator/get/{moderator_id}/client/{client_id}/time_slots (MRA).
// Contract-identical with legacy: external calendar check, optional project filter, returns timeslots
// with payment status, topic name, imported overlap, invalidation fields.
func (h *Handler) GetModeratorTimeslotsMRA(w http.ResponseWriter, r *http.Request) {
	modIDStr := chi.URLParam(r, "moderator_id")
	modID, _ := strconv.ParseInt(modIDStr, 10, 64)
	clientIDStr := chi.URLParam(r, "client_id")
	clientID, _ := strconv.ParseInt(clientIDStr, 10, 64)

	if !h.ModeratorService.TimeSlotAvailable() || !h.ModeratorService.QsUserAvailable() {
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "repository not available"})
		return
	}

	ctx := r.Context()

	// Check external calendar status
	calStatus, _ := h.ModeratorService.GetModExternalCalendarStatusMRA(ctx, modID)
	if calStatus == "In Progress" {
		support.WriteJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"errorMessage": "There is a running import process for this moderator, try again later",
		})
		return
	}

	// Parse optional body for projectsToFilter
	var body struct {
		ProjectsToFilter []int64 `json:"projectsToFilter"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}

	var records []map[string]any
	var err error
	if len(body.ProjectsToFilter) > 0 {
		records, err = h.ModeratorService.GetModeratorTimeSlotsWithFilterMRA(ctx, modID, clientID, body.ProjectsToFilter)
	} else {
		records, err = h.ModeratorService.GetModeratorTimeSlotsMRA(ctx, modID, clientID)
	}
	if err != nil {
		slog.Error("get moderator timeslots mra failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error has occured while getting moderator time slots",
		})
		return
	}

	support.WriteJSON(w, http.StatusOK, records)
}

// GetModeratorInterviewsMRA handles POST /moderator/get-all-interviews/{moderator_id} (MRA).
// Contract-identical with legacy: returns interviews with payment status, honorarium, conference link,
// supports search (query ?q=), project exclusion, and payment status filtering.
func (h *Handler) GetModeratorInterviewsMRA(w http.ResponseWriter, r *http.Request) {
	modIDStr := chi.URLParam(r, "moderator_id")
	modID, _ := strconv.ParseInt(modIDStr, 10, 64)

	if !h.ModeratorService.TimeSlotAvailable() {
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "repository not available"})
		return
	}

	// Parse body
	var body struct {
		ProjectsIDs       string `json:"projectsIds"`
		PaymentStatusCode string `json:"paymentStatusCode"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}

	// Parse search query param
	search := r.URL.Query().Get("q")

	// Parse project IDs to exclude
	var excludeIDs []int64
	if body.ProjectsIDs != "" && body.ProjectsIDs != "()" {
		cleaned := strings.Trim(body.ProjectsIDs, "()")
		for _, s := range strings.Split(cleaned, ",") {
			s = strings.TrimSpace(s)
			if id, err := strconv.ParseInt(s, 10, 64); err == nil {
				excludeIDs = append(excludeIDs, id)
			}
		}
	}

	ctx := r.Context()
	records, err := h.ModeratorService.GetAllInterviewsMRA(ctx, modID, search, excludeIDs, body.PaymentStatusCode)
	if err != nil {
		slog.Error("get moderator interviews mra failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	support.WriteJSON(w, http.StatusOK, records)
}

// GetProjectsForModeratorMRA handles GET /moderator/get-projects/{moderator_id}/client/{client_id} (MRA).
// Contract-identical with legacy: returns {clientId, moderatorId, data: [{id, name}]}
func (h *Handler) GetProjectsForModeratorMRA(w http.ResponseWriter, r *http.Request) {
	modIDStr := chi.URLParam(r, "moderator_id")
	modID, _ := strconv.ParseInt(modIDStr, 10, 64)
	clientIDStr := chi.URLParam(r, "client_id")
	clientID, _ := strconv.ParseInt(clientIDStr, 10, 64)

	if !h.ProjectService.QsProjectAvailable() {
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "repository not available"})
		return
	}

	records, err := h.ProjectService.GetProjectsForModeratorMRA(r.Context(), clientID, modID)
	if err != nil {
		slog.Error("get projects for moderator mra failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while getting projects for moderator",
		})
		return
	}

	support.WriteJSON(w, http.StatusOK, map[string]any{
		"clientId":    clientIDStr,
		"moderatorId": modIDStr,
		"data":        records,
	})
}

// GetModeratorsListForProjectMRA handles GET /moderator/client/{project_id}/list (MRA).
// Legacy path param is named client_id but actually passes project_id.
// Contract-identical: returns {moderatorInfo: [{firstName, lastName, id, clientId, interviewCount, startTime, endTime, timezone}]}
func (h *Handler) GetModeratorsListForProjectMRA(w http.ResponseWriter, r *http.Request) {
	projectIDStr := chi.URLParam(r, "project_id")
	projectID, _ := strconv.ParseInt(projectIDStr, 10, 64)

	if !h.ModeratorService.QsUserAvailable() {
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "repository not available"})
		return
	}

	records, err := h.ModeratorService.GetModeratorsListMRA(r.Context(), projectID)
	if err != nil {
		slog.Error("get moderators list mra failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while getting the list of moderators",
		})
		return
	}

	support.WriteJSON(w, http.StatusOK, map[string]any{
		"moderatorInfo": records,
	})
}

// ──────────────────────────────────────────────
// MRA #50 — UpdateModeratorMRA
// PUT /v1/moderator/get/{moderator_id}
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// MRA #50 — UpdateModeratorMRA
// PUT /v1/moderator/get/{moderator_id}
// ──────────────────────────────────────────────

// UpdateModeratorMRA handles PUT /moderator/get/{moderator_id} (MRA).
// Updates moderator buffer and cascades availability adjustments.
// Contract-identical: returns [] (empty JSON array) on success.
func (h *Handler) UpdateModeratorMRA(w http.ResponseWriter, r *http.Request) {
	moderatorID, err := validate.ParseIDParam(r, "moderator_id")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if !h.ModeratorService.QsUserAvailable() {
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "repository not available"})
		return
	}

	var body struct {
		ModeratorBuffer      int   `json:"moderatorBuffer"`
		UpdateAvailabilities *bool `json:"updateAvailabilities,omitempty"`
	}
	if errs := validate.DecodeAndValidate(r, &body); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	ctx := r.Context()

	// Step 1: fetch old buffer and clientId
	oldBuffer, clientID, err := h.ModeratorService.FetchUserInfoByUserIdMRA(ctx, moderatorID)
	if err != nil {
		slog.Error("fetch user info failed", "error", err, "moderatorId", moderatorID)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	newBuffer := body.ModeratorBuffer

	// Step 2: update moderator buffer
	if err := h.ModeratorService.UpdateModeratorBufferMRA(ctx, moderatorID, newBuffer); err != nil {
		slog.Error("update moderator buffer failed", "error", err, "moderatorId", moderatorID)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	increase := newBuffer > oldBuffer

	// Step 3: update manual availabilities based on buffer
	if err := h.updateAvailabilitiesBasedOnBufferMRA(ctx, newBuffer, moderatorID, clientID, increase); err != nil {
		slog.Error("update availabilities based on buffer failed", "error", err, "moderatorId", moderatorID)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	// Step 4: update imported availabilities based on buffer
	if err := h.updateImportedAvailabilitiesBasedOnBufferMRA(ctx, newBuffer, moderatorID, clientID, increase); err != nil {
		slog.Error("update imported availabilities based on buffer failed", "error", err, "moderatorId", moderatorID)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	// Step 5: clean up invalid availabilities (start >= end)
	if err := h.ModeratorService.CleanUpAvailabilitiesByModeratorIdMRA(ctx, moderatorID); err != nil {
		slog.Error("cleanup availabilities failed", "error", err, "moderatorId", moderatorID)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	// Step 6: remove nested availabilities
	if err := h.ModeratorService.RemoveNestedAvailabilitiesMRA(ctx, moderatorID); err != nil {
		slog.Error("remove nested availabilities failed", "error", err, "moderatorId", moderatorID)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	// Legacy returns result.records from an UPDATE query — empty array.
	support.WriteJSON(w, http.StatusOK, []any{})
}

// updateAvailabilitiesBasedOnBufferMRA adjusts manual moderator availabilities
// based on proximity to scheduled interviews after a buffer change.
func (h *Handler) updateAvailabilitiesBasedOnBufferMRA(ctx context.Context, newBuffer int, moderatorID, clientID int64, increase bool) error {
	// Get future interviews for this moderator
	interviews, err := h.ModeratorService.GetFutureModeratorTimeslotsMRA(ctx, moderatorID, clientID)
	if err != nil {
		return fmt.Errorf("get future timeslots: %w", err)
	}
	if len(interviews) == 0 {
		return nil
	}

	// Get all future manual availabilities with proximity to interviews
	avails, err := h.ModeratorService.GetFutureAvailsWithProximityMRA(ctx, moderatorID, clientID)
	if err != nil {
		return fmt.Errorf("get future avails with proximity: %w", err)
	}

	for _, avail := range avails {
		if err := h.processAvailabilityBufferAdjustmentMRA(ctx, avail.ID, avail.StartTime, avail.EndTime,
			interviews, newBuffer, false); err != nil {
			return err
		}
	}
	return nil
}

// updateImportedAvailabilitiesBasedOnBufferMRA adjusts imported moderator availabilities
// based on proximity to scheduled interviews after a buffer change.
func (h *Handler) updateImportedAvailabilitiesBasedOnBufferMRA(ctx context.Context, newBuffer int, moderatorID, clientID int64, increase bool) error {
	interviews, err := h.ModeratorService.GetFutureModeratorTimeslotsMRA(ctx, moderatorID, clientID)
	if err != nil {
		return fmt.Errorf("get future timeslots: %w", err)
	}
	if len(interviews) == 0 {
		return nil
	}

	avails, err := h.ModeratorService.GetFutureImportedAvailsWithProximityMRA(ctx, moderatorID, clientID)
	if err != nil {
		return fmt.Errorf("get future imported avails with proximity: %w", err)
	}

	for _, avail := range avails {
		if err := h.processAvailabilityBufferAdjustmentMRA(ctx, avail.ID, avail.StartTime, avail.EndTime,
			interviews, newBuffer, true); err != nil {
			return err
		}
	}
	return nil
}

// processAvailabilityBufferAdjustmentMRA handles the two-pass availability adjustment
// for a single availability against all interviews. Works for both manual and imported tables.
func (h *Handler) processAvailabilityBufferAdjustmentMRA(ctx context.Context,
	availID int64, availStart, availEnd time.Time,
	interviews []qs.ModeratorTimeslotMRA, newBuffer int, imported bool) error {

	bufferDur := time.Duration(newBuffer) * time.Minute

	// Pass 1: find nearest interview END TIME to availability START TIME (within 3600s)
	var nearestEndTime *time.Time
	minDiffStart := int64(3601) // > 3600 sentinel
	for i := range interviews {
		diff := int64(availStart.Sub(interviews[i].EndTime).Seconds())
		if diff < 0 {
			diff = -diff
		}
		if diff <= 3600 && diff < minDiffStart {
			minDiffStart = diff
			t := interviews[i].EndTime
			nearestEndTime = &t
		}
	}

	if nearestEndTime != nil {
		// Re-query availability length (may have been modified by previous iteration)
		var al *qs.AvailLengthMRA
		var err error
		if imported {
			al, err = h.ModeratorService.GetImportedModeratorAvailabilityLengthMRA(ctx, availID)
		} else {
			al, err = h.ModeratorService.GetModeratorAvailabilityLengthMRA(ctx, availID)
		}
		if err != nil {
			return fmt.Errorf("get availability length: %w", err)
		}

		shouldDelete := al.Length < 0 ||
			(al.Length != 0 && al.Length < 60 && newBuffer > al.Length &&
				!al.EndTime.After(nearestEndTime.Add(bufferDur)))

		if shouldDelete {
			if imported {
				return h.ModeratorService.DeleteImportedModeratorAvailabilityByIdMRA(ctx, availID)
			}
			return h.ModeratorService.DeleteModeratorAvailabilityByIdMRA(ctx, availID)
		}

		// If interview end is before availability end AND within 3600s of availability start
		if nearestEndTime.Before(al.EndTime) {
			diff := int64(nearestEndTime.Sub(al.StartTime).Seconds())
			if diff < 0 {
				diff = -diff
			}
			if diff <= 3600 {
				newStart := nearestEndTime.Add(bufferDur)
				if imported {
					if err := h.ModeratorService.UpdateImportedModeratorAvailabilityStartTimeMRA(ctx, availID, newStart); err != nil {
						return err
					}
				} else {
					if err := h.ModeratorService.UpdateModeratorAvailabilityStartTimeMRA(ctx, availID, newStart); err != nil {
						return err
					}
				}
			}
		}
	}

	// Pass 2: find nearest interview START TIME to availability END TIME (within 3600s)
	var nearestStartTime *time.Time
	minDiffEnd := int64(3601)
	for i := range interviews {
		diff := int64(interviews[i].StartTime.Sub(availEnd).Seconds())
		if diff < 0 {
			diff = -diff
		}
		if diff <= 3600 && diff < minDiffEnd {
			minDiffEnd = diff
			t := interviews[i].StartTime
			nearestStartTime = &t
		}
	}

	if nearestStartTime != nil {
		var al *qs.AvailLengthMRA
		var err error
		if imported {
			al, err = h.ModeratorService.GetImportedModeratorAvailabilityLengthMRA(ctx, availID)
		} else {
			al, err = h.ModeratorService.GetModeratorAvailabilityLengthMRA(ctx, availID)
		}
		if err != nil {
			return fmt.Errorf("get availability length (pass 2): %w", err)
		}

		shouldDelete := al.Length < 0 ||
			(al.Length != 0 && al.Length < 60 && newBuffer > al.Length &&
				!al.StartTime.Before(nearestStartTime.Add(-bufferDur)))

		if shouldDelete {
			if imported {
				return h.ModeratorService.DeleteImportedModeratorAvailabilityByIdMRA(ctx, availID)
			}
			return h.ModeratorService.DeleteModeratorAvailabilityByIdMRA(ctx, availID)
		}

		// If interview start is after availability start AND within 60 min of availability end
		if nearestStartTime.After(al.StartTime) {
			diff := int64(nearestStartTime.Sub(al.EndTime).Seconds())
			if diff < 0 {
				diff = -diff
			}
			if diff <= 3600 {
				newEnd := nearestStartTime.Add(-bufferDur)
				if imported {
					if err := h.ModeratorService.UpdateImportedModeratorAvailabilityEndTimeMRA(ctx, availID, newEnd); err != nil {
						return err
					}
				} else {
					if err := h.ModeratorService.UpdateModeratorAvailabilityEndTimeMRA(ctx, availID, newEnd); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

// GetModeratorsForTimeSlotMRA handles GET /get-moderators-for-timeSlot/{timeslot_id} (MRA).
// Contract-identical with legacy: returns [{timeSlotId, moderatorId, isHost}]
func (h *Handler) GetModeratorsForTimeSlotMRA(w http.ResponseWriter, r *http.Request) {
	tsIDStr := chi.URLParam(r, "timeslot_id")
	tsID, _ := strconv.ParseInt(tsIDStr, 10, 64)

	if !h.ModeratorService.TimeSlotAvailable() {
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "repository not available"})
		return
	}

	records, err := h.ModeratorService.GetModeratorsForTimeSlotMRA(r.Context(), tsID)
	if err != nil {
		slog.Error("get moderators for timeslot mra failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while getting moderators for timeslot id",
		})
		return
	}

	support.WriteJSON(w, http.StatusOK, records)
}

// GetModeratorsOptionMRA handles GET /get-moderators-option/{timeslot_id} (MRA).
// Contract-identical: returns [{hasConflict, isHost, moderator: {id, firstName, lastName}}]
func (h *Handler) GetModeratorsOptionMRA(w http.ResponseWriter, r *http.Request) {
	tsIDStr := chi.URLParam(r, "timeslot_id")
	tsID, _ := strconv.ParseInt(tsIDStr, 10, 64)

	if !h.ModeratorService.TimeSlotAvailable() {
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "repository not available"})
		return
	}

	ctx := r.Context()

	startTime, endTime, err := h.ModeratorService.GetStartEndTimeBySlotIdMRA(ctx, tsID)
	if err != nil {
		slog.Error("get start end time failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while getting possible moderators for timeslot",
		})
		return
	}

	modsInfo, err := h.ModeratorService.GetModeratorsInfoForSlotMRA(ctx, tsID)
	if err != nil {
		slog.Error("get moderators info failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while getting possible moderators for timeslot",
		})
		return
	}

	var result []map[string]any
	for _, m := range modsInfo {
		hasConflict, err := h.ModeratorService.GetModeratorConflictForSlotMRA(ctx, m.ID, startTime, endTime, tsID)
		if err != nil {
			slog.Error("get moderator conflict failed", "error", err, "moderatorId", m.ID)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": "An error occured while getting possible moderators for timeslot",
			})
			return
		}
		result = append(result, map[string]any{
			"hasConflict": hasConflict,
			"isHost":      m.IsHost,
			"moderator": map[string]any{
				"id":        m.ID,
				"firstName": m.FirstName,
				"lastName":  m.LastName,
			},
		})
	}
	if result == nil {
		result = []map[string]any{}
	}

	support.WriteJSON(w, http.StatusOK, result)
}

// GetParticipantIdMRA handles GET /get-participant-id/{timeslot_id} (MRA).
// Contract-identical: returns [{externalResponderId}]
func (h *Handler) GetParticipantIdMRA(w http.ResponseWriter, r *http.Request) {
	tsIDStr := chi.URLParam(r, "timeslot_id")
	tsID, _ := strconv.ParseInt(tsIDStr, 10, 64)

	if !h.ParticipantService.Available() {
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "repository not available"})
		return
	}

	records, err := h.ParticipantService.GetParticipantIdMRA(r.Context(), tsID)
	if err != nil {
		slog.Error("get participant id mra failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while getting participant id for timeslot id",
		})
		return
	}

	support.WriteJSON(w, http.StatusOK, records)
}

// UpsertModeratorTimeRangeMRA handles POST /project/{project_id}/time_range/moderator/{moderator_id} (MRA).
// Contract-identical with legacy: checks project status is Defining (1), then upserts moderator_time_range.
// Response: transaction result (serialized as the upsert response).
func (h *Handler) UpsertModeratorTimeRangeMRA(w http.ResponseWriter, r *http.Request) {
	projectID, err := validate.ParseIDParam(r, "project_id")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	moderatorID, err := validate.ParseIDParam(r, "moderator_id")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if !h.ProjectService.QsProjectAvailable() {
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "database not configured"})
		return
	}

	var body struct {
		StartTime string `json:"startTime"`
		EndTime   string `json:"endTime"`
		Timezone  string `json:"timezone"`
	}
	if errs := validate.DecodeAndValidate(r, &body); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	// Check project status is Defining (1)
	statusID, err := h.ProjectService.GetProjectStatusByID(r.Context(), projectID)
	if err != nil {
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if statusID != 1 {
		support.WriteJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"error": "Can't update moderator time range because the project status is not defining",
		})
		return
	}

	// Upsert
	if err := h.ProjectService.UpsertModeratorTimeRangePerProject(r.Context(), projectID, moderatorID, body.StartTime, body.EndTime, body.Timezone); err != nil {
		slog.Error("upsert moderator time range failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	// Legacy returns the transaction result which includes numberOfRecordsUpdated
	support.WriteJSON(w, http.StatusOK, map[string]any{"numberOfRecordsUpdated": 1})
}

// ──────────────────────────────────────────────
// MRA #64 — GetModeratorAvailabilityByClientMRA
// GET /v1/moderator/get/{moderator_id}/get-mod-av/{client_id}
// Legacy: returns merged manual+imported availability with overlap resolution.
// ──────────────────────────────────────────────

func (h *Handler) GetModeratorAvailabilityByClientMRA(w http.ResponseWriter, r *http.Request) {
	if !h.ModeratorService.QsUserAvailable() {
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "repository not available",
			"errorMessage": "An error occured while getting moderator availability",
		})
		return
	}

	moderatorID, err := validate.ParseIDParam(r, "moderator_id")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	clientID, err := validate.ParseIDParam(r, "client_id")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	ctx := r.Context()

	// Check external calendar import status
	calStatus, _ := h.ModeratorService.GetModExternalCalendarStatusMRA(ctx, moderatorID)
	if calStatus == "In Progress" {
		support.WriteJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"error": "There is a running import process for this moderator, try again later",
		})
		return
	}

	result := h.getAllModeratorAvailabilityWithImported(ctx, moderatorID, clientID)
	support.WriteJSON(w, http.StatusOK, result)
}

// ──────────────────────────────────────────────
// MRA #65 — StartModeratorImportMRA
// POST /v1/moderator/get/{moderator_id}/imported/{client_id}
// Legacy: Google Sheets import — DB parts only (PARTIAL).
// ──────────────────────────────────────────────

func (h *Handler) StartModeratorImportMRA(w http.ResponseWriter, r *http.Request) {
	if !h.ModeratorService.QsUserAvailable() {
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "repository not available"})
		return
	}

	moderatorID, err := validate.ParseIDParam(r, "moderator_id")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	clientID, err := validate.ParseIDParam(r, "client_id")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	var body struct {
		ExternalCalendarInput    string `json:"externalCalendarInput"`
		ExternalCalendarKeyInput string `json:"externalCalendarKeyInput"`
		ForceUpdate              bool   `json:"forceUpdate"`
		UserID                   any    `json:"userId"`
	}
	if errs := validate.DecodeAndValidate(r, &body); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	ctx := r.Context()

	// Step 1: Check external calendar status
	calStatus, _ := h.ModeratorService.GetModExternalCalendarStatusMRA(ctx, moderatorID)
	if calStatus == "In Progress" && !body.ForceUpdate {
		support.WriteJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"error": "There is a running import process for this moderator, try again later",
		})
		return
	}

	// Step 2: Update external calendar URL and key
	if err := h.ModeratorService.UpdateModExternalCalendarUrlMRA(ctx, moderatorID, body.ExternalCalendarInput, body.ExternalCalendarKeyInput); err != nil {
		slog.Error("update external calendar url failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	// Step 3: Set status to "In Progress"
	if err := h.ModeratorService.UpdateModExternalCalendarStatusMRA(ctx, moderatorID, "In Progress"); err != nil {
		slog.Error("update external calendar status failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	// Step 4: Delete existing imported avails
	if err := h.ModeratorService.DeleteImportedModeratorAvailabilityByModeratorMRA(ctx, moderatorID, clientID); err != nil {
		slog.Error("delete imported moderator availability failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	// PARTIAL: Google Sheets + Lambda calls are external and not replicated here
	slog.Info("StartModeratorImportMRA: import started (PARTIAL — Google Sheets integration not implemented)",
		"moderatorId", moderatorID, "clientId", clientID)

	// Legacy returns JSON.stringify("Importing in progress")
	support.WriteJSON(w, http.StatusOK, "Importing in progress")
}

// ──────────────────────────────────────────────
// MRA #66 — GetImportedAvailabilityFromCurrentSyncMRA
// GET /v1/moderator/get/{moderator_id}/imported-from-current-sync/{client_id}
// Legacy: Google Sheets parsing — DB fallback only (PARTIAL).
// ──────────────────────────────────────────────

func (h *Handler) GetImportedAvailabilityFromCurrentSyncMRA(w http.ResponseWriter, r *http.Request) {
	if !h.ModeratorService.QsUserAvailable() {
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "repository not available"})
		return
	}

	moderatorID, err := validate.ParseIDParam(r, "moderator_id")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	clientID, err := validate.ParseIDParam(r, "client_id")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	// PARTIAL: Legacy parses Google Sheets data. This returns DB-stored imported avails.
	result, err := h.ModeratorService.GetImportedModeratorAvailabilityMRA(r.Context(), moderatorID, clientID)
	if err != nil {
		slog.Error("get imported availability from current sync failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	support.WriteJSON(w, http.StatusOK, result)
}

// ──────────────────────────────────────────────
// Helper — language code → language_id mapping (legacy getLanguageIdByLangCode)
// ──────────────────────────────────────────────

var langCodeToID = map[string]int64{
	"en_us": 1,
	"fr_fr": 2,
	"fr_ca": 3,
	"de_de": 4,
	"es_es": 5,
	"it_it": 6,
	"pt_br": 7,
}

// ──────────────────────────────────────────────
// MRA #67 — GetImportStatusMRA
// GET /v1/mra/moderator/get/{moderator_id}/import-status
// Legacy: get-import-availability-status.js
// ──────────────────────────────────────────────

func (h *Handler) GetImportStatusMRA(w http.ResponseWriter, r *http.Request) {
	if !h.ModeratorService.QsUserAvailable() {
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "repository not available"})
		return
	}

	moderatorID, err := validate.ParseIDParam(r, "moderator_id")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	ctx := r.Context()

	statusResult, err := h.ModeratorService.GetModeratorExternalCalendarMRA(ctx, moderatorID)
	if err != nil {
		slog.Error("get moderator external calendar failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err})
		return
	}

	runningCount, err := h.ModeratorService.GetRunningImportProcessCountMRA(ctx)
	if err != nil {
		slog.Error("get running import process count failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err})
		return
	}

	support.WriteJSON(w, http.StatusOK, map[string]any{
		"Result": map[string]any{
			"statusResult":    statusResult,
			"canNotRunImport": runningCount >= 1,
		},
	})
}

// ──────────────────────────────────────────────
// MRA #68 — UnlinkImportedModeratorMRA
// POST /v1/mra/moderator/unlink-imp-mod/{moderator_id}
// Legacy: unlink-imp-mod.js
// ──────────────────────────────────────────────

func (h *Handler) UnlinkImportedModeratorMRA(w http.ResponseWriter, r *http.Request) {
	if !h.ModeratorService.QsUserAvailable() {
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "repository not available"})
		return
	}

	moderatorID, err := validate.ParseIDParam(r, "moderator_id")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	var req struct {
		ClientID int64 `json:"clientId"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	ctx := r.Context()

	// Check if import is in progress
	calRecords, err := h.ModeratorService.GetModeratorExternalCalendarMRA(ctx, moderatorID)
	if err != nil {
		slog.Error("get moderator external calendar failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	if len(calRecords) > 0 {
		if status, ok := calRecords[0]["status"].(string); ok && status == "In Progress" {
			support.WriteJSON(w, http.StatusMethodNotAllowed, map[string]any{
				"error": "There is a running import process for this moderator, try again later",
			})
			return
		}
	}

	// Delete imported availabilities and external calendar status
	if err := h.ModeratorService.DeleteImportedModeratorAvailabilityByModeratorMRA(ctx, moderatorID, req.ClientID); err != nil {
		slog.Error("delete imported moderator availability failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	if err := h.ModeratorService.DeleteExternalCalStatusMRA(ctx, moderatorID); err != nil {
		slog.Error("delete external calendar status failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	support.WriteJSON(w, http.StatusOK, "Imported Moderator calendar has been unlinked")
}

// ──────────────────────────────────────────────
// MRA #69 — UpdateGoogleSheetFirstDateMRA
// GET /v1/mra/moderator/update-google-sheet-first-date
// Legacy: update-google-sheet-first-date.js (PARTIAL — Google Sheets not implemented)
// ──────────────────────────────────────────────

func (h *Handler) UpdateGoogleSheetFirstDateMRA(w http.ResponseWriter, r *http.Request) {
	if h.ModeratorService.GoogleSheetsConfigured() {
		if err := h.ModeratorService.UpdateFirstDateCalendar(r.Context()); err != nil {
			slog.Warn("google sheets update first date calendar failed", "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": "An error has occured while downloading google sheet",
			})
			return
		}
	} else {
		slog.Info("UpdateGoogleSheetFirstDateMRA: Google Sheets not configured, skipping calendar update")
	}

	support.WriteJSON(w, http.StatusOK, map[string]any{"fileURL": ""})
}
