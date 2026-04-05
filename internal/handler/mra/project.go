package mra

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/xuri/excelize/v2"

	"github.com/InCrowd/unified-qual-api/internal/dto"
	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
	"github.com/InCrowd/unified-qual-api/internal/utilities"
)

// ──────────────────────────────────────────────
// MRA Project handlers
// ──────────────────────────────────────────────

// CreateProjectMRA handles POST /project/create-project (MRA).
// Contract-identical with legacy QS Tool: 9-query transaction creating
// external_client, project, survey, third_party_survey, question,
// survey_question, participant_group, topics, then SELECT project details.
// Response: flat project details object (parsedJson[8]["records"][0]).
func (h *Handler) CreateProjectMRA(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	if errs := utilities.DecodeAndValidate(r, &body); errs != nil {
		utilities.WriteError(w, errs)
		return
	}

	if !h.ProjectService.QsProjectAvailable() {
		utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "database not configured"})
		return
	}

	// Legacy formatProject maps camelCase request to snake_case for DB,
	// but repo.CreateProjectFull expects the camelCase keys from the request body.
	// Look up SalesForceJobNumberText from salesforce_project table.
	sfJobNumber, _ := body["salesForceJobNumber"].(string)
	sfJobNumberText, err := h.ProjectService.GetSalesForceJobNumberText(r.Context(), sfJobNumber)
	if err != nil {
		slog.Error("sf job number text lookup failed", "error", err)
		// Legacy continues with empty string on failure
	}
	body["SalesForceJobNumberText"] = sfJobNumberText

	record, err := h.ProjectService.CreateProjectFull(r.Context(), body)
	if err != nil {
		slog.Error("create project failed", "error", err)
		utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	utilities.WriteJSON(w, http.StatusOK, record)
}

// GetProjectMRA handles GET /project/get-project-details/{project_id} (MRA).
// Contract-identical with legacy QS Tool: complex JOIN returning 24-field flat project object.
// Response: getProjectDetails.records[0] equivalent.
func (h *Handler) GetProjectMRA(w http.ResponseWriter, r *http.Request) {
	projectID, err := utilities.ParseIDParam(r, "id")
	if err != nil {
		utilities.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if !h.ProjectService.QsProjectAvailable() {
		utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "database not configured"})
		return
	}

	record, err := h.ProjectService.GetProjectDetailsMRA(r.Context(), projectID)
	if err != nil {
		slog.Error("get project details failed", "error", err)
		utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if record == nil {
		utilities.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "project not found"})
		return
	}

	utilities.WriteJSON(w, http.StatusOK, record)
}

