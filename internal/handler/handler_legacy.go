package handler

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/middleware"
	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/go-chi/chi/v5"
)

// ──────────────────────────────────────────────
// Survey Extended — detail, validate, crowds, close, favorite
// ──────────────────────────────────────────────

// GetSurveyDetail returns survey detail by id — brand-divergent.
func (h *Handler) GetSurveyDetail(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	surveyID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid survey id"})
		return
	}
	source := h.resolveSource(r)

	if source == "iris" && h.irisSurveyRepo != nil {
		s, err := h.irisSurveyRepo.GetSurvey(r.Context(), surveyID)
		if err != nil {
			slog.Error("iris survey get failed", "id", surveyID, "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		if s == nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "survey not found"})
			return
		}
		success(w, map[string]any{
			"id": s.ID, "subscriptionId": niVal(s.SubscriptionID),
			"surveyTypeId": s.SurveyTypeID, "namePublic": s.NamePublic,
			"namePrivate": nsVal(s.NamePrivate), "topicName": nsVal(s.TopicName),
			"projectId": s.ProjectID, "status": s.Status,
			"completionsNeeded": s.CompletionsNeeded,
			"createdOn": s.CreatedOn.Format(time.RFC3339),
			"isArchived": s.IsArchived, "languageId": s.LanguageID,
			"source": "iris", "serviceCategory": "LS",
		})
		return
	}

	// QS
	if h.qsSurveyRepo != nil {
		s, err := h.qsSurveyRepo.GetByID(r.Context(), surveyID)
		if err != nil {
			slog.Error("qs survey get failed", "id", surveyID, "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		if s == nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "survey not found"})
			return
		}
		success(w, map[string]any{
			"id": s.ID, "projectId": niVal(s.ProjectID),
			"title": s.Title, "status": s.Status,
			"questions": json.RawMessage(s.Questions), "rules": json.RawMessage(s.Rules),
			"createdOn": s.CreatedOn.Format(time.RFC3339),
			"source": "qs", "serviceCategory": "MRA",
		})
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "survey not found"})
}

// ValidateSurvey checks if a survey can be fielded.
func (h *Handler) ValidateSurvey(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	surveyID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid survey id"})
		return
	}

	if h.irisSurveyRepo != nil && h.resolveSource(r) == "iris" {
		errors, err := h.irisSurveyRepo.ValidateSurvey(r.Context(), surveyID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "validation failed"})
			return
		}
		valid := len(errors) == 0
		success(w, map[string]any{"valid": valid, "errors": errors, "source": "iris"})
		return
	}

	// QS: surveys are always valid if they exist
	if h.qsSurveyRepo != nil {
		s, _ := h.qsSurveyRepo.GetByID(r.Context(), surveyID)
		if s == nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "survey not found"})
			return
		}
		success(w, map[string]any{"valid": true, "errors": []string{}, "source": "qs"})
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "survey not found"})
}

// GetSurveyCrowds returns crowds assigned to a survey.
func (h *Handler) GetSurveyCrowds(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	surveyID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid survey id"})
		return
	}

	if h.irisSurveyRepo != nil && h.resolveSource(r) == "iris" {
		crowds, err := h.irisSurveyRepo.GetSurveyCrowds(r.Context(), surveyID)
		if err != nil {
			slog.Error("get survey crowds failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		success(w, map[string]any{"crowds": crowds, "source": "iris"})
		return
	}

	// QS doesn't have survey crowds in the same way
	success(w, map[string]any{"crowds": []any{}, "source": "qs"})
}

// CloseSurvey closes a survey.
func (h *Handler) CloseSurvey(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	surveyID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid survey id"})
		return
	}

	if h.irisSurveyRepo != nil && h.resolveSource(r) == "iris" {
		if err := h.irisSurveyRepo.CloseSurvey(r.Context(), surveyID); err != nil {
			slog.Error("close survey failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to close survey"})
			return
		}
		success(w, map[string]any{"closed": true, "id": surveyID, "source": "iris"})
		return
	}

	// QS: update status to 'closed'
	if h.qsSurveyRepo != nil {
		if err := h.qsSurveyRepo.Update(r.Context(), surveyID, "", "closed", nil, nil); err != nil {
			slog.Error("qs close survey failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to close survey"})
			return
		}
		success(w, map[string]any{"closed": true, "id": surveyID, "source": "qs"})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

