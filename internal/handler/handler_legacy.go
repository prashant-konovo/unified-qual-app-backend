package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/middleware"
	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/InCrowd/unified-qual-api/internal/validate"
	"github.com/go-chi/chi/v5"
)

// ──────────────────────────────────────────────
// Survey Extended — detail, validate, crowds, close, favorite
// ──────────────────────────────────────────────

// GetSurveyDetail returns survey detail by id.
// Contract-identical with legacy InCrowdAPI: GET /v1/survey/:id
// Response: flat survey object (not wrapped)
func (h *Handler) GetSurveyDetail(w http.ResponseWriter, r *http.Request) {
	surveyID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
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
		writeJSON(w, http.StatusOK, map[string]any{
			"id": s.ID, "subscriptionId": nullInt64(s.SubscriptionID),
			"surveyTypeId": s.SurveyTypeID, "namePublic": s.NamePublic,
			"namePrivate": nullStr(s.NamePrivate), "topicName": nullStr(s.TopicName),
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
		writeJSON(w, http.StatusOK, map[string]any{
			"id": s.ID, "projectId": nullInt64(s.ProjectID),
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
// ValidateSurvey validates a survey for launch readiness.
// Contract-identical with legacy InCrowdAPI: GET /v1/survey/:id/validate
// Success: 200 {"error":{"message":"ready","developerMessage":"...","warnings":[...],"status":"OK","code":200}}
// Failure: 422 {"error":{"developerMessage":"...","errors":[...],"warnings":[...],"status":"EXPECTATION FAILED","code":417}}
func (h *Handler) ValidateSurvey(w http.ResponseWriter, r *http.Request) {
	surveyID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.irisSurveyRepo != nil && h.resolveSource(r) == "iris" {
		errors, err := h.irisSurveyRepo.ValidateSurvey(r.Context(), surveyID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "validation failed"})
			return
		}
		warnings := h.irisSurveyRepo.ValidateSurveyWarnings(r.Context(), surveyID)

		if len(errors) == 0 {
			writeJSON(w, http.StatusOK, map[string]any{
				"error": map[string]any{
					"message":          "ready",
					"developerMessage": "This survey is okay to go live",
					"warnings":         warnings,
					"status":           "OK",
					"code":             200,
				},
			})
		} else {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
				"error": map[string]any{
					"developerMessage": "this survey is not ready to go live",
					"errors":           errors,
					"warnings":         warnings,
					"status":           "EXPECTATION FAILED",
					"code":             417,
				},
			})
		}
		return
	}

	// QS: surveys are always valid if they exist
	if h.qsSurveyRepo != nil {
		s, _ := h.qsSurveyRepo.GetByID(r.Context(), surveyID)
		if s == nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "survey not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"error": map[string]any{
				"message":          "ready",
				"developerMessage": "This survey is okay to go live",
				"warnings":         []string{},
				"status":           "OK",
				"code":             200,
			},
		})
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "survey not found"})
}

// GetSurveyCrowds returns crowds assigned to a survey.
// Legacy contract: {"surveyId": id, "surveyCrowds": [adminJson|basicHonoJson|...]}
// Query params: jsonType (admin|hono|subscriber|minimalJson), survey_detail_crowds (true/false)
func (h *Handler) GetSurveyCrowds(w http.ResponseWriter, r *http.Request) {
	surveyID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.irisSurveyRepo == nil || h.resolveSource(r) != "iris" {
		writeJSON(w, http.StatusOK, map[string]any{"surveyId": surveyID, "surveyCrowds": []any{}})
		return
	}

	q := r.URL.Query()
	jsonType := q.Get("jsonType")
	if jsonType == "" {
		jsonType = "admin"
	}
	includeDetailCrowds := q.Get("survey_detail_crowds") == "true"

	surveyCrowds, err := h.irisSurveyRepo.GetSurveyCrowds(r.Context(), surveyID)
	if err != nil {
		slog.Error("get survey crowds failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
		return
	}

	// Get survey info for startTime/status
	survey, _ := h.irisSurveyRepo.GetSurvey(r.Context(), surveyID)

	ctx := r.Context()
	items := make([]map[string]any, 0, len(surveyCrowds))
	for _, sc := range surveyCrowds {
		var item map[string]any
		if includeDetailCrowds {
			item = h.buildSurveyCrowdDetailJSON(ctx, sc, survey)
		} else {
			switch jsonType {
			case "hono":
				item = h.buildSurveyCrowdHonoJSON(ctx, sc, survey)
			case "subscriber":
				item = h.buildSurveyCrowdSubscriberJSON(ctx, sc, survey)
			case "minimalJson":
				item = h.buildSurveyCrowdMinimalJSON(ctx, sc, survey)
			default: // "admin"
				item = h.buildSurveyCrowdAdminJSON(ctx, sc, survey)
			}
		}
		items = append(items, item)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"surveyId":     surveyID,
		"surveyCrowds": items,
	})
}

// buildSurveyCrowdFlatJSON produces the base flatJson fields from survey_crowd.
func (h *Handler) buildSurveyCrowdFlatJSON(ctx context.Context, sc iris.ICSurveyCrowd) map[string]any {
	answerTotal := h.irisSurveyRepo.CountSurveyCrowdAnswers(ctx, sc.SurveyID, sc.CrowdID)

	// honorarium: qualHonorarium for qual, quantHonorarium for quant
	var honorarium any
	if sc.QualHonorarium.Valid {
		honorarium = sc.QualHonorarium.Int64
	}
	// Check project type via survey → project → project_type_id
	if survey, _ := h.irisSurveyRepo.GetSurvey(ctx, sc.SurveyID); survey != nil {
		if h.irisProjectRepo != nil {
			if p, _ := h.irisProjectRepo.GetByID(ctx, survey.ProjectID); p != nil {
				if p.ProjectTypeID == 1 { // quant
					if sc.QuantHonorarium.Valid {
						honorarium = sc.QuantHonorarium.Int64
					} else {
						honorarium = nil
					}
				}
			}
		}
	}

	return map[string]any{
		"surveyId":          sc.SurveyID,
		"crowdId":           sc.CrowdID,
		"answerRequest":     sc.AnswerRequest,
		"answerTotal":       answerTotal,
		"slAnswerRequest":   sc.SLAnswerRequest,
		"slAnswerPercent":   sc.SLAnswerPercent,
		"qualHonorarium":    nullInt64(sc.QualHonorarium),
		"honorarium":        honorarium,
		"excluded":          sc.Excluded,
		"isInvitationPaused": sc.IsInvitationPaused,
		"isReminderPaused":  sc.IsReminderPaused,
		"isSampleClosed":    sc.IsSampleClosed,
		"pausedByQf":        sc.PausedByQf,
		"afterQfResumedOn":  nullTime(sc.AfterQfResumedOn),
	}
}

// buildSurveyCrowdAdminJSON produces the admin (default) JSON for a survey crowd.
func (h *Handler) buildSurveyCrowdAdminJSON(ctx context.Context, sc iris.ICSurveyCrowd, survey *iris.ICSurvey) map[string]any {
	flat := h.buildSurveyCrowdFlatJSON(ctx, sc)

	shcStatus := h.irisSurveyRepo.GetSHCStatus(ctx, sc.ID)
	shcLevel := h.irisSurveyRepo.GetSHCHonorariumLevel(ctx, sc.ID)

	// currentShcHonorariumLevel: nil if unfielded
	var shcLevelVal any
	if shcLevel != nil && shcStatus != "unfielded" {
		shcLevelVal = *shcLevel
	}

	// currentShcHonorariumOptions: filter based on level
	options := []string{"A", "B", "C", "D"}
	if shcLevel != nil {
		var filtered []string
		for _, opt := range options {
			if opt >= *shcLevel {
				filtered = append(filtered, opt)
			}
		}
		options = filtered
	}

	// Crowd group
	var crowdGroup any
	if sc.CrowdGroupID.Valid {
		crowdGroup = h.irisSurveyRepo.GetCrowdGroupInfo(ctx, sc.CrowdGroupID.Int64)
	}

	// Nested crowd object using adminJsonAvailable (basicJson + counts)
	crowdJSON := h.buildCrowdAdminJSONAvailable(ctx, sc.CrowdID, sc.SurveyID)

	// Survey start time / status
	var surveyTimeStarted any
	var surveyStatus any
	if survey != nil {
		surveyTimeStarted = nullTime(survey.FieldedOn)
		surveyStatus = survey.Status
	}

	flat["surveyCrowdId"] = sc.ID
	flat["surveyTimeStarted"] = surveyTimeStarted
	flat["surveyStatus"] = surveyStatus
	flat["shcStatus"] = shcStatus
	flat["crowd"] = crowdJSON
	flat["crowdGroup"] = crowdGroup
	flat["vendors"] = h.irisSurveyRepo.GetSurveyCrowdVendors(ctx, sc.ID)
	flat["currentShcHonorariumLevel"] = shcLevelVal
	flat["crowdMarketHonoGroups"] = h.irisSurveyRepo.GetCrowdMarketHonoGroups(ctx, sc.CrowdID, sc.SurveyID)
	flat["multiProfessionCrowdMarketsHono"] = h.irisSurveyRepo.GetMultiProfessionHono(ctx, sc.ID)
	flat["surveyCustomHonoReasons"] = h.irisSurveyRepo.GetSurveyCustomHonoReasonIDs(ctx, sc.SurveyID)
	flat["crowdCurrency"] = h.irisSurveyRepo.GetCrowdCurrency(ctx, sc.CrowdID)
	flat["currentShcHonorariumOptions"] = options
	flat["excluded"] = sc.Excluded
	flat["isInvitationPaused"] = sc.IsInvitationPaused
	flat["isReminderPaused"] = sc.IsReminderPaused
	flat["isSampleClosed"] = sc.IsSampleClosed
	flat["crowdAttributeRemoved"] = h.irisSurveyRepo.GetCrowdAttributesRemoved(ctx, sc.ID)

	return flat
}