// ListProjectsMRA handles POST /project/get-projects/client/{client_id} (MRA).
// Contract-identical with legacy QS Tool getProjects factory.
// Two code paths:
// 1. Body has keys (externalClientsIds, projectAccountId, userId) → filtered query + saveUserSelection
// 2. Empty body → simpler getProjectsForMods query
// Response: {clientId: <id>, data: records}
func (h *Handler) ListProjectsMRA(w http.ResponseWriter, r *http.Request) {
	clientIDStr := chi.URLParam(r, "client_id")

	if !h.ProjectService.QsProjectAvailable() {
		utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "database not configured"})
		return
	}

	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		body = map[string]any{}
	}

	q := r.URL.Query()
	creatorIDStr := q.Get("creatorId")
	statusStr := q.Get("status")
	sortBy := q.Get("sortBy")
	search := q.Get("q")

	creatorID := -1
	if creatorIDStr != "" {
		if v, err := strconv.Atoi(creatorIDStr); err == nil {
			creatorID = v
		}
	}
	status := -1
	if statusStr != "" {
		if v, err := strconv.Atoi(statusStr); err == nil {
			status = v
		}
	}
	if sortBy == "" {
		sortBy = "modifiedDate"
	}
	sort := "project.modified_on"
	if sortBy != "modifiedDate" {
		sort = "project.created_on"
	}

	var records []map[string]any
	var err error

	if len(body) > 0 {
		// Path 1: filtered query with externalClientsIds
		externalClientsIDsRaw, _ := body["externalClientsIds"].(string)
		cleaned := strings.Trim(externalClientsIDsRaw, "()")
		var externalIDs []string
		for _, part := range strings.Split(cleaned, ",") {
			part = strings.TrimSpace(part)
			part = strings.Trim(part, "'")
			if part != "" {
				externalIDs = append(externalIDs, part)
			}
		}
		if len(externalIDs) == 0 {
			externalIDs = []string{""}
		}

		records, err = h.ProjectService.GetProjectsMRA(r.Context(), creatorID, status, sort, search, externalIDs)
		if err != nil {
			slog.Error("get projects mra failed", "error", err)
			utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}

		// Side effect: save user selections (legacy wraps in try/catch, failures logged only)
		if userIDRaw, ok := body["userId"]; ok {
			projectAccountID, _ := body["projectAccountId"].(string)
			var userID int64
			switch v := userIDRaw.(type) {
			case float64:
				userID = int64(v)
			case string:
				userID, _ = strconv.ParseInt(v, 10, 64)
			}
			if userID > 0 {
				accCleaned := strings.Trim(projectAccountID, "()")
				var accountIDs []string
				for _, part := range strings.Split(accCleaned, ",") {
					part = strings.TrimSpace(part)
					if part != "" {
						accountIDs = append(accountIDs, part)
					}
				}
				if saveErr := h.ProjectService.SaveUserSelection(r.Context(), userID, accountIDs, externalIDs); saveErr != nil {
					slog.Error("save user selection failed", "error", saveErr)
				}
			}
		}
	} else {
		// Path 2: empty body → getProjectsForMods
		clientID, _ := strconv.ParseInt(clientIDStr, 10, 64)
		records, err = h.ProjectService.GetProjectsForModsMRA(r.Context(), clientID)
		if err != nil {
			slog.Error("get projects for mods mra failed", "error", err)
			utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
	}

	output := map[string]any{
		"clientId": clientIDStr,
		"data":     records,
	}
	utilities.WriteJSON(w, http.StatusOK, output)
}

// UpdateProjectMRA handles PUT /project/update-project-details/{project_id} (MRA).
// Contract-identical with legacy QS Tool: 3 code paths based on request body.
// 1. postScreeninBuffer set and != -1 → update buffer + modified_on
// 2. moderatorBuffer set and != -1 → update buffer + modified_on
// 3. else → set scheduler_generated = 1
// Response: {} (empty object — legacy transaction result has no "records" key)
func (h *Handler) UpdateProjectMRA(w http.ResponseWriter, r *http.Request) {
	projectID, err := utilities.ParseIDParam(r, "project_id")
	if err != nil {
		utilities.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if !h.ProjectService.QsProjectAvailable() {
		utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "database not configured",
			"errorMessage": "An error occured while updating project details",
		})
		return
	}

	var body map[string]any
	if errs := utilities.DecodeAndValidate(r, &body); errs != nil {
		utilities.WriteError(w, errs)
		return
	}

	// Legacy formatProject maps: postScreeninBuffer → post_screenin_buffer, moderatorBuffer → moderator_buffer
	postScreeninBuffer := float64(-1)
	if v, ok := body["postScreeninBuffer"]; ok && v != nil {
		if f, ok := v.(float64); ok {
			postScreeninBuffer = f
		}
	}
	moderatorBuffer := float64(-1)
	if v, ok := body["moderatorBuffer"]; ok && v != nil {
		if f, ok := v.(float64); ok {
			moderatorBuffer = f
		}
	}

	if postScreeninBuffer != -1 {
		err = h.ProjectService.UpdatePostScreenInBuffer(r.Context(), projectID, postScreeninBuffer)
	} else if moderatorBuffer != -1 {
		err = h.ProjectService.UpdateModeratorBufferMRA(r.Context(), projectID, moderatorBuffer)
	} else {
		err = h.ProjectService.UpdateSchedulerGenerated(r.Context(), projectID)
	}

	if err != nil {
		slog.Error("update project mra failed", "error", err)
		utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while updating project details",
		})
		return
	}

	// Legacy response: {data: undefined} → JSON.stringify → {}
	utilities.WriteJSON(w, http.StatusOK, map[string]any{})
}