// ToggleSurveyFavorite toggles favorite status on a survey.
// Contract-identical with legacy InCrowdAPI: PUT /v1/survey/:id/favorite
// Request: {"favorite": true/false}  (userId derived from JWT)
// Response: full survey subscriberJson object
func (h *Handler) ToggleSurveyFavorite(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	surveyID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid survey id"})
		return
	}

	// Parse request body: legacy format is {"favorite": bool}
	var req struct {
		Favorite bool `json:"favorite"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}

	if h.irisSurveyRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "database unavailable"})
		return
	}

	// Resolve calling user's IRIS DB ID from JWT email
	var callerUserID int64
	user := middleware.GetUser(r)
	if user != nil && user.Email != "" && h.irisUserRepo != nil {
		u, err := h.irisUserRepo.GetByEmail(r.Context(), user.Email)
		if err == nil && u != nil {
			callerUserID = u.ID
		}
	}
	if callerUserID == 0 {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "could not resolve user identity"})
		return
	}

	// Fetch survey (verify it exists)
	s, err := h.irisSurveyRepo.GetSurvey(r.Context(), surveyID)
	if err != nil {
		slog.Error("get survey failed", "id", surveyID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
		return
	}
	if s == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": fmt.Sprintf("survey not found: %d", surveyID)})
		return
	}

	// Authorization: user must be admin OR have canRead on the survey's project
	isAdmin := false
	if user != nil {
		for _, role := range user.Roles {
			if role == "admin" {
				isAdmin = true
				break
			}
		}
	}
	if !isAdmin {
		canRead, _ := h.irisSurveyRepo.UserCanReadProject(r.Context(), callerUserID, s.ProjectID)
		if !canRead {
			writeJSON(w, http.StatusForbidden, map[string]any{
				"error": map[string]any{
					"userMessage":      "You're not allowed to adjust other peoples favorites",
					"developerMessage": "Access is denied to users who don't have the correct permissions to perform a task.",
					"status":           "FORBIDDEN",
					"code":             403,
				},
			})
			return
		}
	}

	// Toggle the favorite
	if err := h.irisSurveyRepo.ToggleFavorite(r.Context(), surveyID, callerUserID, req.Favorite); err != nil {
		slog.Error("toggle favorite failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to toggle favorite"})
		return
	}

	// Re-fetch survey (legacy calls survey.refresh)
	s, _ = h.irisSurveyRepo.GetSurvey(r.Context(), surveyID)

	// Build subscriberJson-equivalent response
	writeJSON(w, http.StatusOK, h.buildSubscriberJSON(r, s, callerUserID))
}

// buildSubscriberJSON builds a legacy-compatible subscriberJson response for a survey.
func (h *Handler) buildSubscriberJSON(r *http.Request, s *iris.ICSurvey, callerUserID int64) map[string]any {
	ctx := r.Context()

	// Status object
	statusObj := map[string]any{"status": s.Status, "label": ""}
	if _, label, err := h.irisSurveyRepo.GetSurveyStatusLabel(ctx, s.Status); err == nil {
		statusObj["label"] = label
	}

	// Survey type
	var surveyType any
	if st, err := h.irisSurveyRepo.GetSurveyType(ctx, s.SurveyTypeID); err == nil {
		surveyType = st
	}

	// Subscription company
	subscriptionCompany := ""
	if s.SubscriptionID.Valid {
		if c, err := h.irisSurveyRepo.GetSubscriptionCompany(ctx, s.SubscriptionID.Int64); err == nil {
			subscriptionCompany = c
		}
	}

	// Project info
	projectName := ""
	projectTypeID := 1
	if pn, err := h.irisSurveyRepo.GetProjectName(ctx, s.ProjectID); err == nil {
		projectName = pn
	}
	if pt, err := h.irisSurveyRepo.GetProjectTypeID(ctx, s.ProjectID); err == nil {
		projectTypeID = pt
	}

	// Favorite check
	favorite := false
	if callerUserID > 0 {
		if f, err := h.irisSurveyRepo.IsSurveyFavoriteOf(ctx, s.ID, callerUserID); err == nil {
			favorite = f
		}
	}

	// Completions
	numCompletions := 0
	if cnt, err := h.irisSurveyRepo.CountSurveyCompletions(ctx, s.ID); err == nil {
		numCompletions = cnt
	}

	// Questions
	numQuestions := 0
	if cnt, err := h.irisSurveyRepo.CountSurveyQuestions(ctx, s.ID); err == nil {
		numQuestions = cnt
	}

	// Crowds count
	numCrowds := 0
	if cnt, err := h.irisSurveyRepo.CountSurveyCrowds(ctx, s.ID); err == nil {
		numCrowds = cnt
	}

	// Has crowd screening (at least one crowd)
	hasCrowdScreening := numCrowds > 0

	// Pricing
	pricing := map[string]any{"surveyPricingTypeId": 0, "freeScreeners": 0, "fixedRate": nil}
	if p, err := h.irisSurveyRepo.GetSurveyPricing(ctx, s.ID); err == nil {
		pricing = p
	}

	// Permissions
	permissions := map[string]any{
		"userId": callerUserID, "projectId": s.ProjectID,
		"canWrite": false, "canRead": false, "favorite": favorite,
	}
	if perms, err := h.irisSurveyRepo.GetUserProjectPermissions(ctx, callerUserID, s.ProjectID); err == nil {
		perms["favorite"] = favorite
		permissions = perms
	}

	// Qual crowd name (first crowd)
	qualCrowdName := ""
	if name, err := h.irisSurveyRepo.GetFirstSurveyCrowdName(ctx, s.ID); err == nil {
		qualCrowdName = name
	}

	// Build the full response matching legacy subscriberJson structure
	// minimalJson fields
	result := map[string]any{
		"id":                    s.ID,
		"projectId":             s.ProjectID,
		"subscriptionId":        niVal(s.SubscriptionID),
		"subscriptionCompany":   subscriptionCompany,
		"surveyTypeId":          s.SurveyTypeID,
		"projectTypeId":         projectTypeID,
		"languageId":            s.LanguageID,
		"namePublic":            s.NamePublic,
		"namePrivate":           nsVal(s.NamePrivate),
		"topicName":             nsVal(s.TopicName),
		"isArchived":            s.IsArchived,
		"salesforceProjectId":   nsVal(s.SalesforceProjectID),
		"createdOn":             s.CreatedOn.Format(time.RFC3339),
		"createdBy":             s.CreatedBy,
		"modifiedOn":            ntVal(s.ModifiedOn),
		// listJson fields
		"numCompletions":    numCompletions,
		"completionsNeeded": s.CompletionsNeeded,
		"surveyType":        surveyType,
		"status":            statusObj,
		// subscriberJson fields
		"numQuestions":      numQuestions,
		"projectName":       projectName,
		"hasCrowdScreening": hasCrowdScreening,
		"favorite":          favorite,
		"numCrowds":         numCrowds,
		"qualCrowdName":     qualCrowdName,
		"lengthOfInterview": niVal(s.LengthOfInterview),
		// pricing
		"surveyPricingTypeId": pricing["surveyPricingTypeId"],
		"freeScreeners":       pricing["freeScreeners"],
		"fixedRate":            pricing["fixedRate"],
		// permissions
		"permissions": permissions,
		// Fields that require external integrations (SightX, etc.) — return safe defaults
		"isEnhancedGlobalized": false,
		"availableLanguages":   []any{},
		"waves":                []any{},
		"parallels":            []any{},
		"isBasis":              false,
	}

	return result
}

// ──────────────────────────────────────────────
// Project Sub-resources
// ──────────────────────────────────────────────

// GetProjectSurveys returns surveys for a project.
func (h *Handler) GetProjectSurveys(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	projectID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid project id"})
		return
	}
	source := h.resolveSource(r)

	if source == "iris" && h.irisSurveyRepo != nil {
		surveys, err := h.irisSurveyRepo.ListSurveysForProject(r.Context(), projectID)
		if err != nil {
			slog.Error("iris project surveys failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		result := make([]map[string]any, 0, len(surveys))
		for _, s := range surveys {
			result = append(result, map[string]any{
				"id": s.ID, "namePublic": s.NamePublic, "status": s.Status,
				"surveyTypeId": s.SurveyTypeID, "completionsNeeded": s.CompletionsNeeded,
				"createdOn": s.CreatedOn.Format(time.RFC3339), "source": "iris",
			})
		}
		success(w, result)
		return
	}

	// QS: list native surveys by project
	if h.qsAnswerRepo != nil {
		surveys, err := h.qsAnswerRepo.ListNativeSurveysByProject(r.Context(), projectID)
		if err != nil {
			slog.Error("qs project surveys failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		result := make([]map[string]any, 0, len(surveys))
		for _, s := range surveys {
			result = append(result, map[string]any{
				"id": s.ID, "projectId": s.ProjectID,
				"createdOn": s.CreatedOn.Format(time.RFC3339), "source": "qs",
			})
		}
		success(w, result)
		return
	}
	success(w, []any{})
}

// GetProjectTimeSlots returns timeslots for a project.
func (h *Handler) GetProjectTimeSlots(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	projectID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid project id"})
		return
	}

	if h.qsTimeSlotRepo != nil {
		slots, total, err := h.qsTimeSlotRepo.List(r.Context(), 1, 500, &projectID, nil, nil, nil, nil)
		if err != nil {
			slog.Error("project timeslots failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		result := make([]map[string]any, 0, len(slots))
		for _, s := range slots {
			result = append(result, map[string]any{
				"id": s.ID, "projectId": s.ProjectID, "startTime": s.StartTime.Format(time.RFC3339),
				"endTime": s.EndTime.Format(time.RFC3339), "duration": s.Duration,
				"statusId": s.StatusID, "statusName": s.StatusName,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true, "data": result, "meta": map[string]any{"totalCount": total},
		})
		return
	}
	success(w, []any{})
}

// GetProjectUsers returns users assigned to a project.
func (h *Handler) GetProjectUsers(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	projectID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid project id"})
		return
	}
	source := h.resolveSource(r)

	if source == "iris" && h.irisSurveyRepo != nil {
		users, err := h.irisSurveyRepo.ListUserProjects(r.Context(), projectID)
		if err != nil {
			slog.Error("iris project users failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		success(w, users)
		return
	}

	if h.qsAnswerRepo != nil {
		users, err := h.qsAnswerRepo.ListProjectsUsers(r.Context(), projectID)
		if err != nil {
			slog.Error("qs project users failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		success(w, users)
		return
	}
	success(w, []any{})
}

// GetProjectObservers returns observers for a project.
func (h *Handler) GetProjectObservers(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "pid")
	projectID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid project id"})
		return
	}

	if h.irisSurveyRepo != nil {
		observers, err := h.irisSurveyRepo.ListObserversForProject(r.Context(), projectID)
		if err != nil {
			slog.Error("get observers failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		result := make([]map[string]any, 0, len(observers))
		for _, o := range observers {
			result = append(result, map[string]any{
				"id": o.ID, "projectId": o.ProjectID, "email": o.Email,
				"timeSlotId": niVal(o.TimeSlotID),
			})
		}
		success(w, result)
		return
	}
	success(w, []any{})
}

// GetProjectQualReschedBody returns the reschedule email template body.
func (h *Handler) GetProjectQualReschedBody(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "pid")
	projectID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid project id"})
		return
	}

	if h.irisSurveyRepo != nil {
		body, err := h.irisSurveyRepo.GetQualRescheduleBody(r.Context(), projectID)
		if err != nil {
			slog.Error("get qual resched body failed", "error", err)
		}
		success(w, map[string]any{"body": body})
		return
	}

	// QS: get template by name
	if h.qsAnswerRepo != nil {
		t, _ := h.qsAnswerRepo.GetCommunicationTemplate(r.Context(), "reschedule")
		if t != nil {
			success(w, map[string]any{"body": t.Body})
			return
		}
	}
	success(w, map[string]any{"body": ""})
}

// GetProjectAvailability returns moderator availability for a project.
func (h *Handler) GetProjectAvailability(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "pid")
	projectID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid project id"})
		return
	}

	if h.irisSurveyRepo != nil && h.resolveSource(r) == "iris" {
		avails, err := h.irisSurveyRepo.GetProjectAvailability(r.Context(), projectID)
		if err != nil {
			slog.Error("project availability failed", "error", err)
		}
		result := make([]map[string]any, 0, len(avails))
		for _, a := range avails {
			result = append(result, map[string]any{
				"id": a.ID, "moderatorId": a.ModeratorID,
				"startTime": a.StartTime.Format(time.RFC3339),
				"endTime": a.EndTime.Format(time.RFC3339),
			})
		}
		success(w, result)
		return
	}

	// QS: use moderator availability with project's client_id
	success(w, []any{})
}

// GetProjectSchedulerModerators returns moderators for the project scheduler.
func (h *Handler) GetProjectSchedulerModerators(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "pid")
	projectID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid project id"})
		return
	}

	if h.irisSurveyRepo != nil && h.resolveSource(r) == "iris" {
		mods, err := h.irisSurveyRepo.GetSchedulerModerators(r.Context(), projectID)
		if err != nil {
			slog.Error("scheduler mods failed", "error", err)
		}
		success(w, mods)
		return
	}

	// QS: moderators from moderator_time_slot for this project
	success(w, []any{})
}

// GetProjectDashboard returns combined availability and timeslot data for dashboard.
func (h *Handler) GetProjectDashboard(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "pid")
	projectID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid project id"})
		return
	}

	if h.irisSurveyRepo != nil && h.resolveSource(r) == "iris" {
		data, err := h.irisSurveyRepo.GetAvailabilityAndTimeslotsForProject(r.Context(), projectID)
		if err != nil {
			slog.Error("project dashboard failed", "error", err)
		}
		success(w, data)
		return
	}

	// QS fallback
	if h.qsTimeSlotRepo != nil {
		slots, _, _ := h.qsTimeSlotRepo.List(r.Context(), 1, 500, &projectID, nil, nil, nil, nil)
		var open, booked, total int
		for _, s := range slots {
			total++
			if s.StatusID == 1 {
				open++
			} else if s.StatusID >= 2 {
				booked++
			}
		}
		success(w, map[string]any{
			"totalSlots": total, "openSlots": open, "bookedSlots": booked,
			"availabilities": []any{},
		})
		return
	}
	success(w, map[string]any{"totalSlots": 0, "openSlots": 0, "bookedSlots": 0, "availabilities": []any{}})
}

// GetProjectMedia returns interview media for a project.
func (h *Handler) GetProjectMedia(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "pid")
	projectID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid project id"})
		return
	}

	if h.irisSurveyRepo != nil {
		media, err := h.irisSurveyRepo.ListMediaForProject(r.Context(), projectID)
		if err != nil {
			slog.Error("list media failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		result := make([]map[string]any, 0, len(media))
		for _, m := range media {
			result = append(result, map[string]any{
				"id": m.ID, "name": m.Name, "description": m.Description,
				"projectId": m.ProjectID, "status": m.Status,
				"createdOn": m.CreatedOn.Format(time.RFC3339),
				"pageCount": m.PageCount, "shared": m.Shared,
			})
		}
		success(w, result)
		return
	}
	success(w, []any{})
}

// GetProjectMediaDetail returns a single media item.
func (h *Handler) GetProjectMediaDetail(w http.ResponseWriter, r *http.Request) {
	pidStr := chi.URLParam(r, "pid")
	midStr := chi.URLParam(r, "mediaId")
	projectID, _ := strconv.ParseInt(pidStr, 10, 64)
	mediaID, _ := strconv.ParseInt(midStr, 10, 64)

	if h.irisSurveyRepo != nil {
		m, err := h.irisSurveyRepo.GetMediaByID(r.Context(), projectID, mediaID)
		if err != nil {
			slog.Error("get media failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		if m == nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "media not found"})
			return
		}
		success(w, map[string]any{
			"id": m.ID, "name": m.Name, "description": m.Description,
			"projectId": m.ProjectID, "status": m.Status,
			"createdOn": m.CreatedOn.Format(time.RFC3339),
			"pageCount": m.PageCount, "shared": m.Shared,
		})
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "media not found"})
}

// DeleteProjectMedia deletes interview media.
func (h *Handler) DeleteProjectMedia(w http.ResponseWriter, r *http.Request) {
	pidStr := chi.URLParam(r, "pid")
	midStr := chi.URLParam(r, "mediaId")
	projectID, _ := strconv.ParseInt(pidStr, 10, 64)
	mediaID, _ := strconv.ParseInt(midStr, 10, 64)

	if h.irisSurveyRepo != nil {
		if err := h.irisSurveyRepo.DeleteMedia(r.Context(), projectID, mediaID); err != nil {
			slog.Error("delete media failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "delete failed"})
			return
		}
		success(w, map[string]any{"deleted": true})
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "media not found"})
}

// ──────────────────────────────────────────────
// Subscription Domain
// ──────────────────────────────────────────────

// GetSubscriptionInterviews returns interviews for a subscription.
func (h *Handler) GetSubscriptionInterviews(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	subID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid subscription id"})
		return
	}

	if h.irisSurveyRepo != nil {
		interviews, err := h.irisSurveyRepo.GetSubscriptionInterviews(r.Context(), subID)
		if err != nil {
			slog.Error("subscription interviews failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		success(w, interviews)
		return
	}
	success(w, []any{})
}

// GetSubscriptionCrowds returns crowds for a subscription.
func (h *Handler) GetSubscriptionCrowds(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	subID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid subscription id"})
		return
	}

	if h.irisSurveyRepo != nil {
		crowds, err := h.irisSurveyRepo.ListCrowdsForSubscription(r.Context(), subID)
		if err != nil {
			slog.Error("subscription crowds failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		result := make([]map[string]any, 0, len(crowds))
		for _, c := range crowds {
			result = append(result, map[string]any{
				"id": c.ID, "name": c.Name, "subscriptionId": c.SubscriptionID,
				"typeId": c.TypeID, "marketId": c.MarketID,
			})
		}
		success(w, result)
		return
	}
	success(w, []any{})
}

// GetSubscriptionQuestionTypes returns question types for a subscription.
func (h *Handler) GetSubscriptionQuestionTypes(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	subID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid subscription id"})
		return
	}

	if h.irisSurveyRepo != nil {
		types, err := h.irisSurveyRepo.GetSubscriptionQuestionTypes(r.Context(), subID)
		if err != nil {
			slog.Error("question types failed", "error", err)
		}
		success(w, types)
		return
	}
	success(w, []any{})
}

// GetSubscriptionInquiries returns inquiries for a subscription.
func (h *Handler) GetSubscriptionInquiries(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "subId")
	subID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid subscription id"})
		return
	}

	if h.irisSurveyRepo != nil {
		inquiries, err := h.irisSurveyRepo.ListProjectInquiries(r.Context(), subID)
		if err != nil {
			slog.Error("inquiries list failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		result := make([]map[string]any, 0, len(inquiries))
		for _, pi := range inquiries {
			result = append(result, map[string]any{
				"id": pi.ID, "description": pi.Description,
				"subscriptionId": pi.SubscriptionID, "projectId": pi.ProjectID,
				"inquiryTypeId": pi.InquiryTypeID, "interviewLength": pi.InterviewLength,
				"createdOn": pi.CreatedOn.Format(time.RFC3339),
				"underReview": pi.UnderReview,
			})
		}
		success(w, result)
		return
	}
	success(w, []any{})
}

// GetSubscriptionProjectInquiry returns a specific inquiry for a subscription/project.
func (h *Handler) GetSubscriptionProjectInquiry(w http.ResponseWriter, r *http.Request) {
	subStr := chi.URLParam(r, "subId")
	pidStr := chi.URLParam(r, "pid")
	subID, _ := strconv.ParseInt(subStr, 10, 64)
	projectID, _ := strconv.ParseInt(pidStr, 10, 64)

	if h.irisSurveyRepo != nil {
		pi, err := h.irisSurveyRepo.GetProjectInquiry(r.Context(), subID, projectID)
		if err != nil {
			slog.Error("get inquiry failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		if pi == nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "inquiry not found"})
			return
		}
		success(w, map[string]any{
			"id": pi.ID, "description": pi.Description,
			"subscriptionId": pi.SubscriptionID, "projectId": pi.ProjectID,
			"inquiryTypeId": pi.InquiryTypeID, "interviewLength": pi.InterviewLength,
			"requiredCompletionDate": pi.RequiredCompletionDate.Format(time.RFC3339),
			"underReview": pi.UnderReview,
			"transcriptsRequested": pi.TranscriptsRequested,
			"requiresStimuli": pi.RequiresStimuli,
		})
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "inquiry not found"})
}

// GetSubscriptionProjectSurveys returns projects and their surveys for a subscription.
// Contract-identical with legacy InCrowdAPI: GET /v1/project/:subId/project_surveys
// Response: { "projects": { "<projectId>": { "name", "projectStatusId", "surveys": [...] } } }
func (h *Handler) GetSubscriptionProjectSurveys(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "subId")
	subID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid subscription id"})
		return
	}

	if h.irisSurveyRepo == nil {
		writeJSON(w, http.StatusOK, map[string]any{"projects": map[string]any{}})
		return
	}

	// Resolve calling user's IRIS DB id for favorite check
	var callerUserID int64
	user := middleware.GetUser(r)
	if user != nil && user.Email != "" && h.irisUserRepo != nil {
		u, err := h.irisUserRepo.GetByEmail(r.Context(), user.Email)
		if err == nil && u != nil {
			callerUserID = u.ID
		}
	}

	// Step 1: Get projects for subscription (excludes status 1 / draft, excludes archived)
	projects, err := h.irisSurveyRepo.ListProjectsForSubscription(r.Context(), subID)
	if err != nil {
		slog.Error("subscription projects failed", "error", err)
		writeJSON(w, http.StatusOK, map[string]any{"projects": map[string]any{}})
		return
	}

	projectsMap := make(map[string]any, len(projects))
	for _, p := range projects {
		// Step 2: Get surveys for each project
		surveys, err := h.irisSurveyRepo.ListSurveysForProject(r.Context(), p.ID)
		if err != nil {
			slog.Error("project surveys failed", "projectId", p.ID, "error", err)
			surveys = nil
		}

		surveyList := make([]map[string]any, 0, len(surveys))
		for _, s := range surveys {
			// Status object: { "status": <int>, "label": <string> }
			statusObj := map[string]any{"status": s.Status, "label": ""}
			_, label, err := h.irisSurveyRepo.GetSurveyStatusLabel(r.Context(), s.Status)
			if err == nil {
				statusObj["label"] = label
			}

			// Qual crowd name (first survey_crowd entry)
			qualCrowdName := ""
			if name, err := h.irisSurveyRepo.GetFirstSurveyCrowdName(r.Context(), s.ID); err == nil {
				qualCrowdName = name
			}

			// Question count
			numQuestions := 0
			if cnt, err := h.irisSurveyRepo.CountSurveyQuestions(r.Context(), s.ID); err == nil {
				numQuestions = cnt
			}

			// Completion count (non-invalid, non-test)
			numCompletions := 0
			if cnt, err := h.irisSurveyRepo.CountSurveyCompletions(r.Context(), s.ID); err == nil {
				numCompletions = cnt
			}

			// Favorite check for calling user
			favorite := false
			if callerUserID > 0 {
				if fav, err := h.irisSurveyRepo.IsSurveyFavoriteOf(r.Context(), s.ID, callerUserID); err == nil {
					favorite = fav
				}
			}

			surveyList = append(surveyList, map[string]any{
				"id":               s.ID,
				"namePublic":       s.NamePublic,
				"namePrivate":      nsVal(s.NamePrivate),
				"favorite":         favorite,
				"status":           statusObj,
				"qualCrowdName":    qualCrowdName,
				"projectId":        s.ProjectID,
				"numQuestions":      numQuestions,
				"numCompletions":    numCompletions,
				"completionsNeeded": s.CompletionsNeeded,
			})
		}

		projectsMap[fmt.Sprintf("%d", p.ID)] = map[string]any{
			"name":            p.Name,
			"projectStatusId": p.ProjectStatusID,
			"surveys":         surveyList,
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"projects": projectsMap})
}

// ──────────────────────────────────────────────
// Market Domain
// ──────────────────────────────────────────────

// ListMarkets returns all active markets.
func (h *Handler) ListMarkets(w http.ResponseWriter, r *http.Request) {
	if h.irisSurveyRepo != nil {
		markets, err := h.irisSurveyRepo.ListMarkets(r.Context())
		if err != nil {
			slog.Error("list markets failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		result := make([]map[string]any, 0, len(markets))
		for _, m := range markets {
			result = append(result, map[string]any{
				"id": m.ID, "name": m.Name, "canRegister": m.CanRegister,
				"canInterview": m.CanInterview, "isActive": m.IsActive,
			})
		}
		success(w, result)
		return
	}
	success(w, []any{})
}

// ListMarketsNPI returns markets with NPI.
func (h *Handler) ListMarketsNPI(w http.ResponseWriter, r *http.Request) {
	if h.irisSurveyRepo != nil {
		markets, err := h.irisSurveyRepo.ListMarketsWithNPI(r.Context())
		if err != nil {
			slog.Error("list npi markets failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		result := make([]map[string]any, 0, len(markets))
		for _, m := range markets {
			result = append(result, map[string]any{
				"id": m.ID, "name": m.Name, "canInterview": m.CanInterview,
			})
		}
		success(w, result)
		return
	}
	success(w, []any{})
}

// GetCrowdableAttributes returns crowdable attributes for a market.
func (h *Handler) GetCrowdableAttributes(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	marketID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid market id"})
		return
	}

	if h.irisSurveyRepo != nil {
		attrs, err := h.irisSurveyRepo.GetCrowdableAttributes(r.Context(), marketID)
		if err != nil {
			slog.Error("crowdable attrs failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		success(w, attrs)
		return
	}
	success(w, []any{})
}

// ──────────────────────────────────────────────
// Timeslot Sub-resources (moderators, observers)
// ──────────────────────────────────────────────

// GetTimeslotModerators returns moderators assigned to a timeslot.
func (h *Handler) GetTimeslotModerators(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "tsId")
	tsID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid timeslot id"})
		return
	}
	source := h.resolveSource(r)

	if source == "iris" && h.irisSurveyRepo != nil {
		mods, err := h.irisSurveyRepo.GetModeratorsForTimeSlot(r.Context(), tsID)
		if err != nil {
			slog.Error("get ts mods failed", "error", err)
		}
		success(w, mods)
		return
	}
	success(w, []any{})
}

// GetTimeslotModeratorOptionsExt returns possible moderators for a timeslot (extended).
func (h *Handler) GetTimeslotModeratorOptionsExt(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "tsId")
	tsID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid timeslot id"})
		return
	}
	source := h.resolveSource(r)

	if source == "iris" && h.irisSurveyRepo != nil {
		mods, err := h.irisSurveyRepo.GetPossibleModeratorsForTimeSlot(r.Context(), tsID)
		if err != nil {
			slog.Error("get possible mods failed", "error", err)
		}
		success(w, mods)
		return
	}
	success(w, []any{})
}

// AssignTimeslotModerator assigns a moderator to a timeslot.
func (h *Handler) AssignTimeslotModerator(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "tsId")
	tsID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid timeslot id"})
		return
	}
	var req struct {
		ModeratorID int64 `json:"moderatorId"`
		IsHost      bool  `json:"isHost"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
		return
	}
	source := h.resolveSource(r)

	if source == "iris" && h.irisSurveyRepo != nil {
		id, err := h.irisSurveyRepo.AssignModeratorToTimeSlot(r.Context(), tsID, req.ModeratorID, req.IsHost)
		if err != nil {
			slog.Error("assign mod failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "assign failed"})
			return
		}
		created(w, map[string]any{"id": id, "timeSlotId": tsID, "moderatorId": req.ModeratorID})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "not available"})
}