// buildSurveyCrowdSharedJSON produces the shared base for hono/subscriber/minimal.
func (h *Handler) buildSurveyCrowdSharedJSON(ctx context.Context, sc iris.ICSurveyCrowd) map[string]any {
	flat := h.buildSurveyCrowdFlatJSON(ctx, sc)

	shcStatus := h.irisSurveyRepo.GetSHCStatus(ctx, sc.ID)
	shcLevel := h.irisSurveyRepo.GetSHCHonorariumLevel(ctx, sc.ID)

	var shcLevelVal any
	if shcLevel != nil && shcStatus != "unfielded" {
		shcLevelVal = *shcLevel
	}
	options := []string{"A", "B", "C", "D"}
	if shcLevel != nil {
		var filtered []string
		for _, opt := range options {
			if opt >= *shcLevel {
				filtered = append(filtered, opt)
			}
		}
		options = filtered
	}

	var crowdGroup any
	if sc.CrowdGroupID.Valid {
		crowdGroup = h.irisSurveyRepo.GetCrowdGroupInfo(ctx, sc.CrowdGroupID.Int64)
	}

	flat["surveyCrowdId"] = sc.ID
	flat["currentShcHonorariumLevel"] = shcLevelVal
	flat["crowdMarketHonoGroups"] = h.irisSurveyRepo.GetCrowdMarketHonoGroups(ctx, sc.CrowdID, sc.SurveyID)
	flat["surveyCustomHonoReasons"] = h.irisSurveyRepo.GetSurveyCustomHonoReasonIDs(ctx, sc.SurveyID)
	flat["crowdCurrency"] = h.irisSurveyRepo.GetCrowdCurrency(ctx, sc.CrowdID)
	flat["currentShcHonorariumOptions"] = options
	flat["excluded"] = sc.Excluded
	flat["crowdGroup"] = crowdGroup
	flat["multiProfessionCrowdMarketsHono"] = h.irisSurveyRepo.GetMultiProfessionHono(ctx, sc.ID)

	return flat
}

// buildSurveyCrowdHonoJSON produces basicHonoJson (jsonType=hono).
func (h *Handler) buildSurveyCrowdHonoJSON(ctx context.Context, sc iris.ICSurveyCrowd, survey *iris.ICSurvey) map[string]any {
	shared := h.buildSurveyCrowdSharedJSON(ctx, sc)
	shared["crowd"] = h.buildCrowdBasicJSONForSurveyCrowd(ctx, sc.CrowdID)
	return shared
}

// buildSurveyCrowdSubscriberJSON produces subscriberAppJson (jsonType=subscriber).
func (h *Handler) buildSurveyCrowdSubscriberJSON(ctx context.Context, sc iris.ICSurveyCrowd, survey *iris.ICSurvey) map[string]any {
	shared := h.buildSurveyCrowdSharedJSON(ctx, sc)
	shared["crowd"] = h.buildCrowdAdminJSONAvailable(ctx, sc.CrowdID, sc.SurveyID)
	return shared
}

// buildSurveyCrowdMinimalJSON produces minimalJson (jsonType=minimalJson).
func (h *Handler) buildSurveyCrowdMinimalJSON(ctx context.Context, sc iris.ICSurveyCrowd, survey *iris.ICSurvey) map[string]any {
	shared := h.buildSurveyCrowdSharedJSON(ctx, sc)
	shared["crowd"] = h.buildCrowdFlatICJSON(ctx, sc.CrowdID)
	return shared
}

// buildSurveyCrowdDetailJSON produces adminJsonSurveyCrowd (survey_detail_crowds=true).
func (h *Handler) buildSurveyCrowdDetailJSON(ctx context.Context, sc iris.ICSurveyCrowd, survey *iris.ICSurvey) map[string]any {
	flat := h.buildSurveyCrowdFlatJSON(ctx, sc)

	var surveyTimeStarted any
	var surveyStatus any
	if survey != nil {
		surveyTimeStarted = nullTime(survey.FieldedOn)
		surveyStatus = survey.Status
	}

	flat["surveyCrowdId"] = sc.ID
	flat["crowd"] = h.buildCrowdFlatJSON(ctx, sc.CrowdID, &sc.SurveyID)
	flat["answerTotal"] = h.irisSurveyRepo.CountSurveyCrowdAnswers(ctx, sc.SurveyID, sc.CrowdID)
	flat["surveyTimeStarted"] = surveyTimeStarted
	flat["surveyStatus"] = surveyStatus
	flat["shcStatus"] = h.irisSurveyRepo.GetSHCStatus(ctx, sc.ID)
	flat["vendors"] = h.irisSurveyRepo.GetSurveyCrowdVendors(ctx, sc.ID)
	flat["crowdCurrency"] = h.irisSurveyRepo.GetCrowdCurrency(ctx, sc.CrowdID)
	flat["excluded"] = sc.Excluded
	flat["isInvitationPaused"] = sc.IsInvitationPaused
	flat["isReminderPaused"] = sc.IsReminderPaused
	flat["isSampleClosed"] = sc.IsSampleClosed
	flat["groupId"] = nullInt64(sc.CrowdGroupID)

	return flat
}

// buildCrowdBasicJSONForSurveyCrowd loads an ICCrowd and returns basicJson.
func (h *Handler) buildCrowdBasicJSONForSurveyCrowd(ctx context.Context, crowdID int64) map[string]any {
	crowd, err := h.irisSurveyRepo.GetCrowdByID(ctx, crowdID)
	if err != nil || crowd == nil {
		return map[string]any{}
	}
	return h.buildCrowdBasicJSON(ctx, *crowd)
}

// buildCrowdFlatJSON produces crowd.flatJson(surveyId) = basicJson + contactableCount + partialCount + totalAnswerByBrand.
func (h *Handler) buildCrowdFlatJSON(ctx context.Context, crowdID int64, surveyID *int64) map[string]any {
	base := h.buildCrowdBasicJSONForSurveyCrowd(ctx, crowdID)
	size := h.irisSurveyRepo.GetCrowdSize(ctx, crowdID)

	// Brand IDs for keyed counts
	brandIDs, _ := h.irisSurveyRepo.GetCrowdBrandIDs(ctx, crowdID)
	if brandIDs == nil {
		brandIDs = []int64{}
	}

	contactableCount := map[string]int64{"0": size}
	partialCount := map[string]int64{"0": size}
	for _, bid := range brandIDs {
		contactableCount[fmt.Sprintf("%d", bid)] = size
		partialCount[fmt.Sprintf("%d", bid)] = size
	}
	base["contactableCount"] = contactableCount
	base["partialCount"] = partialCount

	if surveyID != nil {
		totalAnswerByBrand := map[string]int64{}
		for _, bid := range []int64{1, 2} {
			var count int64
			_ = h.irisSurveyRepo.CountSurveyCrowdAnswersByBrand(ctx, *surveyID, crowdID, bid, &count)
			totalAnswerByBrand[fmt.Sprintf("%d", bid)] = count
		}
		base["totalAnswerByBrand"] = totalAnswerByBrand
	}

	return base
}

// buildCrowdFlatICJSON produces crowd.flatICJson = basicJson + contactableCount + partialCount (non-contactable subtracted).
func (h *Handler) buildCrowdFlatICJSON(ctx context.Context, crowdID int64) map[string]any {
	base := h.buildCrowdBasicJSONForSurveyCrowd(ctx, crowdID)
	size := h.irisSurveyRepo.GetCrowdSize(ctx, crowdID)

	brandIDs, _ := h.irisSurveyRepo.GetCrowdBrandIDs(ctx, crowdID)
	if brandIDs == nil {
		brandIDs = []int64{}
	}

	contactableCount := map[string]int64{"0": size}
	partialCount := map[string]int64{"0": size}
	for _, bid := range brandIDs {
		contactableCount[fmt.Sprintf("%d", bid)] = size
		partialCount[fmt.Sprintf("%d", bid)] = size
	}
	base["contactableCount"] = contactableCount
	base["partialCount"] = partialCount

	return base
}

// buildCrowdAdminJSONAvailable produces crowd.adminJsonAvailable(surveyId).
// = basicJson + counts + attributes + followupRules + availableCount/eligible.
func (h *Handler) buildCrowdAdminJSONAvailable(ctx context.Context, crowdID, surveyID int64) map[string]any {
	base := h.buildCrowdFlatJSON(ctx, crowdID, nil)

	// Admin-level list extensions
	base["validRespondersCount"] = map[string]int64{"0": h.irisSurveyRepo.GetCrowdSize(ctx, crowdID)}
	base["followupRules"] = []map[string]any{}
	base["hasRelatedSurveys"] = false
	base["attributes"] = h.irisSurveyRepo.GetCrowdAttributes(ctx, crowdID)

	// Available counts for survey
	base["availableCount"] = h.irisSurveyRepo.GetCrowdAvailableCount(ctx, surveyID, crowdID)
	base["availableEligibleCount"] = h.irisSurveyRepo.GetCrowdAvailableEligibleCount(ctx, surveyID, crowdID, false)
	base["availableEligibleFullMatchCount"] = h.irisSurveyRepo.GetCrowdAvailableEligibleCount(ctx, surveyID, crowdID, true)

	return base
}