// UpdateExternalSurveyIDMRA handles PUT /project/update-external-survey-id/{project_id} (MRA).
// Contract-identical with legacy: updates external_survey_id + modified_on.
// Request: {externalSurveyId}. Response: {} (empty object).
func (h *Handler) UpdateExternalSurveyIDMRA(w http.ResponseWriter, r *http.Request) {
	projectID, err := utilities.ParseIDParam(r, "project_id")
	if err != nil {
		utilities.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if !h.ProjectService.QsProjectAvailable() {
		utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "database not configured",
			"errorMessage": "An error occured while updating external survey id",
		})
		return
	}

	var body dto.MraUpdateExternalSurveyIDRequest
	if errs := utilities.DecodeAndValidate(r, &body); errs != nil {
		utilities.WriteError(w, errs)
		return
	}

	if err := h.ProjectService.UpdateExternalSurveyID(r.Context(), projectID, body.ExternalSurveyID); err != nil {
		slog.Error("update external survey id failed", "error", err)
		utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while updating external survey id",
		})
		return
	}

	utilities.WriteJSON(w, http.StatusOK, map[string]any{})
}

// ResetProjectModeratorsMRA handles POST /project/{project_id}/moderators_reset (MRA).
// Contract-identical with legacy: supports "reset" and "unassign" modes.
// Reset: diffs moderatorIds against existing, adds/removes from projects_users + moderator_time_range.
// Unassign: removes single moderator, returns updated moderator list.
func (h *Handler) ResetProjectModeratorsMRA(w http.ResponseWriter, r *http.Request) {
	projectID, err := utilities.ParseIDParam(r, "project_id")
	if err != nil {
		utilities.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if !h.ProjectService.QsProjectAvailable() {
		utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "database not configured",
			"errorMessage": "An error occured while trying to reset project moderators",
		})
		return
	}

	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = r.Header.Get("X-Mode")
	}

	var body dto.MraResetProjectModeratorsRequest
	if errs := utilities.DecodeAndValidate(r, &body); errs != nil {
		utilities.WriteError(w, errs)
		return
	}

	if mode == "reset" {
		existingIDs, err := h.ProjectService.GetProjectModeratorIDs(r.Context(), projectID)
		if err != nil {
			slog.Error("get project mod ids failed", "error", err)
			utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": "An error occured while trying to reset project moderators",
			})
			return
		}

		if err := h.ProjectService.ResetProjectModeratorsMRA(r.Context(), projectID, body.ModeratorIDs, existingIDs); err != nil {
			slog.Error("reset project mods failed", "error", err)
			utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": "An error occured while trying to reset project moderators",
			})
			return
		}

		// Legacy returns transaction result array
		utilities.WriteJSON(w, http.StatusOK, []map[string]any{})
	} else if mode == "unassign" {
		if len(body.ModeratorIDs) == 0 {
			utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        "no moderator id provided",
				"errorMessage": "An error occured while trying to reset project moderators",
			})
			return
		}
		modID := body.ModeratorIDs[0]
		if err := h.ProjectService.UnassignModeratorFromProject(r.Context(), modID, projectID); err != nil {
			slog.Error("unassign moderator failed", "error", err)
			utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": "An error occured while trying to reset project moderators",
			})
			return
		}

		mods, err := h.ProjectService.GetModeratorsList(r.Context(), projectID)
		if err != nil {
			slog.Error("get moderators list failed", "error", err)
			utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": "An error occured while trying to reset project moderators",
			})
			return
		}
		utilities.WriteJSON(w, http.StatusOK, mods)
	} else {
		utilities.WriteJSON(w, 422, map[string]any{
			"errorMessage": "Missing mode in request",
		})
	}
}