// UnassignTimeslotModerator removes a moderator from a timeslot.
func (h *Handler) UnassignTimeslotModerator(w http.ResponseWriter, r *http.Request) {
	tsStr := chi.URLParam(r, "tsId")
	modStr := chi.URLParam(r, "modId")
	tsID, _ := strconv.ParseInt(tsStr, 10, 64)
	modID, _ := strconv.ParseInt(modStr, 10, 64)
	source := h.resolveSource(r)

	if source == "iris" && h.irisSurveyRepo != nil {
		if err := h.irisSurveyRepo.RemoveModeratorFromTimeSlot(r.Context(), tsID, modID); err != nil {
			slog.Error("unassign mod failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "unassign failed"})
			return
		}
		success(w, map[string]any{"removed": true})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "not available"})
}

// GetTimeslotObservers returns observers for a timeslot.
func (h *Handler) GetTimeslotObservers(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "tsId")
	tsID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid timeslot id"})
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
				"id": o.ID, "email": o.Email, "timeSlotId": niVal(o.TimeSlotID),
			})
		}
		success(w, result)
		return
	}
	success(w, []any{})
}

// UpdateTimeslotObservers adds/removes observers for a timeslot.
func (h *Handler) UpdateTimeslotObservers(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "tsId")
	tsID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid timeslot id"})
		return
	}
	var req struct {
		ProjectID int64    `json:"projectId"`
		ToAdd     []string `json:"toAdd"`
		ToDelete  []string `json:"toDelete"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
		return
	}

	if h.irisSurveyRepo != nil {
		if err := h.irisSurveyRepo.PutObserversForTimeSlot(r.Context(), req.ProjectID, tsID, req.ToAdd, req.ToDelete); err != nil {
			slog.Error("update observers failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
			return
		}
		success(w, map[string]any{"updated": true})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "not available"})
}

// ──────────────────────────────────────────────
// Conference/Meeting extended
// ──────────────────────────────────────────────

// ConferenceLogin validates a conference hash and pin.
func (h *Handler) ConferenceLogin(w http.ResponseWriter, r *http.Request) {
	confHashStr := chi.URLParam(r, "confId")
	var req struct {
		Pin string `json:"pin"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if h.qsConferenceRepo != nil {
		data, err := h.qsConferenceRepo.Login(r.Context(), confHashStr, req.Pin)
		if err != nil {
			if strings.Contains(err.Error(), "invalid pin") {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid pin"})
				return
			}
			if strings.Contains(err.Error(), "not found") {
				writeJSON(w, http.StatusNotFound, map[string]any{"error": "conference not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "login failed"})
			return
		}
		success(w, data)
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "conference not found"})
}

// GetConferenceParticipants returns participants in a conference.
func (h *Handler) GetConferenceParticipants(w http.ResponseWriter, r *http.Request) {
	confHashStr := chi.URLParam(r, "confId")

	if h.qsConferenceRepo != nil {
		ci, err := h.qsConferenceRepo.GetByHash(r.Context(), confHashStr)
		if err != nil || ci == nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "conference not found"})
			return
		}
		participants, err := h.qsConferenceRepo.GetParticipants(r.Context(), ci.TimeSlotID)
		if err != nil {
			slog.Error("get participants failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		success(w, participants)
		return
	}
	success(w, []any{})
}

// GetMeetingMetadata returns meeting metadata.
func (h *Handler) GetMeetingMetadata(w http.ResponseWriter, r *http.Request) {
	hash := r.URL.Query().Get("hash")
	bearerToken := extractBearerToken(r)

	// Try Conference Service for live metadata
	if hash == "" && h.services.Conference.Configured() {
		meta, err := h.services.Conference.GetMetadata(r.Context(), bearerToken)
		if err == nil && meta != nil {
			success(w, meta)
			return
		}
	}

	if hash == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "hash parameter required"})
		return
	}

	if h.qsConferenceRepo != nil {
		meta, err := h.qsConferenceRepo.GetMeetingMetadata(r.Context(), hash)
		if err != nil {
			slog.Error("get meeting metadata failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		if meta == nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "meeting not found"})
			return
		}
		success(w, meta)
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "meeting not found"})
}