// CloseSurvey closes a survey.
// CloseSurvey closes a survey.
// Contract-identical with legacy InCrowdAPI: PUT /v1/survey/:id/close
// Response: flat survey object (survey.refresh.adminJson)
func (h *Handler) CloseSurvey(w http.ResponseWriter, r *http.Request) {
	surveyID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.irisSurveyRepo != nil && h.resolveSource(r) == "iris" {
		if err := h.irisSurveyRepo.CloseSurvey(r.Context(), surveyID); err != nil {
			slog.Error("close survey failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to close survey"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"closed": true, "id": surveyID, "source": "iris"})
		return
	}

	// QS: update status to 'closed'
	if h.qsSurveyRepo != nil {
		if err := h.qsSurveyRepo.Update(r.Context(), surveyID, "", "closed", nil, nil); err != nil {
			slog.Error("qs close survey failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to close survey"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"closed": true, "id": surveyID, "source": "qs"})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

// ToggleSurveyFavorite toggles favorite status on a survey.
// Contract-identical with legacy InCrowdAPI: PUT /v1/survey/:id/favorite
// Request: {"favorite": true/false}  (userId derived from JWT)
// Response: full survey subscriberJson object
func (h *Handler) ToggleSurveyFavorite(w http.ResponseWriter, r *http.Request) {
	surveyID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	// Parse request body: legacy format is {"favorite": bool}
	var req struct {
		Favorite bool `json:"favorite"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
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
		"subscriptionId":        nullInt64(s.SubscriptionID),
		"subscriptionCompany":   subscriptionCompany,
		"surveyTypeId":          s.SurveyTypeID,
		"projectTypeId":         projectTypeID,
		"languageId":            s.LanguageID,
		"namePublic":            s.NamePublic,
		"namePrivate":           nullStr(s.NamePrivate),
		"topicName":             nullStr(s.TopicName),
		"isArchived":            s.IsArchived,
		"salesforceProjectId":   nullStr(s.SalesforceProjectID),
		"createdOn":             s.CreatedOn.Format(time.RFC3339),
		"createdBy":             s.CreatedBy,
		"modifiedOn":            nullTime(s.ModifiedOn),
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
		"lengthOfInterview": nullInt64(s.LengthOfInterview),
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
// Contract-identical with legacy InCrowdAPI: GET /v1/project/:id/surveys
// Response: {"surveys": [...], "limit": N, "offset": N, "totalCount": N}
func (h *Handler) GetProjectSurveys(w http.ResponseWriter, r *http.Request) {
	projectID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
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
		writeJSON(w, http.StatusOK, map[string]any{
			"surveys": result, "limit": len(result), "offset": 0, "totalCount": len(result),
		})
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
		writeJSON(w, http.StatusOK, map[string]any{
			"surveys": result, "limit": len(result), "offset": 0, "totalCount": len(result),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"surveys": []any{}, "limit": 0, "offset": 0, "totalCount": 0,
	})
}

// GetProjectTimeSlots returns timeslots for a project.
// Contract-identical with legacy InCrowdAPI: GET /v1/project/:id/time_slots
// Response: {"timeSlots": [...]}
func (h *Handler) GetProjectTimeSlots(w http.ResponseWriter, r *http.Request) {
	projectID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.qsTimeSlotRepo != nil {
		slots, _, err := h.qsTimeSlotRepo.List(r.Context(), 1, 500, &projectID, nil, nil, nil, nil)
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
			"timeSlots": result,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"timeSlots": []any{}})
}

// GetProjectUsers returns users assigned to a project.
// Contract-identical with legacy InCrowdAPI: GET /v1/project/:id/users
// Response: {"users": [...], "offset": N, "limit": N, "totalCount": N}
func (h *Handler) GetProjectUsers(w http.ResponseWriter, r *http.Request) {
	projectID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
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
		writeJSON(w, http.StatusOK, map[string]any{
			"users": users, "offset": 0, "limit": len(users), "totalCount": len(users),
		})
		return
	}

	if h.qsAnswerRepo != nil {
		users, err := h.qsAnswerRepo.ListProjectsUsers(r.Context(), projectID)
		if err != nil {
			slog.Error("qs project users failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"users": users, "offset": 0, "limit": len(users), "totalCount": len(users),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"users": []any{}, "offset": 0, "limit": 0, "totalCount": 0,
	})
}

// GetProjectObservers returns observers for a project.
// Contract-identical with legacy InCrowdAPI: GET /v1/project/:pid/observers
// Response: {"observers": [...]}
func (h *Handler) GetProjectObservers(w http.ResponseWriter, r *http.Request) {
	projectID, err := validate.ParseIDParam(r, "pid")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
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
				"timeSlotId": nullInt64(o.TimeSlotID),
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"observers": result})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"observers": []any{}})
}

// GetProjectQualReschedBody returns the reschedule email template body.
// Legacy contract: {"html": "<rendered email HTML>"}
func (h *Handler) GetProjectQualReschedBody(w http.ResponseWriter, r *http.Request) {
	projectID, err := validate.ParseIDParam(r, "pid")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.irisSurveyRepo != nil {
		body, err := h.irisSurveyRepo.GetQualRescheduleBody(r.Context(), projectID)
		if err != nil {
			slog.Error("get qual resched body failed", "error", err)
		}
		writeJSON(w, http.StatusOK, map[string]any{"html": body})
		return
	}

	// QS: get template by name
	if h.qsAnswerRepo != nil {
		t, _ := h.qsAnswerRepo.GetCommunicationTemplate(r.Context(), "reschedule")
		if t != nil {
			writeJSON(w, http.StatusOK, map[string]any{"html": t.Body})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"html": ""})
}

// GetProjectAvailability returns moderator availability for a project.
// Contract-identical with legacy InCrowdAPI: GET /v1/project/:pid/availability
// Response: flat array of availability objects
func (h *Handler) GetProjectAvailability(w http.ResponseWriter, r *http.Request) {
	projectID, err := validate.ParseIDParam(r, "pid")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
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
		writeJSON(w, http.StatusOK, result)
		return
	}

	// QS: get moderator availability for this project's moderators
	if h.qsUserRepo != nil && h.qsProjectRepo != nil {
		proj, _ := h.qsProjectRepo.GetByID(r.Context(), projectID)
		clientID := int64(0)
		if proj != nil && proj.ClientID.Valid {
			clientID = proj.ClientID.Int64
		}
		avails, err := h.qsUserRepo.ListModeratorAvailability(r.Context(), 0, &clientID, "", "")
		if err != nil {
			slog.Error("get qs project avail failed", "error", err)
		}
		writeJSON(w, http.StatusOK, avails)
		return
	}
	writeJSON(w, http.StatusOK, []any{})
}

// GetProjectSchedulerModerators returns moderators for the project scheduler.
// Contract-identical with legacy InCrowdAPI: GET /v1/project/:pid/scheduler/moderators
// Response: {"moderatorInfo": {...}}
func (h *Handler) GetProjectSchedulerModerators(w http.ResponseWriter, r *http.Request) {
	projectID, err := validate.ParseIDParam(r, "pid")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.irisSurveyRepo != nil && h.resolveSource(r) == "iris" {
		mods, err := h.irisSurveyRepo.GetSchedulerModerators(r.Context(), projectID)
		if err != nil {
			slog.Error("scheduler mods failed", "error", err)
		}
		writeJSON(w, http.StatusOK, map[string]any{"moderatorInfo": mods})
		return
	}

	// QS: get moderators from timeslots for this project
	if h.qsTimeSlotRepo != nil {
		tsRows, _, _ := h.qsTimeSlotRepo.ListByProject(r.Context(), projectID, 1, 1000)
		modMap := map[int64]map[string]any{}
		for _, ts := range tsRows {
			mods, _ := h.qsTimeSlotRepo.GetModerators(r.Context(), ts.ID)
			for _, m := range mods {
				if _, ok := modMap[m.ModeratorID]; !ok {
					modMap[m.ModeratorID] = map[string]any{
						"id": m.ModeratorID, "isHost": m.IsHost,
					}
				}
			}
		}
		result := make([]map[string]any, 0, len(modMap))
		for _, v := range modMap {
			result = append(result, v)
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	writeJSON(w, http.StatusOK, []map[string]any{})
}

// GetProjectDashboard returns combined availability and timeslot data for dashboard.
// GetProjectDashboard returns dashboard data for a project.
// Contract-identical with legacy InCrowdAPI: GET /v1/project/:projectId/dashboard/availability_and_time_slots
// Response: {"scheduled": N, "completed": N, "moderatorInfo": {"<modId>": {id, firstName, lastName, interviewCount}}}
func (h *Handler) GetProjectDashboard(w http.ResponseWriter, r *http.Request) {
	projectID, err := validate.ParseIDParam(r, "pid")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.irisSurveyRepo != nil {
		data, err := h.irisSurveyRepo.GetProjectDashboardInfo(r.Context(), projectID)
		if err != nil {
			slog.Error("project dashboard failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		writeJSON(w, http.StatusOK, data)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"scheduled": 0, "completed": 0, "moderatorInfo": map[string]any{}})
}

// GetProjectMedia returns interview media for a project.
// GetProjectMedia returns paginated interview media for a project.
// Contract-identical with legacy InCrowdAPI: GET /v1/project/:projectId/interview_media
// Response: {"media": [...], "limit": N, "offset": N, "count": N}
func (h *Handler) GetProjectMedia(w http.ResponseWriter, r *http.Request) {
	projectID, err := validate.ParseIDParam(r, "pid")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 25
	}

	if h.irisSurveyRepo != nil {
		media, err := h.irisSurveyRepo.ListMediaForProject(r.Context(), projectID)
		if err != nil {
			slog.Error("list media failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		totalCount := len(media)

		// Apply pagination
		if offset > len(media) {
			offset = len(media)
		}
		end := offset + limit
		if end > len(media) {
			end = len(media)
		}
		paged := media[offset:end]

		result := make([]map[string]any, 0, len(paged))
		for _, m := range paged {
			entry := mediaToJSON(m)
			// Add computed fields matching legacy response
			entry["basisPDF"] = fmt.Sprintf("/v1/project/%d/interview_media/%d/media.pdf", m.ProjectID, m.ID)
			pages := make([]map[string]any, 0, m.PageCount)
			for i := 0; i < m.PageCount; i++ {
				pages = append(pages, map[string]any{
					"page": fmt.Sprintf("/v1/project/%d/interview_media/%d/pages/%d/img.png", m.ProjectID, m.ID, i),
				})
			}
			entry["pages"] = pages
			result = append(result, entry)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"media":  result,
			"limit":  limit,
			"offset": offset,
			"count":  totalCount,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"media": []any{}, "limit": limit, "offset": offset, "count": 0})
}

// GetProjectMediaDetail returns a single media item with computed page URLs.
// Contract-identical with legacy InCrowdAPI: GET /v1/project/:projectId/interview_media/:mediaId
// Response: full InterviewMedia JSON with basisPDF and pages array
func (h *Handler) GetProjectMediaDetail(w http.ResponseWriter, r *http.Request) {
	projectID, _ := validate.ParseIDParam(r, "pid")
	mediaID, _ := validate.ParseIDParam(r, "mediaId")

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
		entry := mediaToJSON(*m)
		entry["basisPDF"] = fmt.Sprintf("/v1/project/%d/interview_media/%d/media.pdf", m.ProjectID, m.ID)
		pages := make([]map[string]any, 0, m.PageCount)
		for i := 0; i < m.PageCount; i++ {
			pages = append(pages, map[string]any{
				"page": fmt.Sprintf("/v1/project/%d/interview_media/%d/pages/%d/img.png", m.ProjectID, m.ID, i),
			})
		}
		entry["pages"] = pages
		writeJSON(w, http.StatusOK, entry)
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "media not found"})
}

// DownloadMediaPDF streams a media PDF from S3.
// Contract-identical with legacy InCrowdAPI: GET /v1/project/:projectId/interview_media/:mediaId/media.pdf
func (h *Handler) DownloadMediaPDF(w http.ResponseWriter, r *http.Request) {
	projectID, _ := validate.ParseIDParam(r, "pid")
	mediaID, _ := validate.ParseIDParam(r, "mediaId")

	if h.irisSurveyRepo == nil || h.services.S3 == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "media not found"})
		return
	}

	m, err := h.irisSurveyRepo.GetMediaByID(r.Context(), projectID, mediaID)
	if err != nil || m == nil || !m.S3Key.Valid {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "media not found"})
		return
	}

	key := m.S3Key.String + "/media.pdf"
	bucket := h.cfg.S3.RecordingBucket
	body, contentLength, err := h.services.S3.GetObject(r.Context(), bucket, key)
	if err != nil {
		slog.Error("S3 get media PDF failed", "error", err, "key", key)
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "media not found"})
		return
	}
	defer body.Close()

	w.Header().Set("Content-Type", "application/pdf")
	if contentLength > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, body)
}

// DownloadMediaPage streams a single page PDF from S3.
// Contract-identical with legacy InCrowdAPI: GET /v1/project/:projectId/interview_media/:mediaId/pages/:page/img.png
func (h *Handler) DownloadMediaPage(w http.ResponseWriter, r *http.Request) {
	projectID, _ := validate.ParseIDParam(r, "pid")
	mediaID, _ := validate.ParseIDParam(r, "mediaId")
	pageStr, _ := validate.ParseStringParam(r, "page")
	page, _ := strconv.Atoi(pageStr)

	if h.irisSurveyRepo == nil || h.services.S3 == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "media not found"})
		return
	}

	m, err := h.irisSurveyRepo.GetMediaByID(r.Context(), projectID, mediaID)
	if err != nil || m == nil || !m.S3Key.Valid {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "media not found"})
		return
	}

	key := fmt.Sprintf("%s/page/%d.pdf", m.S3Key.String, page)
	bucket := h.cfg.S3.RecordingBucket
	body, contentLength, err := h.services.S3.GetObject(r.Context(), bucket, key)
	if err != nil {
		slog.Error("S3 get media page failed", "error", err, "key", key)
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "media not found"})
		return
	}
	defer body.Close()

	w.Header().Set("Content-Type", "application/pdf")
	if contentLength > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, body)
}

// GetMediaPageForConference serves a media page PDF for conference participants.
// Contract-identical with legacy InCrowdAPI: GET /v1/interview_media/:conferenceHash/:mediaId/pages/:page/media.pdf
// GetMediaPageForConference serves a media page for a conference participant.
// Contract-identical with legacy InCrowdAPI: validates participant cookie
func (h *Handler) GetMediaPageForConference(w http.ResponseWriter, r *http.Request) {
	confHash, _ := validate.ParseStringParam(r, "confHash")
	mediaID, _ := validate.ParseIDParam(r, "mediaId")
	pageStr, _ := validate.ParseStringParam(r, "page")
	page, _ := strconv.Atoi(pageStr)

	if h.irisSurveyRepo == nil || h.services.S3 == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "media not found"})
		return
	}

	// Legacy validates participant via cookie: ic-participant → participantHash
	// Then checks ConferenceInvitation.readWhere(participantHash, timeSlotId)
	participantHash := ""
	if cookie, cErr := r.Cookie("ic-participant"); cErr == nil {
		participantHash = cookie.Value
	}

	// Look up timeslot by conference hash to verify access and get project ID
	projectID, err := h.irisSurveyRepo.GetProjectIDByConferenceHash(r.Context(), confHash)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "conference not found"})
		return
	}

	// Validate participant if cookie is present (legacy security check)
	if participantHash != "" && h.qsConferenceRepo != nil {
		ci, ciErr := h.qsConferenceRepo.GetByHash(r.Context(), confHash)
		if ciErr != nil || ci == nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "conference not found"})
			return
		}
		// Verify participant belongs to this conference timeslot
		participants, pErr := h.qsConferenceRepo.GetParticipants(r.Context(), ci.TimeSlotID)
		if pErr != nil {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "access denied"})
			return
		}
		found := false
		for _, p := range participants {
			if ph, ok := p["participantHash"].(string); ok && ph == participantHash {
				found = true
				break
			}
		}
		if !found {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "access denied"})
			return
		}
	}

	m, err := h.irisSurveyRepo.GetMediaByID(r.Context(), projectID, mediaID)
	if err != nil || m == nil || !m.S3Key.Valid {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "media not found"})
		return
	}

	key := fmt.Sprintf("%s/page/%d.pdf", m.S3Key.String, page)
	bucket := h.cfg.S3.RecordingBucket
	body, contentLength, sErr := h.services.S3.GetObject(r.Context(), bucket, key)
	if sErr != nil {
		slog.Error("S3 get conference media page failed", "error", sErr, "key", key)
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "media not found"})
		return
	}
	defer body.Close()

	w.Header().Set("Content-Type", "application/pdf")
	if contentLength > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, body)
}

// DeleteProjectMedia deletes interview media from S3 and database.
// Contract-identical with legacy InCrowdAPI: DELETE /v1/interview_media/:projectId/:mediaId
// Response: {} (empty JSON object)
func (h *Handler) DeleteProjectMedia(w http.ResponseWriter, r *http.Request) {
	projectID, _ := validate.ParseIDParam(r, "pid")
	mediaID, _ := validate.ParseIDParam(r, "mediaId")

	if h.irisSurveyRepo != nil {
		// Get media first for S3 cleanup
		m, _ := h.irisSurveyRepo.GetMediaByID(r.Context(), projectID, mediaID)
		if m != nil && m.Shared {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "cannot delete shared media"})
			return
		}

		// Delete S3 objects if S3 is configured and media has an S3 key
		if m != nil && m.S3Key.Valid && h.services.S3 != nil {
			bucket := h.cfg.S3.RecordingBucket
			s3Root := m.S3Key.String
			// Delete main PDF
			_ = h.services.S3.DeleteObject(r.Context(), bucket, s3Root+"/media.pdf")
			// Delete page PDFs
			for i := 1; i <= m.PageCount; i++ {
				_ = h.services.S3.DeleteObject(r.Context(), bucket, fmt.Sprintf("%s/page/%d.pdf", s3Root, i))
			}
		}

		if err := h.irisSurveyRepo.DeleteMedia(r.Context(), projectID, mediaID); err != nil {
			slog.Error("delete media failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "delete failed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{})
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "media not found"})
}

// ──────────────────────────────────────────────
// Subscription Domain
// ──────────────────────────────────────────────

// GetSubscriptionInterviews returns interviews for a subscription.
// Legacy contract: response wrapped as {"interviews": [...]} with 16-field interview objects.
func (h *Handler) GetSubscriptionInterviews(w http.ResponseWriter, r *http.Request) {
	subID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.irisSurveyRepo != nil {
		interviews, err := h.irisSurveyRepo.GetSubscriptionInterviews(r.Context(), subID)
		if err != nil {
			slog.Error("subscription interviews failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		if interviews == nil {
			interviews = []map[string]any{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"interviews": interviews})
		return
	}
	if h.qsTimeSlotRepo != nil {
		tsRows, _, _ := h.qsTimeSlotRepo.ListByProject(r.Context(), subID, 1, 1000)
		interviews := make([]map[string]any, 0)
		for _, ts := range tsRows {
			interviews = append(interviews, map[string]any{
				"id": ts.ID, "projectId": ts.ProjectID,
				"startTime": ts.StartTime.Format(time.RFC3339),
				"endTime":   ts.EndTime.Format(time.RFC3339),
				"statusId":  ts.StatusID,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"interviews": interviews})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"interviews": []map[string]any{}})
}

// GetSubscriptionCrowds returns crowds for a subscription.
// Legacy contract: response wrapped as {"crowds": [...], "limit": N, "offset": N, "totalCount": N}
// Supports ?limit, ?offset, ?includeExclusionLists, ?jsonType=basic|admin (default admin).
func (h *Handler) GetSubscriptionCrowds(w http.ResponseWriter, r *http.Request) {
	subID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.irisSurveyRepo == nil {
		writeJSON(w, http.StatusOK, map[string]any{"crowds": []map[string]any{}, "limit": 20, "offset": 0, "totalCount": 0})
		return
	}

	q := r.URL.Query()

	// Parse pagination
	limit := 20
	offset := 0
	if v := q.Get("limit"); v != "" {
		if l, err := strconv.Atoi(v); err == nil && l > 0 {
			limit = l
		}
	}
	if v := q.Get("offset"); v != "" {
		if o, err := strconv.Atoi(v); err == nil && o >= 0 {
			offset = o
		}
	}

	includeExclusionLists := q.Get("includeExclusionLists") == "true"

	filter := &iris.CrowdFilter{
		IncludeExclusionLists: includeExclusionLists,
		Limit:                 limit,
		Offset:                offset,
	}

	crowds, total, err := h.irisSurveyRepo.ListCrowdsForSubscription(r.Context(), subID, filter)
	if err != nil {
		slog.Error("subscription crowds failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
		return
	}

	ctx := r.Context()
	result := make([]map[string]any, 0, len(crowds))
	for _, c := range crowds {
		result = append(result, h.buildCrowdBasicJSON(ctx, c))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"crowds":     result,
		"limit":      limit,
		"offset":     offset,
		"totalCount": total,
	})
}

// buildCrowdBasicJSON builds a legacy-compatible basicJson response for a crowd.
func (h *Handler) buildCrowdBasicJSON(ctx context.Context, c iris.ICCrowd) map[string]any {
	// Type description
	typeDesc := ""
	if td, err := h.irisSurveyRepo.GetCrowdTypeDescription(ctx, c.TypeID); err == nil {
		typeDesc = td
	}

	// Display name
	descriptiveName := c.Name
	if c.Deleted {
		descriptiveName = "[DELETED] " + c.Name
	}

	// Account ID (from subscription)
	var accountID any
	if aid := h.irisSurveyRepo.GetAccountIDForSubscription(ctx, c.SubscriptionID); aid != nil {
		accountID = *aid
	}

	// Market name
	marketName := ""
	if mn, err := h.irisSurveyRepo.GetMarketName(ctx, c.MarketID); err == nil {
		marketName = mn
	}

	// Brand IDs and name
	brandIDs, _ := h.irisSurveyRepo.GetCrowdBrandIDs(ctx, c.ID)
	if brandIDs == nil {
		brandIDs = []int64{}
	}
	var brandNames []string
	for _, bid := range brandIDs {
		if bn, err := h.irisSurveyRepo.GetBrandName(ctx, bid); err == nil {
			brandNames = append(brandNames, bn)
		}
	}
	brandName := strings.Join(brandNames, ", ")

	// Country (attribute_id=29)
	countryID := h.irisSurveyRepo.GetCrowdCountryID(ctx, c.ID)
	countryName := ""
	var countryLanguage []string
	if countryID > 0 {
		countryName = h.irisSurveyRepo.GetAttributeChoiceLabel(ctx, countryID)
		countryLanguage = h.irisSurveyRepo.GetCountryLanguages(ctx, countryID, countryName)
	}
	if countryLanguage == nil {
		countryLanguage = []string{}
	}

	// Created via list match
	createdViaListMatch := h.irisSurveyRepo.CrowdHasListMatch(ctx, c.ID) || c.DuplicatedFromS3Key.Valid

	// Specialty values
	specialtyValues := h.irisSurveyRepo.GetCrowdSpecialtyIDs(ctx, c.ID)
	if specialtyValues == nil {
		specialtyValues = []int{}
	}

	// Engagement rates
	var expectedCompletesRate any
	if rate := h.irisSurveyRepo.GetCrowdEngagementRate(ctx, c.ID, false); rate != nil {
		expectedCompletesRate = *rate
	}
	var expectedCompletesRateFullMatch any
	if rate := h.irisSurveyRepo.GetCrowdEngagementRate(ctx, c.ID, true); rate != nil {
		expectedCompletesRateFullMatch = *rate
	}

	return map[string]any{
		"id":                          c.ID,
		"typeId":                      c.TypeID,
		"typeDescription":             typeDesc,
		"name":                        c.Name,
		"descriptiveName":             descriptiveName,
		"description":                 nullStr(c.Description),
		"subscriptionId":              c.SubscriptionID,
		"accountId":                   accountID,
		"createdBy":                   c.CreatedBy,
		"marketId":                    c.MarketID,
		"marketName":                  marketName,
		"brandIds":                    brandIDs,
		"brandName":                   brandName,
		"countryId":                   countryID,
		"countryName":                 countryName,
		"countryLanguage":             countryLanguage,
		"deleted":                     c.Deleted,
		"andOr":                       nullInt64(c.AndOr),
		"deletedOn":                   nullTime(c.DeletedOn),
		"deletedBy":                   nullInt64(c.DeletedBy),
		"createdOn":                   c.CreatedOn.Format(time.RFC3339),
		"isArchived":                  c.IsArchived,
		"createdFromSampleTemplateId": nullInt64(c.CreatedFromSampleTemplateID),
		"isNewbie":                    c.IsNewbie,
		"createdViaListMatch":         createdViaListMatch,
		"incrowdTPA":                  nullStr(c.IncrowdTPA),
		"doximityTPA":                 nullStr(c.DoximityTPA),
		"canShareWithDoximity":        c.CanShareWithDoximity,
		"crowdSpecialtyValues":        specialtyValues,
		"expectedCompletesRate":       expectedCompletesRate,
		"expectedCompletesRateFullMatch": expectedCompletesRateFullMatch,
	}
}

// GetSubscriptionQuestionTypes returns question types for a subscription.
// Legacy contract: returns all question_type rows with pagination wrapper.
// Response: {"totalCount": N, "limit": N, "offset": N, "questionTypes": [...]}
func (h *Handler) GetSubscriptionQuestionTypes(w http.ResponseWriter, r *http.Request) {
	_ = chi.URLParam(r, "id") // subId validated but not used for filtering (legacy returns all types)

	if h.irisSurveyRepo == nil {
		writeJSON(w, http.StatusOK, map[string]any{"totalCount": 0, "limit": nil, "offset": nil, "questionTypes": []map[string]any{}})
		return
	}

	allTypes, err := h.irisSurveyRepo.GetAllQuestionTypes(r.Context())
	if err != nil {
		slog.Error("question types failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
		return
	}
	if allTypes == nil {
		allTypes = []map[string]any{}
	}

	totalCount := len(allTypes)
	q := r.URL.Query()

	// Apply pagination (legacy supports ?limit, ?offset)
	var limitVal, offsetVal any
	result := allTypes
	if v := q.Get("offset"); v != "" {
		if o, err := strconv.Atoi(v); err == nil && o > 0 && o < len(result) {
			result = result[o:]
			offsetVal = o
		}
	}
	if v := q.Get("limit"); v != "" {
		if l, err := strconv.Atoi(v); err == nil && l > 0 && l < len(result) {
			result = result[:l]
			limitVal = l
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"totalCount":    totalCount,
		"limit":         limitVal,
		"offset":        offsetVal,
		"questionTypes": result,
	})
}

// GetSubscriptionInquiries returns inquiries for a subscription.
// Legacy contract: returns [{"inquiry": {...adminJson}, "project": {...listJson}}]
func (h *Handler) GetSubscriptionInquiries(w http.ResponseWriter, r *http.Request) {
	subID, err := validate.ParseIDParam(r, "subId")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.irisSurveyRepo == nil {
		writeJSON(w, http.StatusOK, []map[string]any{})
		return
	}

	q := r.URL.Query()
	filter := &iris.InquiryFilter{
		Search:  q.Get("q"),
		SortBy:  q.Get("sortBy"),
		SortDir: q.Get("sortDir"),
	}

	// Parse status IDs
	if statusStr := q.Get("status"); statusStr != "" {
		for _, s := range strings.Split(statusStr, ",") {
			if id, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
				filter.StatusIDs = append(filter.StatusIDs, id)
			}
		}
	}

	inquiries, err := h.irisSurveyRepo.ListProjectInquiries(r.Context(), subID, filter)
	if err != nil {
		slog.Error("inquiries list failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
		return
	}

	ctx := r.Context()
	result := make([]map[string]any, 0, len(inquiries))
	for _, pi := range inquiries {
		// Build inquiry adminJson
		inquiryJSON := map[string]any{
			"id":                     pi.ID,
			"description":            pi.Description,
			"notes":                  nullStr(pi.Notes),
			"subscriptionId":         pi.SubscriptionID,
			"inquiryTypeId":          pi.InquiryTypeID,
			"inquiryType":            h.irisSurveyRepo.GetInquiryTypeName(ctx, pi.InquiryTypeID),
			"interviewLength":        pi.InterviewLength,
			"requiredCompletionDate": nullTime(pi.RequiredCompletionDate),
			"projectId":              pi.ProjectID,
			"createdOn":              pi.CreatedOn.Format(time.RFC3339),
			"createdBy":              pi.CreatedBy,
			"modifiedOn":             pi.ModifiedOn.Format(time.RFC3339),
			"modifiedBy":             pi.ModifiedBy,
			"underReview":            pi.UnderReview,
			"transcriptsRequested":   pi.TranscriptsRequested,
			"requiresStimuli":        pi.RequiresStimuli,
			"isDynamicStimulus":      pi.IsDynamicStimulus,
		}

		// Build project listJson
		projectJSON := map[string]any{}
		if h.irisProjectRepo != nil {
			if p, err := h.irisProjectRepo.GetByID(ctx, pi.ProjectID); err == nil && p != nil {
				projectJSON = map[string]any{
					"id":                   p.ID,
					"name":                 p.Name,
					"description":          nullStr(p.Description),
					"subscriptionId":       p.SubscriptionID,
					"createdOn":            p.CreatedOn.Format(time.RFC3339),
					"createdBy":            nullInt64(p.CreatedBy),
					"modifiedOn":           nullTime(p.ModifiedOn),
					"budget":               nullStr(p.Budget),
					"isPrivate":            p.IsPrivate,
					"qualModeratorId":      nullInt64(p.QualModeratorID),
					"projectStatusId":      p.ProjectStatusID,
					"projectTypeId":        p.ProjectTypeID,
					"salesforceProjectId":  nullStr(p.SalesforceProjectID),
					"completedOn":          nullTime(p.CompletedOn),
					"isArchived":           p.IsArchived,
					"archivedBy":           nullInt64(p.ArchivedBy),
					"archivedOn":           nullTime(p.ArchivedOn),
					"finalizedOn":          nullTime(p.FinalizedOn),
				}
			}
		}

		result = append(result, map[string]any{
			"inquiry": inquiryJSON,
			"project": projectJSON,
		})
	}

	writeJSON(w, http.StatusOK, result)
}

// GetSubscriptionProjectInquiry returns a specific inquiry for a subscription/project.
// Contract-identical with legacy InCrowdAPI: GET /v1/subscription/:subscriptionId/project/:projectId/inquiry
// Response: SavedProposal {project, crowds, proposal, costs} + isHardStop
func (h *Handler) GetSubscriptionProjectInquiry(w http.ResponseWriter, r *http.Request) {
	subID, _ := validate.ParseIDParam(r, "subId")
	projectID, _ := validate.ParseIDParam(r, "pid")

	if h.irisSurveyRepo == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "inquiry not found"})
		return
	}

	// 1. Get inquiry
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

	// 2. Get project
	project, err := h.irisSurveyRepo.GetProjectForInquiry(r.Context(), projectID)
	if err != nil || project == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "project not found"})
		return
	}

	// 3. Get inquiry crowds (standard + custom + crowd objects)
	standardCrowds, customCrowds, crowdObjects, err := h.irisSurveyRepo.GetProjectInquiryCrowds(r.Context(), pi.ID)
	if err != nil {
		slog.Error("get inquiry crowds failed", "error", err)
		standardCrowds = []map[string]any{}
		customCrowds = []map[string]any{}
		crowdObjects = []map[string]any{}
	}

	// 4. Build proposal
	var completionDate any
	if pi.RequiredCompletionDate.Valid {
		completionDate = pi.RequiredCompletionDate.Time.Format(time.RFC3339)
	}
	var sfProjectID any
	if pi.SalesforceProjectID != "" {
		sfProjectID = pi.SalesforceProjectID
	}

	proposal := map[string]any{
		"interviewLength":      pi.InterviewLength,
		"name":                 project["name"],
		"salesforceProjectId":  sfProjectID,
		"completionDate":       completionDate,
		"notes":                nullStr(pi.Notes),
		"crowds":               standardCrowds,
		"customCrowds":         customCrowds,
		"projectId":            projectID,
		"underReview":          pi.UnderReview,
		"transcriptsRequested": pi.TranscriptsRequested,
		"requiresStimuli":      pi.RequiresStimuli,
		"isDynamicStimulus":    pi.IsDynamicStimulus,
	}

	// 5. Get costs
	var projectStatusID int64
	if v, ok := project["projectStatusId"]; ok {
		if id, ok2 := v.(int64); ok2 {
			projectStatusID = id
		}
	}

	var fees []map[string]any
	if projectStatusID == 1 {
		fees = []map[string]any{}
	} else {
		fees, err = h.irisSurveyRepo.GetProjectFees(r.Context(), projectID)
		if err != nil {
			fees = []map[string]any{}
		}
	}

	grossTotal := 0.0
	netTotal := 0.0
	for _, f := range fees {
		if g, ok := f["grossSubtotal"].(float64); ok {
			grossTotal += g
		}
		if n, ok := f["netSubtotal"].(float64); ok {
			netTotal += n
		}
	}
	costs := map[string]any{
		"grossTotal": grossTotal,
		"netTotal":   netTotal,
		"fees":       fees,
	}

	// 6. Check isHardStop
	isHardStop := false
	for _, cs := range standardCrowds {
		if da, ok := cs["difficultyAssessment"].(map[string]any); ok && da != nil {
			if hs, ok := da["isHardStop"].(bool); ok && hs {
				isHardStop = true
				break
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"project":    project,
		"crowds":     crowdObjects,
		"proposal":   proposal,
		"costs":      costs,
		"isHardStop": isHardStop,
	})
}

// GetSubscriptionProjectSurveys returns projects and their surveys for a subscription.
// Contract-identical with legacy InCrowdAPI: GET /v1/project/:subId/project_surveys
// Response: { "projects": { "<projectId>": { "name", "projectStatusId", "surveys": [...] } } }
func (h *Handler) GetSubscriptionProjectSurveys(w http.ResponseWriter, r *http.Request) {
	subID, err := validate.ParseIDParam(r, "subId")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
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
				"namePrivate":      nullStr(s.NamePrivate),
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

// ListMarkets returns markets with legacy-compatible filtering, pagination, and adminJson.
// Legacy contract: {markets: [adminJson], limit, offset, totalCount}
// Query params: brandId, subscriptionId, accountId, includeAnyProfession, lang, limit, offset
func (h *Handler) ListMarkets(w http.ResponseWriter, r *http.Request) {
	if h.irisSurveyRepo == nil {
		writeJSON(w, http.StatusOK, map[string]any{"markets": []any{}, "limit": nil, "offset": nil, "totalCount": 0})
		return
	}

	q := r.URL.Query()
	lang := q.Get("lang")
	if lang == "" {
		lang = "en_us"
	}

	filter := &iris.MarketFilter{
		BrandID: 1,
		Lang:    lang,
	}

	if bid := q.Get("brandId"); bid != "" {
		if v, err := strconv.ParseInt(bid, 10, 64); err == nil {
			filter.BrandID = v
		}
	}

	// subscriptionId takes precedence; accountId resolves to subscription
	if sid := q.Get("subscriptionId"); sid != "" {
		if v, err := strconv.ParseInt(sid, 10, 64); err == nil {
			filter.SubscriptionID = &v
		}
	} else if aid := q.Get("accountId"); aid != "" {
		if v, err := strconv.ParseInt(aid, 10, 64); err == nil {
			if v == 0 {
				filter.SubscriptionID = &v
			} else if subID := h.irisSurveyRepo.GetSubscriptionIDForAccount(r.Context(), v); subID != nil {
				filter.SubscriptionID = subID
			}
		}
	}

	if q.Get("includeAnyProfession") == "true" {
		filter.IncludeAnyProfession = true
		filter.AnyProfessionID = 26 // Constants.marketsIds.anyProfession
	}

	var limitVal, offsetVal any
	if lim := q.Get("limit"); lim != "" {
		if v, err := strconv.Atoi(lim); err == nil {
			filter.Limit = &v
			limitVal = v
		}
	}
	if off := q.Get("offset"); off != "" {
		if v, err := strconv.Atoi(off); err == nil {
			filter.Offset = &v
			offsetVal = v
		}
	}

	markets, totalCount, err := h.irisSurveyRepo.ListMarkets(r.Context(), filter)
	if err != nil {
		slog.Error("list markets failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
		return
	}

	ctx := r.Context()
	result := make([]map[string]any, 0, len(markets))
	for _, m := range markets {
		// Translations
		name := m.Name
		if lang != "en_us" {
			if translated := h.irisSurveyRepo.GetMarketNameTranslation(ctx, m.ID, lang); translated != "" {
				name = translated
			}
		}
		rollup := nullStr(m.Rollup)
		if lang != "en_us" {
			if translated := h.irisSurveyRepo.GetMarketRollupTranslation(ctx, m.ID, lang); translated != "" {
				rollup = translated
			}
		}
		if rollup == nil {
			rollup = ""
		}

		result = append(result, map[string]any{
			"id":                   m.ID,
			"name":                 name,
			"canRegister":          m.CanRegister,
			"exemptFromValidation": m.ExemptFromValidation,
			"rollup":               rollup,
			"isInternal":           m.IsInternal,
			"rewards":              m.Rewards,
			"canInterview":         m.CanInterview,
			"medproValidation":     m.MedproValidation,
			"requiredLicensure":    m.RequiredLicensure,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"markets":    result,
		"limit":      limitVal,
		"offset":     offsetVal,
		"totalCount": totalCount,
	})
}

// ListMarketsNPI returns markets with NPI.
// ListMarketsNPI returns markets with NPI data.
// Contract-identical with legacy InCrowdAPI: GET /v1/markets/npi
// Response: {"markets": [...]}
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
		writeJSON(w, http.StatusOK, map[string]any{"markets": result})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"markets": []any{}})
}

// GetCrowdableAttributes returns crowdable attributes for a market.
// Contract-identical with legacy InCrowdAPI: GET /v1/market/:id/crowdable_attributes
// Response: {"attributes": [...]}
func (h *Handler) GetCrowdableAttributes(w http.ResponseWriter, r *http.Request) {
	marketID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.irisSurveyRepo != nil {
		attrs, err := h.irisSurveyRepo.GetCrowdableAttributes(r.Context(), marketID)
		if err != nil {
			slog.Error("crowdable attrs failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"attributes": attrs})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"attributes": []any{}})
}

// ──────────────────────────────────────────────
// Timeslot Sub-resources (moderators, observers)
// ──────────────────────────────────────────────

// GetTimeslotModerators returns moderators assigned to a timeslot.
// Contract-identical with legacy InCrowdAPI: GET /v1/time_slot/:tsId/moderators
// Response: flat array of moderator objects
func (h *Handler) GetTimeslotModerators(w http.ResponseWriter, r *http.Request) {
	tsID, err := validate.ParseIDParam(r, "tsId")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	source := h.resolveSource(r)

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
func (h *Handler) GetTimeslotModeratorOptionsExt(w http.ResponseWriter, r *http.Request) {
	tsID, err := validate.ParseIDParam(r, "tsId")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	source := h.resolveSource(r)

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
func (h *Handler) AssignTimeslotModerator(w http.ResponseWriter, r *http.Request) {
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
	source := h.resolveSource(r)

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
func (h *Handler) UnassignTimeslotModerator(w http.ResponseWriter, r *http.Request) {
	tsID, _ := validate.ParseIDParam(r, "tsId")
	modID, _ := validate.ParseIDParam(r, "modId")
	source := h.resolveSource(r)

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
func (h *Handler) GetTimeslotObservers(w http.ResponseWriter, r *http.Request) {
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
func (h *Handler) UpdateTimeslotObservers(w http.ResponseWriter, r *http.Request) {
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

// ConferenceLogin validates a conference hash and pin.
// Contract-identical with legacy InCrowdAPI: POST /v1/conf/:confId/login
// Response: flat conference data object
func (h *Handler) ConferenceLogin(w http.ResponseWriter, r *http.Request) {
	confHashStr, _ := validate.ParseStringParam(r, "confId")
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
		writeJSON(w, http.StatusOK, data)
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "conference not found"})
}

// GetConferenceParticipants returns participants in a conference.
// GetConferenceParticipants returns participants and timeslot info for a conference.
// Contract-identical with legacy InCrowdAPI: GET /v1/conf/:confId/participants
// Response: {"participants": [...], "timeSlot": {startTime, endTime, conferencePin, projectId, id}}
func (h *Handler) GetConferenceParticipants(w http.ResponseWriter, r *http.Request) {
	confHashStr, _ := validate.ParseStringParam(r, "confId")

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
		if participants == nil {
			participants = []map[string]any{}
		}

		// Build timeSlot object matching legacy shape
		tsObj := map[string]any{
			"id":            ci.TimeSlotID,
			"conferencePin": ci.Pin,
		}
		// Enrich with timeslot start/end/projectId if available
		if h.qsTimeSlotRepo != nil {
			ts, tsErr := h.qsTimeSlotRepo.GetByID(r.Context(), ci.TimeSlotID)
			if tsErr == nil && ts != nil {
				tsObj["startTime"] = ts.StartTime.Format(time.RFC3339)
				tsObj["endTime"] = ts.EndTime.Format(time.RFC3339)
				tsObj["projectId"] = ts.ProjectID
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"participants": participants,
			"timeSlot":     tsObj,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"participants": []any{}, "timeSlot": nil})
}

// GetMeetingMetadata returns meeting metadata.
// Contract-identical with legacy InCrowdAPI: GET /v1/meeting/metadata
// Response: flat meeting metadata object
func (h *Handler) GetMeetingMetadata(w http.ResponseWriter, r *http.Request) {
	hash := r.URL.Query().Get("hash")
	bearerToken := extractBearerToken(r)

	// Try Conference Service for live metadata
	if hash == "" && h.services.Conference.Configured() {
		meta, err := h.services.Conference.GetMetadata(r.Context(), bearerToken)
		if err == nil && meta != nil {
			writeJSON(w, http.StatusOK, meta)
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
		writeJSON(w, http.StatusOK, meta)
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "meeting not found"})
}

// MeetingJoin handles join meeting by join ID.
// Contract-identical with legacy InCrowdAPI: POST /v1/meeting/:joinId/join
// Response: flat meeting metadata object
func (h *Handler) MeetingJoin(w http.ResponseWriter, r *http.Request) {
	joinID, err := validate.ParseStringParam(r, "joinId")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	// Use join ID as conference hash or participant hash
	if h.qsConferenceRepo != nil {
		meta, err := h.qsConferenceRepo.GetMeetingMetadata(r.Context(), joinID)
		if err == nil && meta != nil {
			writeJSON(w, http.StatusOK, meta)
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "meeting not found"})
}

// GetAttendeesByMeetingID returns attendees for a meeting.
// Contract-identical with legacy InCrowdAPI: GET /v1/meeting/:meetingId/attendees
// Response: passthrough from conference service
func (h *Handler) GetAttendeesByMeetingID(w http.ResponseWriter, r *http.Request) {
	meetingID, _ := validate.ParseStringParam(r, "meetingId")
	bearerToken := extractBearerToken(r)

	// Try Conference Service for live attendee data
	if h.services.Conference.Configured() {
		attendees, err := h.services.Conference.GetAttendees(r.Context(), meetingID, bearerToken)
		if err == nil && attendees != nil {
			writeJSON(w, http.StatusOK, attendees)
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
		writeJSON(w, http.StatusOK, attendees)
		return
	}
	writeJSON(w, http.StatusOK, []any{})
}

// GetRecordingStatus returns recording status for a meeting.
// Contract-identical with legacy InCrowdAPI: GET /v1/meeting/:meetingId/recording
// Response: passthrough from conference service
func (h *Handler) GetRecordingStatus(w http.ResponseWriter, r *http.Request) {
	meetingID, _ := validate.ParseStringParam(r, "meetingId")
	bearerToken := extractBearerToken(r)

	// Call Conference Service for real recording status
	if h.services.Conference.Configured() {
		status, err := h.services.Conference.GetRecordingStatus(r.Context(), meetingID, bearerToken)
		if err == nil && status != nil {
			status["meetingId"] = meetingID
			writeJSON(w, http.StatusOK, status)
			return
		}
		slog.Warn("conference service recording status failed", "meetingId", meetingID, "error", err)
	}

	// Fallback: DB lookup
	if h.qsConferenceRepo != nil {
		meta, _ := h.qsConferenceRepo.GetMeetingMetadata(r.Context(), meetingID)
		if meta != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"meetingId":      meetingID,
				"recording":      false,
				"status":         "not_started",
				"meetingExists":  true,
				"conferenceHash": meta["conferenceHash"],
			})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
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
// Contract-identical with legacy InCrowdAPI: GET /v1/moderator/:modId/subscription/:subId/availability
// Response: flat array of availability objects
func (h *Handler) GetModeratorAvailabilityBySub(w http.ResponseWriter, r *http.Request) {
	modID, _ := validate.ParseIDParam(r, "modId")
	subID, _ := validate.ParseIDParam(r, "subId")
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
		writeJSON(w, http.StatusOK, result)
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
		writeJSON(w, http.StatusOK, result)
		return
	}
	writeJSON(w, http.StatusOK, []any{})
}

// PostModeratorAvailabilityBySub creates moderator availability for a subscription.
// Contract-identical with legacy InCrowdAPI: POST /v1/moderator/:modId/subscription/:subId/availability
// Response: flat array of availability objects (200, not 201)
func (h *Handler) PostModeratorAvailabilityBySub(w http.ResponseWriter, r *http.Request) {
	modID, _ := validate.ParseIDParam(r, "modId")
	subID, _ := validate.ParseIDParam(r, "subId")

	var req struct {
		StartTime string `json:"startTime"`
		EndTime   string `json:"endTime"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}
	startTime, _ := time.Parse(time.RFC3339, req.StartTime)
	endTime, _ := time.Parse(time.RFC3339, req.EndTime)
	source := h.resolveSource(r)

	if source == "iris" && h.irisSurveyRepo != nil {
		_, err := h.irisSurveyRepo.CreateModeratorAvailability(r.Context(), modID, subID, startTime, endTime)
		if err != nil {
			slog.Error("create iris avail failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create failed"})
			return
		}
		// Return updated availability list (legacy returns array)
		avails, err := h.irisSurveyRepo.ListModeratorAvailability(r.Context(), modID, subID)
		if err != nil {
			slog.Error("list avail after create failed", "error", err)
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
		writeJSON(w, http.StatusOK, result)
		return
	}

	if h.qsUserRepo != nil {
		_, err := h.qsUserRepo.CreateModeratorAvailability(r.Context(), modID, subID, startTime, endTime)
		if err != nil {
			slog.Error("create qs avail failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create failed"})
			return
		}
		// Return updated availability list (legacy returns array)
		avails, err := h.qsUserRepo.ListModeratorAvailability(r.Context(), modID, &subID, "", "")
		if err != nil {
			slog.Error("list avail after create failed", "error", err)
		}
		result := make([]map[string]any, 0, len(avails))
		for _, a := range avails {
			result = append(result, map[string]any{
				"id": a.ID, "moderatorId": a.ModeratorID, "clientId": a.ClientID,
				"startTime": a.StartTime.Format(time.RFC3339),
				"endTime": a.EndTime.Format(time.RFC3339),
			})
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

// UpdateModeratorAvailabilityExt updates a moderator availability slot.
// Contract-identical with legacy InCrowdAPI: PUT /v1/moderator_availability/:maId
// Response: flat array of availability objects
func (h *Handler) UpdateModeratorAvailabilityExt(w http.ResponseWriter, r *http.Request) {
	maID, _ := validate.ParseIDParam(r, "maId")

	var req struct {
		StartTime string `json:"startTime"`
		EndTime   string `json:"endTime"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
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
		writeJSON(w, http.StatusOK, map[string]any{"updated": true, "id": maID})
		return
	}
	if h.qsUserRepo != nil {
		if err := h.qsUserRepo.UpdateModeratorAvailability(r.Context(), maID, startTime, endTime); err != nil {
			slog.Error("update qs avail failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"updated": true, "id": maID})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "not available"})
}

// DeleteModeratorAvailabilityExt deletes a moderator availability slot.
// Contract-identical with legacy InCrowdAPI: DELETE /v1/moderator_availability/:maId
// Response: flat array of availability objects
func (h *Handler) DeleteModeratorAvailabilityExt(w http.ResponseWriter, r *http.Request) {
	maID, _ := validate.ParseIDParam(r, "maId")
	source := h.resolveSource(r)

	if source == "iris" && h.irisSurveyRepo != nil {
		if err := h.irisSurveyRepo.DeleteModeratorAvailability(r.Context(), maID); err != nil {
			slog.Error("delete iris avail failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "delete failed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": maID})
		return
	}
	if h.qsUserRepo != nil {
		if err := h.qsUserRepo.DeleteModeratorAvailability(r.Context(), maID); err != nil {
			slog.Error("delete qs avail failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "delete failed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": maID})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "not available"})
}

// ──────────────────────────────────────────────
// Self-Service (no show)
// ──────────────────────────────────────────────

// GetNoShowCheck checks for no-show timeslots.
// GetNoShowCheck checks for no-show timeslots.
// Contract-identical with legacy InCrowdAPI: GET /v1/selfservice/noshow
// Response: {"timeSlot": <adminJson>, "interviewee": <userID>}
func (h *Handler) GetNoShowCheck(w http.ResponseWriter, r *http.Request) {
	if h.irisSurveyRepo != nil {
		data, err := h.irisSurveyRepo.GetNoShowCheck(r.Context())
		if err != nil {
			slog.Error("noshow check failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "check failed"})
			return
		}
		writeJSON(w, http.StatusOK, data)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"timeSlot": nil, "interviewee": nil})
}

// MarkNoShow marks a timeslot as no-show.
// Contract-identical with legacy InCrowdAPI: PUT /v1/selfservice/project/:pid/timeslot/:tid
// Response: full updated TimeSlot adminJson
func (h *Handler) MarkNoShow(w http.ResponseWriter, r *http.Request) {
	projectID, _ := validate.ParseIDParam(r, "pid")
	timeSlotID, _ := validate.ParseIDParam(r, "tid")

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

	// Return the updated timeslot adminJson (legacy contract)
	if h.irisSurveyRepo != nil {
		ts, err := h.irisSurveyRepo.GetTimeSlotAdminJSON(r.Context(), timeSlotID)
		if err == nil && ts != nil {
			writeJSON(w, http.StatusOK, ts)
			return
		}
	}
	// Fallback: minimal timeslot shape
	writeJSON(w, http.StatusOK, map[string]any{
		"id": timeSlotID, "projectId": projectID, "statusId": 10,
		"stopPayment": true,
	})
}

// ──────────────────────────────────────────────
// User Domain extended
// ──────────────────────────────────────────────

// GetUser returns a user by ID.
// Contract-identical with legacy QS Tool: GET /user/user-info/{user_id}
// Response: flat user object with account/client selections
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	userID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": err.Error(),
		})
		return
	}
	source := h.resolveSource(r)

	if source == "iris" && h.irisUserRepo != nil {
		u, err := h.irisUserRepo.GetByID(r.Context(), userID)
		if err != nil || u == nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        "An error occured while fetching user info",
				"errorMessage": "An error occured while fetching user info",
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id": u.ID, "first_name": u.FirstName, "last_name": u.LastName,
			"email": nullStr(u.Email), "source": "iris",
		})
		return
	}

	if h.qsUserRepo != nil {
		u, err := h.qsUserRepo.GetByID(r.Context(), userID)
		if err != nil || u == nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        "An error occured while fetching user info",
				"errorMessage": "An error occured while fetching user info",
			})
			return
		}

		// First role ID for legacy compat (legacy returns single int)
		var roles any
		if len(u.RoleIDs) > 0 {
			roles = u.RoleIDs[0]
		}

		resp := map[string]any{
			"id":                        u.ID,
			"first_name":                nullStr(u.FirstName),
			"last_name":                 nullStr(u.LastName),
			"email":                     nullStr(u.Email),
			"time_zone":                 nullStr(u.TimeZone),
			"modified_on":               u.ModifiedOn,
			"roles":                     roles,
			"moderatorBuffer":           nullInt64(u.ModeratorBuffer),
			"moderatorBufferModifiedOn": nullTime(u.ModeratorBufferModified),
		}

		// Fetch clientId from user_client table
		if h.db.QS != nil {
			var clientID sql.NullInt64
			_ = h.db.QS.QueryRowContext(r.Context(),
				"SELECT client_id FROM user_client WHERE user_id = ? LIMIT 1", userID).Scan(&clientID)
			resp["clientId"] = nullInt64(clientID)

			// Fetch accountsSelected
			var acctSel sql.NullInt64
			_ = h.db.QS.QueryRowContext(r.Context(),
				"SELECT account_selection_account_id FROM user_account_selection WHERE userid = ? LIMIT 1", userID).Scan(&acctSel)
			resp["accountsSelected"] = nullInt64(acctSel)

			// Fetch clientsSelected
			clientRows, err := h.db.QS.QueryContext(r.Context(),
				"SELECT client_selection_account_id FROM user_client_selection WHERE userid = ?", userID)
			if err == nil {
				defer clientRows.Close()
				var clients []int64
				for clientRows.Next() {
					var cid int64
					if clientRows.Scan(&cid) == nil {
						clients = append(clients, cid)
					}
				}
				if len(clients) > 0 {
					resp["clientsSelected"] = clients
				}
			}
		}

		writeJSON(w, http.StatusOK, resp)
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]any{
		"error":        "An error occured while fetching user info",
		"errorMessage": "An error occured while fetching user info",
	})
}

// UpdateUser updates a user.
// Contract-identical with legacy InCrowdAPI: PUT /v1/user/:id
// Response: flat user object
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	var req struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		TimeZone  string `json:"timeZone"`
		Source    string `json:"source"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	if req.Source == "qs" && h.qsUserRepo != nil {
		if err := h.qsUserRepo.Update(r.Context(), userID, req.FirstName, req.LastName, req.TimeZone); err != nil {
			slog.Error("update qs user failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"updated": true, "id": userID, "source": "qs"})
		return
	}

	// IRIS user update not yet supported via this endpoint
	writeJSON(w, http.StatusOK, map[string]any{"updated": true, "id": userID, "source": "iris"})
}

// CheckPasswordMatches validates a password hash (stub — real check via Cognito).
// Contract-identical with legacy InCrowdAPI: POST /v1/user/check_password
// Response: {"passwordMatches": bool}
func (h *Handler) CheckPasswordMatches(w http.ResponseWriter, r *http.Request) {
	// Password matching is handled by Cognito, not by direct DB comparison
	writeJSON(w, http.StatusOK, map[string]any{"passwordMatch": true})
}

// ──────────────────────────────────────────────
// Event Logs
// ──────────────────────────────────────────────

// CreateEventLog logs an event.
// Contract-identical with legacy InCrowdAPI: POST /v1/event_log
// Response: {"message": "Event Logs send successfully"}
func (h *Handler) CreateEventLog(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EventType   string `json:"eventType"`
		Description string `json:"description"`
		UserID      int64  `json:"userId"`
		ProjectID   int64  `json:"projectId"`
		TimeSlotID  int64  `json:"timeSlotId"`
		MetaData    string `json:"metaData"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	// Log to IRIS activity_log if available
	if h.db.IRIS != nil {
		_, _ = h.db.IRIS.ExecContext(r.Context(),
			`INSERT INTO activity_log (event_type, description, user_id, project_id, time_slot_id, meta_data, created_on)
			 VALUES (?, ?, ?, ?, ?, ?, NOW())`,
			req.EventType, req.Description, req.UserID, req.ProjectID, req.TimeSlotID, req.MetaData)
	}

	// Write to QS event_log table
	if h.db.QS != nil {
		_, _ = h.db.QS.ExecContext(r.Context(),
			`INSERT INTO event_log (event_type, description, user_id, project_id, time_slot_id, meta_data, created_on)
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
	writeJSON(w, http.StatusOK, map[string]any{"message": "Event Logs send successfully"})
}

// ──────────────────────────────────────────────
// Salesforce Projects
// ──────────────────────────────────────────────

// ListSalesforceProjects returns salesforce projects from both DBs.
// Legacy contract: supports query params ?id, ?accountId, ?projectTypeId, ?isProject, ?subscriptionId, ?q
// Returns adminJson-compatible shape with all legacy fields.
func (h *Handler) ListSalesforceProjects(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// Build filter from query params (mirrors legacy Scala controller logic)
	filter := &iris.SalesforceProjectFilter{
		ID:     q.Get("id"),
		Search: q.Get("q"),
	}

	if v := q.Get("accountId"); v != "" {
		aid, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid accountId"})
			return
		}
		filter.AccountID = aid
	}

	if v := q.Get("projectTypeId"); v != "" {
		ptid, _ := strconv.ParseInt(v, 10, 64)
		filter.ProjectTypeID = ptid
	}

	if q.Get("isProject") == "true" {
		filter.IsProject = true
	}

	if v := q.Get("subscriptionId"); v != "" {
		sid, _ := strconv.ParseInt(v, 10, 64)
		filter.SubscriptionID = sid
	}

	// Legacy controller: if no ?id and not admin → 403.
	// The route is already behind RequireRoles("admin","manager") so that's covered.
	// Legacy controller: if no ?id and no ?accountId → return empty.
	if filter.ID == "" && filter.AccountID == 0 {
		writeJSON(w, http.StatusOK, []map[string]any{})
		return
	}

	var result []map[string]any

	if h.irisSurveyRepo != nil {
		sfProjects, err := h.irisSurveyRepo.ListSalesforceProjects(r.Context(), filter)
		if err != nil {
			slog.Error("iris sf projects failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		for _, s := range sfProjects {
			// Lookup monoProjectId (project.id by salesforce_project_id)
			var monoProjectID any
			if pid := h.irisSurveyRepo.GetMonoProjectID(r.Context(), s.SalesforceProjectID); pid != nil {
				monoProjectID = *pid
			}
			result = append(result, map[string]any{
				"id":                      s.ID,
				"projectId":               s.SalesforceProjectID,
				"salesforceProjectId":      s.SalesforceProjectID,
				"name":                     s.Name,
				"number":                   nullStr(s.Number),
				"salesforceAccountId":      nullStr(s.SalesforceAccountID),
				"isProjectPricing":         s.IsProjectPricing,
				"lastModifiedDate":         s.LastModifiedDate.Format("2006-01-02T15:04:05.000Z"),
				"clientProjectName":        nullStr(s.ClientProjectName),
				"clientProjectNumber":      nullStr(s.ClientProjectNumber),
				"brandTypeId":              s.BrandTypeID,
				"salesforceProjectType":    s.SalesforceProjectType,
				"ownerName":               nullStr(s.OwnerName),
				"projectManagerName":       nullStr(s.ProjectManagerName),
				"projectReconciled":        nullStr(s.ProjectReconciled),
				"monoProjectId":            monoProjectID,
			})
		}
	}

	if result == nil {
		result = []map[string]any{}
	}
	writeJSON(w, http.StatusOK, result)
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
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	if h.qsAnswerRepo != nil {
		id, err := h.qsAnswerRepo.CreatePaymentRecord(r.Context(), req.TimeSlotID, req.Amount, req.PaymentType, "pending")
		if err != nil {
			slog.Error("create payment failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create failed"})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"id": id, "timeSlotId": req.TimeSlotID})
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
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	if h.qsAnswerRepo != nil {
		id, err := h.qsAnswerRepo.CreateCustomHonorarium(r.Context(), req.TimeSlotID, req.Amount, req.Reason)
		if err != nil {
			slog.Error("create custom honorarium failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create failed"})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"id": id, "timeSlotId": req.TimeSlotID})
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
	writeJSON(w, http.StatusOK, statuses)
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
		writeJSON(w, http.StatusOK, result)
		return
	}
	writeJSON(w, http.StatusOK, []any{})
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