// GetEmailTemplateMRA handles GET /project/{project_id}/get_email_template (MRA).
// Contract-identical with legacy: routes by reschedule/addLink/editLink/invalidateReschedule
// query params to communication_type_id, queries by language_code.
// Response: {body_content: "..."} (records[0]).
func (h *Handler) GetEmailTemplateMRA(w http.ResponseWriter, r *http.Request) {
	if !h.ProjectService.QsProjectAvailable() {
		utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "database not configured",
			"errorMessage": "An error occured while getting the email template",
		})
		return
	}

	q := r.URL.Query()
	reschedule := q.Get("reschedule")
	invalidateReschedule := q.Get("invalidateReschedule")
	addLink := q.Get("addLink")
	editLink := q.Get("editLink")
	responderLanguage := q.Get("responderLanguage")

	var typeID int
	if reschedule == "true" {
		typeID = 2
	} else if invalidateReschedule == "true" {
		typeID = 17
	} else if reschedule == "false" && addLink == "false" && editLink == "false" {
		typeID = 4 // cancel
	} else if addLink == "true" {
		typeID = 1 // schedule
	} else if editLink == "true" {
		typeID = 3 // update
	}

	if typeID == 0 {
		utilities.WriteJSON(w, http.StatusOK, map[string]any{})
		return
	}

	record, err := h.ProjectService.GetEmailTemplateMRA(r.Context(), typeID, responderLanguage)
	if err != nil {
		slog.Error("get email template failed", "error", err)
		utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while getting the email template",
		})
		return
	}
	if record == nil {
		record = map[string]any{}
	}

	utilities.WriteJSON(w, http.StatusOK, record)
}

// HandleProjectExportMRA handles POST /project/{project_id}/handle-export (MRA).
// Contract-identical with legacy: queries export data, generates Excel, uploads to S3,
// returns presigned URL. Response: {fileURL: "<presigned url>"}.
func (h *Handler) HandleProjectExportMRA(w http.ResponseWriter, r *http.Request) {
	projectID, err := utilities.ParseIDParam(r, "project_id")
	if err != nil {
		utilities.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if !h.ProjectService.QsProjectAvailable() {
		utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "database not configured",
			"errorMessage": "An error occured while exporting project",
		})
		return
	}

	var body dto.MraHandleProjectExportRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		body.PMTimeZone = "America/New_York"
		body.PMTimeZoneAbbr = "EDT"
	}
	if body.PMTimeZone == "" {
		body.PMTimeZone = "America/New_York"
	}
	if body.PMTimeZoneAbbr == "" {
		body.PMTimeZoneAbbr = "EDT"
	}

	rescheduleLinkPrefix := "https://apolloqualscheduler.com/scheduler/qstool/"

	exportRows, err := h.ProjectService.HandleProjectExportMRA(r.Context(), projectID, body.PMTimeZone, body.PMTimeZoneAbbr, rescheduleLinkPrefix)
	if err != nil {
		slog.Error("export query failed", "error", err)
		utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while exporting project",
		})
		return
	}

	noTsRows, err := h.ProjectService.HandleProjectNoTimeslotExportMRA(r.Context(), projectID, body.PMTimeZone, body.PMTimeZoneAbbr)
	if err != nil {
		slog.Error("no-timeslot export query failed", "error", err)
	}

	projectName, err := h.ProjectService.GetProjectName(r.Context(), projectID)
	if err != nil {
		projectName = fmt.Sprintf("Project_%d", projectID)
	}

	allRows := append(exportRows, noTsRows...)

	f := excelize.NewFile()
	sheetName := "Sheet1"
	columns := []string{
		"User ID", "Duration", "Time of Interview", "First Name", "Last Name",
		"Sess Key", "Respondent Time of Interview", "Honorarium", "Modified Date",
		"Email", "Phone", "Conference Link", "Moderator First Name", "Moderator Last Name",
		"PM First Name", "PM Last Name", "Status", "Comment", "Reschedule Link",
	}
	for i, col := range columns {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheetName, cell, col)
	}
	for rowIdx, row := range allRows {
		for colIdx, val := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			_ = f.SetCellValue(sheetName, cell, val)
		}
	}

	s3Key := projectName + "_Schedule.xlsx"

	if h.ProjectService.S3ExportConfigured() {
		buf, err := f.WriteToBuffer()
		if err != nil {
			slog.Error("excel write failed", "error", err)
			utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": "An error occured while exporting project",
			})
			return
		}

		bucket := h.ProjectService.ExportBucket()
		_, err = h.ProjectService.UploadFileToS3(r.Context(), bucket, s3Key, buf, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		if err != nil {
			slog.Error("s3 upload failed", "error", err)
			utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": "An error occured while exporting project",
			})
			return
		}

		presignedURL, err := h.ProjectService.GetS3PresignedURL(r.Context(), bucket, s3Key, 15*time.Minute)
		if err != nil {
			slog.Error("presign failed", "error", err)
			utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": "An error occured while exporting project",
			})
			return
		}

		utilities.WriteJSON(w, http.StatusOK, map[string]any{"fileURL": presignedURL})
		return
	}

	utilities.WriteJSON(w, http.StatusOK, map[string]any{"fileURL": "", "rows": len(allRows)})
}