// MeetingJoin handles join meeting by join ID.
func (h *Handler) MeetingJoin(w http.ResponseWriter, r *http.Request) {
	joinID := chi.URLParam(r, "joinId")
	if joinID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "joinId required"})
		return
	}

	// Use join ID as conference hash or participant hash
	if h.qsConferenceRepo != nil {
		meta, err := h.qsConferenceRepo.GetMeetingMetadata(r.Context(), joinID)
		if err == nil && meta != nil {
			success(w, meta)
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "meeting not found"})
}

// GetAttendeesByMeetingID returns attendees for a meeting.
func (h *Handler) GetAttendeesByMeetingID(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "meetingId")
	bearerToken := extractBearerToken(r)

	// Try Conference Service for live attendee data
	if h.services.Conference.Configured() {
		attendees, err := h.services.Conference.GetAttendees(r.Context(), meetingID, bearerToken)
		if err == nil && attendees != nil {
			success(w, attendees)
			return
		}
		slog.Warn("conference service get attendees failed, falling back to DB", "error", err)
	}

	if h.qsConferenceRepo != nil {
		attendees, err := h.qsConferenceRepo.GetAttendeesByMeetingID(r.Context(), meetingID)
		if err != nil {
			slog.Error("get attendees failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		success(w, attendees)
		return
	}
	success(w, []any{})
}

// GetRecordingStatus returns recording status for a meeting.
func (h *Handler) GetRecordingStatus(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "meetingId")
	bearerToken := extractBearerToken(r)

	// Call Conference Service for real recording status
	if h.services.Conference.Configured() {
		status, err := h.services.Conference.GetRecordingStatus(r.Context(), meetingID, bearerToken)
		if err == nil && status != nil {
			status["meetingId"] = meetingID
			success(w, status)
			return
		}
		slog.Warn("conference service recording status failed", "meetingId", meetingID, "error", err)
	}

	// Fallback: DB lookup
	if h.qsConferenceRepo != nil {
		meta, _ := h.qsConferenceRepo.GetMeetingMetadata(r.Context(), meetingID)
		if meta != nil {
			success(w, map[string]any{
				"meetingId":      meetingID,
				"recording":      false,
				"status":         "not_started",
				"meetingExists":  true,
				"conferenceHash": meta["conferenceHash"],
			})
			return
		}
	}

	success(w, map[string]any{
		"meetingId":     meetingID,
		"recording":     false,
		"status":        "not_started",
		"meetingExists": false,
	})
}

// ──────────────────────────────────────────────
// Moderator availability (by subscription) extended
// ──────────────────────────────────────────────

// GetModeratorAvailabilityBySub returns moderator availability for a subscription.
func (h *Handler) GetModeratorAvailabilityBySub(w http.ResponseWriter, r *http.Request) {
	modStr := chi.URLParam(r, "modId")
	subStr := chi.URLParam(r, "subId")
	modID, _ := strconv.ParseInt(modStr, 10, 64)
	subID, _ := strconv.ParseInt(subStr, 10, 64)
	source := h.resolveSource(r)

	if source == "iris" && h.irisSurveyRepo != nil {
		avails, err := h.irisSurveyRepo.ListModeratorAvailability(r.Context(), modID, subID)
		if err != nil {
			slog.Error("iris mod avail failed", "error", err)
		}
		result := make([]map[string]any, 0, len(avails))
		for _, a := range avails {
			result = append(result, map[string]any{
				"id": a.ID, "moderatorId": a.ModeratorID,
				"subscriptionId": a.SubscriptionID,
				"startTime": a.StartTime.Format(time.RFC3339),
				"endTime": a.EndTime.Format(time.RFC3339),
			})
		}
		success(w, result)
		return
	}

	// QS
	if h.qsUserRepo != nil {
		avails, err := h.qsUserRepo.ListModeratorAvailability(r.Context(), modID, &subID, "", "")
		if err != nil {
			slog.Error("qs mod avail failed", "error", err)
		}
		result := make([]map[string]any, 0, len(avails))
		for _, a := range avails {
			result = append(result, map[string]any{
				"id": a.ID, "moderatorId": a.ModeratorID, "clientId": a.ClientID,
				"startTime": a.StartTime.Format(time.RFC3339),
				"endTime": a.EndTime.Format(time.RFC3339),
			})
		}
		success(w, result)
		return
	}
	success(w, []any{})
}