// UpdateSampleSizeMRA handles PUT /project/{project_id}/update-sample-size (MRA).
// Contract-identical with legacy: validates sampleSize vs current scheduled+completed,
// auto-transitions project_status_id between InProgress(2)↔Completed(3).
// Response: empty body on success (legacy returns transaction result which serializes to nothing).
func (h *Handler) UpdateSampleSizeMRA(w http.ResponseWriter, r *http.Request) {
	projectID, err := utilities.ParseIDParam(r, "project_id")
	if err != nil {
		utilities.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if !h.ProjectService.QsProjectAvailable() {
		utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "database not configured",
			"errorMessage": "An error occured while updating project sample size",
		})
		return
	}

	var body dto.MraUpdateSampleSizeRequest
	if errs := utilities.DecodeAndValidate(r, &body); errs != nil {
		utilities.WriteError(w, errs)
		return
	}

	// Get current project details to validate
	project, err := h.ProjectService.GetProjectDetailsMRA(r.Context(), projectID)
	if err != nil || project == nil {
		utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "project not found",
			"errorMessage": "An error occured while updating project sample size",
		})
		return
	}

	currentSampleSize, _ := project["sampleSize"].(int64)
	scheduled, _ := project["scheduled"].(int64)
	completed, _ := project["completed"].(int64)
	projectStatusID, _ := project["projectStatusId"].(int64)

	// Validation: sampleSize must differ from current and > 0
	if body.SampleSize == currentSampleSize || body.SampleSize == 0 {
		utilities.WriteJSON(w, http.StatusBadRequest, map[string]any{
			"errorMessage": "Updated sample size must be greater than 0 and different than the original sample size",
		})
		return
	}

	// Validation: sampleSize must be >= scheduled + completed
	if body.SampleSize < scheduled+completed {
		utilities.WriteJSON(w, http.StatusBadRequest, map[string]any{
			"errorMessage": "Condition newSampleSize >= shceduled+completed interviews not verified",
		})
		return
	}

	const projectStatusInProgress int64 = 2
	const projectStatusCompleted int64 = 3

	// Auto-transition logic
	if projectStatusID == projectStatusInProgress && scheduled == 0 && completed == body.SampleSize {
		// InProgress → Completed
		if err := h.ProjectService.UpdateSampleSizeProjectStatusMRA(r.Context(), projectID, body.SampleSize, projectStatusCompleted); err != nil {
			slog.Error("update sample size with status failed", "error", err)
			utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": "An error occured while updating project sample size",
			})
			return
		}
	} else if projectStatusID == projectStatusCompleted && scheduled == 0 {
		// Completed → InProgress
		if err := h.ProjectService.UpdateSampleSizeProjectStatusMRA(r.Context(), projectID, body.SampleSize, projectStatusInProgress); err != nil {
			slog.Error("update sample size with status failed", "error", err)
			utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": "An error occured while updating project sample size",
			})
			return
		}
	} else {
		// Just update sample_size
		if err := h.ProjectService.UpdateSampleSizeMRA(r.Context(), projectID, body.SampleSize); err != nil {
			slog.Error("update sample size failed", "error", err)
			utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": "An error occured while updating project sample size",
			})
			return
		}
	}

	// Legacy returns transaction result["records"] which is undefined → JSON.stringify produces empty
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Brand", "mra")
	w.WriteHeader(http.StatusOK)
}