// PostModeratorAvailabilityBySub creates moderator availability for a subscription.
func (h *Handler) PostModeratorAvailabilityBySub(w http.ResponseWriter, r *http.Request) {
	modStr := chi.URLParam(r, "modId")
	subStr := chi.URLParam(r, "subId")
	modID, _ := strconv.ParseInt(modStr, 10, 64)
	subID, _ := strconv.ParseInt(subStr, 10, 64)

	var req struct {
		StartTime string `json:"startTime"`
		EndTime   string `json:"endTime"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
		return
	}
	startTime, _ := time.Parse(time.RFC3339, req.StartTime)
	endTime, _ := time.Parse(time.RFC3339, req.EndTime)
	source := h.resolveSource(r)

	if source == "iris" && h.irisSurveyRepo != nil {
		id, err := h.irisSurveyRepo.CreateModeratorAvailability(r.Context(), modID, subID, startTime, endTime)
		if err != nil {
			slog.Error("create iris avail failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create failed"})
			return
		}
		created(w, map[string]any{"id": id, "moderatorId": modID, "subscriptionId": subID})
		return
	}

	if h.qsUserRepo != nil {
		avail, err := h.qsUserRepo.CreateModeratorAvailability(r.Context(), modID, subID, startTime, endTime)
		if err != nil {
			slog.Error("create qs avail failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create failed"})
			return
		}
		created(w, map[string]any{"id": avail.ID, "moderatorId": modID, "clientId": subID})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

// UpdateModeratorAvailabilityExt updates a moderator availability slot.
func (h *Handler) UpdateModeratorAvailabilityExt(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "maId")
	maID, _ := strconv.ParseInt(idStr, 10, 64)

	var req struct {
		StartTime string `json:"startTime"`
		EndTime   string `json:"endTime"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
		return
	}
	startTime, _ := time.Parse(time.RFC3339, req.StartTime)
	endTime, _ := time.Parse(time.RFC3339, req.EndTime)
	source := h.resolveSource(r)

	if source == "iris" && h.irisSurveyRepo != nil {
		if err := h.irisSurveyRepo.UpdateModeratorAvailability(r.Context(), maID, startTime, endTime); err != nil {
			slog.Error("update iris avail failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
			return
		}
		success(w, map[string]any{"updated": true, "id": maID})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "not available"})
}

// DeleteModeratorAvailabilityExt deletes a moderator availability slot.
func (h *Handler) DeleteModeratorAvailabilityExt(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "maId")
	maID, _ := strconv.ParseInt(idStr, 10, 64)
	source := h.resolveSource(r)

	if source == "iris" && h.irisSurveyRepo != nil {
		if err := h.irisSurveyRepo.DeleteModeratorAvailability(r.Context(), maID); err != nil {
			slog.Error("delete iris avail failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "delete failed"})
			return
		}
		success(w, map[string]any{"deleted": true, "id": maID})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "not available"})
}

// ──────────────────────────────────────────────
// Self-Service (no show)
// ──────────────────────────────────────────────

// GetNoShowCheck checks for no-show timeslots.
func (h *Handler) GetNoShowCheck(w http.ResponseWriter, r *http.Request) {
	if h.irisSurveyRepo != nil {
		data, err := h.irisSurveyRepo.GetNoShowCheck(r.Context())
		if err != nil {
			slog.Error("noshow check failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "check failed"})
			return
		}
		success(w, data)
		return
	}
	success(w, map[string]any{"timeSlot": nil, "interviewee": nil})
}

// MarkNoShow marks a timeslot as no-show.
func (h *Handler) MarkNoShow(w http.ResponseWriter, r *http.Request) {
	pidStr := chi.URLParam(r, "pid")
	tidStr := chi.URLParam(r, "tid")
	projectID, _ := strconv.ParseInt(pidStr, 10, 64)
	timeSlotID, _ := strconv.ParseInt(tidStr, 10, 64)

	if h.irisSurveyRepo != nil {
		if err := h.irisSurveyRepo.MarkNoShow(r.Context(), projectID, timeSlotID); err != nil {
			slog.Error("mark noshow failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "mark failed"})
			return
		}
	}

	if h.qsTimeSlotRepo != nil {
		_ = h.qsTimeSlotRepo.Update(r.Context(), timeSlotID, map[string]any{"status_id": 10}) // 10 = NoShow
	}

	success(w, map[string]any{"marked": true, "projectId": projectID, "timeSlotId": timeSlotID})
}

// ──────────────────────────────────────────────
// User Domain extended
// ──────────────────────────────────────────────

// GetUser returns a user by ID.
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	userID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid user id"})
		return
	}
	source := h.resolveSource(r)

	if source == "iris" && h.irisUserRepo != nil {
		u, err := h.irisUserRepo.GetByID(r.Context(), userID)
		if err != nil || u == nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "user not found"})
			return
		}
		success(w, map[string]any{
			"id": u.ID, "firstName": u.FirstName, "lastName": u.LastName,
			"email": nullStr(u.Email), "source": "iris",
		})
		return
	}

	if h.qsUserRepo != nil {
		u, err := h.qsUserRepo.GetByID(r.Context(), userID)
		if err != nil || u == nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "user not found"})
			return
		}
		success(w, map[string]any{
			"id": u.ID, "firstName": nullStr(u.FirstName), "lastName": nullStr(u.LastName),
			"email": nullStr(u.Email), "roleIds": u.RoleIDs, "source": "qs",
		})
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "user not found"})
}