// GetUnavailableModeratorsMRA handles GET /project/{project_id}/get-unavailable-moderators (MRA).
// Contract-identical with legacy: checks moderator availability against project settings,
// returns {displayError, displayWarning} flags.
func (h *Handler) GetUnavailableModeratorsMRA(w http.ResponseWriter, r *http.Request) {
	projectID, err := utilities.ParseIDParam(r, "project_id")
	if err != nil {
		utilities.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if !h.ProjectService.QsProjectAvailable() {
		utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "database not configured",
			"errorMessage": "An error occured while getting available and unavailable moderators assigned to the project",
		})
		return
	}

	// 1. Get project details
	project, err := h.ProjectService.GetProjectDetailsMRA(r.Context(), projectID)
	if err != nil || project == nil {
		utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"errorMessage": "An error occured while getting project details",
		})
		return
	}

	clientID, _ := project["clientId"].(int64)
	interviewLength, _ := project["interviewLength"].(int64)
	postScreeninBuffer, _ := project["postScreeninBuffer"].(string)

	bufferHours := 0.0
	if postScreeninBuffer != "" {
		_, _ = fmt.Sscanf(postScreeninBuffer, "%f", &bufferHours)
	}

	// 2. Get moderator time ranges per project
	timeRanges, err := h.ProjectService.GetModeratorsTimeRangePerProject(r.Context(), projectID)
	if err != nil {
		slog.Error("get moderator time ranges failed", "error", err)
	}
	timeRangeMap := map[int64]qs.ModeratorTimeRange{}
	for _, tr := range timeRanges {
		timeRangeMap[tr.ModeratorID] = tr
	}

	// 3. Get project moderator IDs
	modIDs, err := h.ProjectService.GetProjectModeratorIDs(r.Context(), projectID)
	if err != nil || len(modIDs) == 0 {
		utilities.WriteJSON(w, http.StatusBadRequest, map[string]any{
			"errorMessage": "No moderators assigned to the project ",
		})
		return
	}

	// 4. Get all moderator availability per client
	availabilities, err := h.ProjectService.GetAllModeratorsAvailabilityPerClient(r.Context(), clientID, projectID)
	if err != nil {
		slog.Error("get moderator availability failed", "error", err)
		utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while getting available and unavailable moderators assigned to the project",
		})
		return
	}

	output := map[string]any{
		"displayError":   false,
		"displayWarning": false,
	}

	if len(availabilities) == 0 {
		output["displayError"] = true
		utilities.WriteJSON(w, http.StatusOK, output)
		return
	}

	// 5. Compute available/unavailable sets
	availableMods := map[int64]bool{}
	unavailableMods := map[int64]bool{}

	now := time.Now().UTC()
	bufferDuration := time.Duration(bufferHours * float64(time.Hour))

	for _, av := range availabilities {
		startTime := av.StartTime
		endTime := av.EndTime

		minutes := int(endTime.Sub(startTime).Minutes())
		for i := 1; i <= minutes/15; i++ {
			offset := time.Duration((i-1)*15) * time.Minute
			newStart := startTime.Add(offset)
			newEnd := newStart.Add(time.Duration(interviewLength) * time.Minute)
			respectsBuffer := newStart.After(now.Add(bufferDuration)) || newStart.Equal(now.Add(bufferDuration))

			isWithin := true
			if tr, ok := timeRangeMap[av.ModeratorID]; ok {
				isWithin = isWithinTimeRange(tr, newStart, newEnd)
			}

			if (newStart.Equal(startTime) || newStart.After(startTime)) && (newEnd.Equal(endTime) || newEnd.Before(endTime)) && respectsBuffer && isWithin {
				availableMods[av.ModeratorID] = true
				delete(unavailableMods, av.ModeratorID)
			} else {
				if !availableMods[av.ModeratorID] {
					unavailableMods[av.ModeratorID] = true
				}
			}
		}
	}

	if len(availableMods) == 0 {
		output["displayError"] = true
		utilities.WriteJSON(w, http.StatusOK, output)
		return
	}

	if len(unavailableMods) > 0 {
		output["displayWarning"] = true
		utilities.WriteJSON(w, http.StatusOK, output)
		return
	}

	// Check if any assigned moderator has no availability
	for _, modID := range modIDs {
		if !availableMods[modID] {
			output["displayWarning"] = true
			break
		}
	}

	utilities.WriteJSON(w, http.StatusOK, output)
}

// isWithinTimeRange checks if a time slot falls within a moderator's configured time range.
// Mirrors legacy isWithinTimeRange helper: compares HHmm formatted times.
func isWithinTimeRange(tr qs.ModeratorTimeRange, newStart, newEnd time.Time) bool {
	loc, err := time.LoadLocation(tr.Timezone)
	if err != nil {
		return true // default to allowing if timezone unknown
	}
	convertedStart := newStart.In(loc)
	convertedEnd := newEnd.In(loc)

	startHHMM, _ := strconv.Atoi(convertedStart.Format("1504"))
	endHHMM, _ := strconv.Atoi(convertedEnd.Format("1504"))
	if endHHMM == 0 {
		endHHMM = 2400
	}

	rangeStart, _ := strconv.Atoi(tr.StartTime)
	rangeEnd, _ := strconv.Atoi(tr.EndTime)

	return rangeStart <= startHHMM && rangeEnd >= endHHMM
}

// GetModeratorsCountMRA handles GET /project/get-moderators-count/{project_id}/sample-size/{sample_size} (MRA).
// Contract-identical with legacy: sums availability slots divided by interviewLength,
// compares against sampleSize - completed to produce displayWarning flag.
// Response: {avModCount, displayWarning}.
func (h *Handler) GetModeratorsCountMRA(w http.ResponseWriter, r *http.Request) {
	projectID, err := utilities.ParseIDParam(r, "project_id")
	if err != nil {
		utilities.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	sampleSize, err := utilities.ParseIDParam(r, "sample_size")
	if err != nil {
		utilities.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if !h.ProjectService.QsProjectAvailable() {
		utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "database not configured",
			"errorMessage": "An error occured while getting the number of available moderators",
		})
		return
	}

	// Get moderator availability per role
	availabilities, err := h.ProjectService.GetAllModeratorsAvailabilityPerRole(r.Context(), projectID)
	if err != nil {
		slog.Error("get moderator availability per role failed", "error", err)
		utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while getting the number of available moderators",
		})
		return
	}

	// Get project details for interviewLength and completed count
	project, err := h.ProjectService.GetProjectDetailsMRA(r.Context(), projectID)
	if err != nil || project == nil {
		utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"errorMessage": "An error occured while getting project details",
		})
		return
	}

	interviewLength, _ := project["interviewLength"].(int64)
	completed, _ := project["completed"].(int64)

	if interviewLength == 0 {
		interviewLength = 1 // avoid divide by zero
	}

	// Sum availability slots: Math.floor(minutes / interviewLength)
	var modIdAvailabilitiesCount int64
	for _, av := range availabilities {
		minutes := int64(av.EndTime.Sub(av.StartTime).Minutes())
		modIdAvailabilitiesCount += minutes / interviewLength
	}

	displayWarning := false
	if modIdAvailabilitiesCount < sampleSize-completed {
		displayWarning = true
	}

	utilities.WriteJSON(w, http.StatusOK, map[string]any{
		"avModCount":     modIdAvailabilitiesCount,
		"displayWarning": displayWarning,
	})
}