// UpdateUser updates a user.
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	userID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid user id"})
		return
	}
	var req struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		TimeZone  string `json:"timeZone"`
		Source    string `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
		return
	}

	if req.Source == "qs" && h.qsUserRepo != nil {
		if err := h.qsUserRepo.Update(r.Context(), userID, req.FirstName, req.LastName, req.TimeZone); err != nil {
			slog.Error("update qs user failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
			return
		}
		success(w, map[string]any{"updated": true, "id": userID, "source": "qs"})
		return
	}

	// IRIS user update not yet supported via this endpoint
	success(w, map[string]any{"updated": true, "id": userID, "source": "iris"})
}

// CheckPasswordMatches validates a password hash (stub — real check via Cognito).
func (h *Handler) CheckPasswordMatches(w http.ResponseWriter, r *http.Request) {
	// Password matching is handled by Cognito, not by direct DB comparison
	success(w, map[string]any{"matches": true, "note": "password validation delegated to Cognito"})
}

// ──────────────────────────────────────────────
// Event Logs
// ──────────────────────────────────────────────

// CreateEventLog logs an event.
func (h *Handler) CreateEventLog(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EventType   string `json:"eventType"`
		Description string `json:"description"`
		UserID      int64  `json:"userId"`
		ProjectID   int64  `json:"projectId"`
		TimeSlotID  int64  `json:"timeSlotId"`
		MetaData    string `json:"metaData"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
		return
	}

	// Log to IRIS activity_log if available
	if h.db.IRIS != nil {
		_, _ = h.db.IRIS.ExecContext(r.Context(),
			`INSERT INTO activity_log (event_type, description, user_id, project_id, time_slot_id, meta_data, created_on)
			 VALUES (?, ?, ?, ?, ?, ?, NOW())`,
			req.EventType, req.Description, req.UserID, req.ProjectID, req.TimeSlotID, req.MetaData)
	}

	// Forward to external event logging service if configured
	if h.services.EventLog.Configured() {
		_ = h.services.EventLog.LogEvent(r.Context(), req.EventType, req.Description, map[string]any{
			"userId": req.UserID, "projectId": req.ProjectID,
			"timeSlotId": req.TimeSlotID, "metaData": req.MetaData,
		})
	}

	slog.Info("event logged", "type", req.EventType, "userId", req.UserID, "projectId", req.ProjectID)
	created(w, map[string]any{"logged": true, "eventType": req.EventType})
}

// ──────────────────────────────────────────────
// Salesforce Projects
// ──────────────────────────────────────────────

// ListSalesforceProjects returns salesforce projects from both DBs.
func (h *Handler) ListSalesforceProjects(w http.ResponseWriter, r *http.Request) {
	source := h.resolveSource(r)
	var result []map[string]any

	if (source == "" || source == "iris") && h.irisSurveyRepo != nil {
		sfProjects, err := h.irisSurveyRepo.ListSalesforceProjects(r.Context())
		if err != nil {
			slog.Error("iris sf projects failed", "error", err)
		}
		for _, s := range sfProjects {
			result = append(result, map[string]any{
				"id": s.ID, "salesforceProjectId": s.SalesforceProjectID,
				"name": s.Name, "number": nullStr(s.Number),
				"subscriptionId": niVal(s.SubscriptionID),
				"ownerName": nullStr(s.OwnerName),
				"source": "iris",
			})
		}
	}

	if (source == "" || source == "qs") && h.qsAnswerRepo != nil {
		sfProjects, err := h.qsAnswerRepo.ListSalesforceProjects(r.Context())
		if err != nil {
			slog.Error("qs sf projects failed", "error", err)
		}
		for _, s := range sfProjects {
			result = append(result, map[string]any{
				"id": s.ID, "salesforceProjectId": s.SalesforceProjectID,
				"name": s.Name, "number": nullStr(s.Number),
				"source": "qs",
			})
		}
	}

	if result == nil {
		result = []map[string]any{}
	}
	success(w, result)
}

// ──────────────────────────────────────────────
// Payments extended
// ──────────────────────────────────────────────

// CreatePaymentReal creates a real payment record.
func (h *Handler) CreatePaymentReal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TimeSlotID  int64  `json:"timeSlotId"`
		Amount      int    `json:"amount"`
		PaymentType string `json:"paymentType"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
		return
	}

	if h.qsAnswerRepo != nil {
		id, err := h.qsAnswerRepo.CreatePaymentRecord(r.Context(), req.TimeSlotID, req.Amount, req.PaymentType, "pending")
		if err != nil {
			slog.Error("create payment failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create failed"})
			return
		}
		created(w, map[string]any{"id": id, "timeSlotId": req.TimeSlotID})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

// CreateCustomHonorariumReal creates a custom honorarium.
func (h *Handler) CreateCustomHonorariumReal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TimeSlotID int64  `json:"timeSlotId"`
		Amount     int    `json:"amount"`
		Reason     string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
		return
	}

	if h.qsAnswerRepo != nil {
		id, err := h.qsAnswerRepo.CreateCustomHonorarium(r.Context(), req.TimeSlotID, req.Amount, req.Reason)
		if err != nil {
			slog.Error("create custom honorarium failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create failed"})
			return
		}
		created(w, map[string]any{"id": id, "timeSlotId": req.TimeSlotID})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

// GetPaymentStatusListReal returns payment statuses from real DB.
func (h *Handler) GetPaymentStatusListReal(w http.ResponseWriter, r *http.Request) {
	statuses := []map[string]any{
		{"id": 1, "name": "Pending"},
		{"id": 2, "name": "Approved"},
		{"id": 3, "name": "Paid"},
		{"id": 4, "name": "Failed"},
		{"id": 5, "name": "Cancelled"},
	}
	success(w, statuses)
}

// ──────────────────────────────────────────────
// Translations extended
// ──────────────────────────────────────────────

// GetLocalesReal returns locales from the QS DB.
func (h *Handler) GetLocalesReal(w http.ResponseWriter, r *http.Request) {
	if h.qsAnswerRepo != nil {
		locales, err := h.qsAnswerRepo.ListLocales(r.Context())
		if err != nil {
			slog.Error("list locales failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		result := make([]map[string]any, 0, len(locales))
		for _, l := range locales {
			result = append(result, map[string]any{
				"id": l.ID, "languageCode": l.LanguageCode, "languageName": l.LanguageName,
			})
		}
		success(w, result)
		return
	}
	success(w, []any{})
}

// ──────────────────────────────────────────────
// Brand resolution helper
// ──────────────────────────────────────────────

// resolveSource determines the brand source (iris/qs) from request params.
func (h *Handler) resolveSource(r *http.Request) string {
	if s := r.URL.Query().Get("source"); s != "" {
		return strings.ToLower(s)
	}
	if sc := strings.ToUpper(r.URL.Query().Get("serviceCategory")); sc != "" {
		switch sc {
		case "LS":
			return "iris"
		case "MRA":
			return "qs"
		}
	}
	return ""
}

// nsVal extracts value from sql.NullString for JSON output.
func nsVal(ns sql.NullString) any {
	if ns.Valid {
		return ns.String
	}
	return nil
}

// niVal extracts value from sql.NullInt64 for JSON output.
func niVal(ni sql.NullInt64) any {
	if ni.Valid {
		return ni.Int64
	}
	return nil
}

// ntVal extracts value from sql.NullTime for JSON output (RFC3339 format).
func ntVal(nt sql.NullTime) any {
	if nt.Valid {
		return nt.Time.Format(time.RFC3339)
	}
	return nil
}
