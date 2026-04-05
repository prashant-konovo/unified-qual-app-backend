package ls

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/handler/support"
	"github.com/InCrowd/unified-qual-api/internal/dto"

	"github.com/InCrowd/unified-qual-api/internal/middleware"
	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/InCrowd/unified-qual-api/internal/validate"
)

// ──────────────────────────────────────────────
// LS Survey handlers
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Survey Extended — detail, validate, crowds, close, favorite
// ──────────────────────────────────────────────

// GetSurveyDetail returns survey detail by id.
// Contract-identical with legacy InCrowdAPI: GET /v1/survey/:id
// Response: flat survey object (not wrapped)
func (h *Handler) GetSurveyDetail(w http.ResponseWriter, r *http.Request) {
	surveyID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	source := support.ResolveSource(r)

	if source == "iris" && h.SurveyService.IrisAvailable() {
		s, err := h.SurveyService.GetSurvey(r.Context(), surveyID)
		if err != nil {
			slog.Error("iris survey get failed", "id", surveyID, "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		if s == nil {
			support.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "survey not found"})
			return
		}
		support.WriteJSON(w, http.StatusOK, map[string]any{
			"id": s.ID, "subscriptionId": dto.NullInt64(s.SubscriptionID),
			"surveyTypeId": s.SurveyTypeID, "namePublic": s.NamePublic,
			"namePrivate": dto.NullStr(s.NamePrivate), "topicName": dto.NullStr(s.TopicName),
			"projectId": s.ProjectID, "status": s.Status,
			"completionsNeeded": s.CompletionsNeeded,
			"createdOn":         s.CreatedOn.Format(time.RFC3339),
			"isArchived":        s.IsArchived, "languageId": s.LanguageID,
			"source": "iris", "serviceCategory": "LS",
		})
		return
	}

	// QS
	if h.SurveyService.QsAvailable() {
		s, err := h.SurveyService.GetByID(r.Context(), surveyID)
		if err != nil {
			slog.Error("qs survey get failed", "id", surveyID, "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		if s == nil {
			support.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "survey not found"})
			return
		}
		support.WriteJSON(w, http.StatusOK, map[string]any{
			"id": s.ID, "projectId": dto.NullInt64(s.ProjectID),
			"title": s.Title, "status": s.Status,
			"questions": json.RawMessage(s.Questions), "rules": json.RawMessage(s.Rules),
			"createdOn": s.CreatedOn.Format(time.RFC3339),
			"source":    "qs", "serviceCategory": "MRA",
		})
		return
	}
	support.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "survey not found"})
}

// ValidateSurvey checks if a survey can be fielded.
// ValidateSurvey validates a survey for launch readiness.
// Contract-identical with legacy InCrowdAPI: GET /v1/survey/:id/validate
// Success: 200 {"error":{"message":"ready","developerMessage":"...","warnings":[...],"status":"OK","code":200}}
// Failure: 422 {"error":{"developerMessage":"...","errors":[...],"warnings":[...],"status":"EXPECTATION FAILED","code":417}}
func (h *Handler) ValidateSurvey(w http.ResponseWriter, r *http.Request) {
	surveyID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.SurveyService.IrisAvailable() && support.ResolveSource(r) == "iris" {
		errors, err := h.SurveyService.ValidateSurvey(r.Context(), surveyID)
		if err != nil {
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "validation failed"})
			return
		}
		warnings := h.SurveyService.ValidateSurveyWarnings(r.Context(), surveyID)

		// Hook: SL completions calculation + basis survey propagation
		// Matches Scala: when brandType=2, slEligible=1, slCompletionsNeeded=0
		if err := h.SurveyService.CalculateSLCompletions(r.Context(), surveyID); err != nil {
			slog.Warn("hook: SL completions calculation failed (non-fatal)", "surveyId", surveyID, "error", err)
		}

		if len(errors) == 0 {
			support.WriteJSON(w, http.StatusOK, map[string]any{
				"error": map[string]any{
					"message":          "ready",
					"developerMessage": "This survey is okay to go live",
					"warnings":         warnings,
					"status":           "OK",
					"code":             200,
				},
			})
		} else {
			support.WriteJSON(w, http.StatusUnprocessableEntity, map[string]any{
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
	if h.SurveyService.QsAvailable() {
		s, _ := h.SurveyService.GetByID(r.Context(), surveyID)
		if s == nil {
			support.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "survey not found"})
			return
		}
		support.WriteJSON(w, http.StatusOK, map[string]any{
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
	support.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "survey not found"})
}

// GetSurveyCrowds returns crowds assigned to a survey.
// Legacy contract: {"surveyId": id, "surveyCrowds": [adminJson|basicHonoJson|...]}
// Query params: jsonType (admin|hono|subscriber|minimalJson), survey_detail_crowds (true/false)
func (h *Handler) GetSurveyCrowds(w http.ResponseWriter, r *http.Request) {
	surveyID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if !h.SurveyService.IrisAvailable() || support.ResolveSource(r) != "iris" {
		support.WriteJSON(w, http.StatusOK, map[string]any{"surveyId": surveyID, "surveyCrowds": []any{}})
		return
	}

	q := r.URL.Query()
	jsonType := q.Get("jsonType")
	if jsonType == "" {
		jsonType = "admin"
	}
	includeDetailCrowds := q.Get("survey_detail_crowds") == "true"

	surveyCrowds, err := h.SurveyService.GetSurveyCrowds(r.Context(), surveyID)
	if err != nil {
		slog.Error("get survey crowds failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
		return
	}

	// Get survey info for startTime/status
	survey, _ := h.SurveyService.GetSurvey(r.Context(), surveyID)

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

	support.WriteJSON(w, http.StatusOK, map[string]any{
		"surveyId":     surveyID,
		"surveyCrowds": items,
	})
}

// buildSurveyCrowdFlatJSON produces the base flatJson fields from survey_crowd.
func (h *Handler) buildSurveyCrowdFlatJSON(ctx context.Context, sc iris.ICSurveyCrowd) map[string]any {
	answerTotal := h.SurveyService.CountSurveyCrowdAnswers(ctx, sc.SurveyID, sc.CrowdID)

	// honorarium: qualHonorarium for qual, quantHonorarium for quant
	var honorarium any
	if sc.QualHonorarium.Valid {
		honorarium = sc.QualHonorarium.Int64
	}
	// Check project type via survey → project → project_type_id
	if survey, _ := h.SurveyService.GetSurvey(ctx, sc.SurveyID); survey != nil {
		if h.IrisProjectRepo != nil {
			if p, _ := h.IrisProjectRepo.GetByID(ctx, survey.ProjectID); p != nil {
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
		"surveyId":           sc.SurveyID,
		"crowdId":            sc.CrowdID,
		"answerRequest":      sc.AnswerRequest,
		"answerTotal":        answerTotal,
		"slAnswerRequest":    sc.SLAnswerRequest,
		"slAnswerPercent":    sc.SLAnswerPercent,
		"qualHonorarium":     dto.NullInt64(sc.QualHonorarium),
		"honorarium":         honorarium,
		"excluded":           sc.Excluded,
		"isInvitationPaused": sc.IsInvitationPaused,
		"isReminderPaused":   sc.IsReminderPaused,
		"isSampleClosed":     sc.IsSampleClosed,
		"pausedByQf":         sc.PausedByQf,
		"afterQfResumedOn":   dto.NullTime(sc.AfterQfResumedOn),
	}
}

// buildSurveyCrowdAdminJSON produces the admin (default) JSON for a survey crowd.
func (h *Handler) buildSurveyCrowdAdminJSON(ctx context.Context, sc iris.ICSurveyCrowd, survey *iris.ICSurvey) map[string]any {
	flat := h.buildSurveyCrowdFlatJSON(ctx, sc)

	shcStatus := h.SurveyService.GetSHCStatus(ctx, sc.ID)
	shcLevel := h.SurveyService.GetSHCHonorariumLevel(ctx, sc.ID)

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
		crowdGroup = h.SurveyService.GetCrowdGroupInfo(ctx, sc.CrowdGroupID.Int64)
	}

	// Nested crowd object using adminJsonAvailable (basicJson + counts)
	crowdJSON := h.buildCrowdAdminJSONAvailable(ctx, sc.CrowdID, sc.SurveyID)

	// Survey start time / status
	var surveyTimeStarted any
	var surveyStatus any
	if survey != nil {
		surveyTimeStarted = dto.NullTime(survey.FieldedOn)
		surveyStatus = survey.Status
	}

	flat["surveyCrowdId"] = sc.ID
	flat["surveyTimeStarted"] = surveyTimeStarted
	flat["surveyStatus"] = surveyStatus
	flat["shcStatus"] = shcStatus
	flat["crowd"] = crowdJSON
	flat["crowdGroup"] = crowdGroup
	flat["vendors"] = h.SurveyService.GetSurveyCrowdVendors(ctx, sc.ID)
	flat["currentShcHonorariumLevel"] = shcLevelVal
	flat["crowdMarketHonoGroups"] = h.SurveyService.GetCrowdMarketHonoGroups(ctx, sc.CrowdID, sc.SurveyID)
	flat["multiProfessionCrowdMarketsHono"] = h.SurveyService.GetMultiProfessionHono(ctx, sc.ID)
	flat["surveyCustomHonoReasons"] = h.SurveyService.GetSurveyCustomHonoReasonIDs(ctx, sc.SurveyID)
	flat["crowdCurrency"] = h.SurveyService.GetCrowdCurrency(ctx, sc.CrowdID)
	flat["currentShcHonorariumOptions"] = options
	flat["excluded"] = sc.Excluded
	flat["isInvitationPaused"] = sc.IsInvitationPaused
	flat["isReminderPaused"] = sc.IsReminderPaused
	flat["isSampleClosed"] = sc.IsSampleClosed
	flat["crowdAttributeRemoved"] = h.SurveyService.GetCrowdAttributesRemoved(ctx, sc.ID)

	return flat
}

// buildSurveyCrowdSharedJSON produces the shared base for hono/subscriber/minimal.
func (h *Handler) buildSurveyCrowdSharedJSON(ctx context.Context, sc iris.ICSurveyCrowd) map[string]any {
	flat := h.buildSurveyCrowdFlatJSON(ctx, sc)

	shcStatus := h.SurveyService.GetSHCStatus(ctx, sc.ID)
	shcLevel := h.SurveyService.GetSHCHonorariumLevel(ctx, sc.ID)

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
		crowdGroup = h.SurveyService.GetCrowdGroupInfo(ctx, sc.CrowdGroupID.Int64)
	}

	flat["surveyCrowdId"] = sc.ID
	flat["currentShcHonorariumLevel"] = shcLevelVal
	flat["crowdMarketHonoGroups"] = h.SurveyService.GetCrowdMarketHonoGroups(ctx, sc.CrowdID, sc.SurveyID)
	flat["surveyCustomHonoReasons"] = h.SurveyService.GetSurveyCustomHonoReasonIDs(ctx, sc.SurveyID)
	flat["crowdCurrency"] = h.SurveyService.GetCrowdCurrency(ctx, sc.CrowdID)
	flat["currentShcHonorariumOptions"] = options
	flat["excluded"] = sc.Excluded
	flat["crowdGroup"] = crowdGroup
	flat["multiProfessionCrowdMarketsHono"] = h.SurveyService.GetMultiProfessionHono(ctx, sc.ID)

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
		surveyTimeStarted = dto.NullTime(survey.FieldedOn)
		surveyStatus = survey.Status
	}

	flat["surveyCrowdId"] = sc.ID
	flat["crowd"] = h.buildCrowdFlatJSON(ctx, sc.CrowdID, &sc.SurveyID)
	flat["answerTotal"] = h.SurveyService.CountSurveyCrowdAnswers(ctx, sc.SurveyID, sc.CrowdID)
	flat["surveyTimeStarted"] = surveyTimeStarted
	flat["surveyStatus"] = surveyStatus
	flat["shcStatus"] = h.SurveyService.GetSHCStatus(ctx, sc.ID)
	flat["vendors"] = h.SurveyService.GetSurveyCrowdVendors(ctx, sc.ID)
	flat["crowdCurrency"] = h.SurveyService.GetCrowdCurrency(ctx, sc.CrowdID)
	flat["excluded"] = sc.Excluded
	flat["isInvitationPaused"] = sc.IsInvitationPaused
	flat["isReminderPaused"] = sc.IsReminderPaused
	flat["isSampleClosed"] = sc.IsSampleClosed
	flat["groupId"] = dto.NullInt64(sc.CrowdGroupID)

	return flat
}

// buildCrowdBasicJSONForSurveyCrowd loads an ICCrowd and returns basicJson.
func (h *Handler) buildCrowdBasicJSONForSurveyCrowd(ctx context.Context, crowdID int64) map[string]any {
	crowd, err := h.SurveyService.GetCrowdByID(ctx, crowdID)
	if err != nil || crowd == nil {
		return map[string]any{}
	}
	return h.buildCrowdBasicJSON(ctx, *crowd)
}

// buildCrowdFlatJSON produces crowd.flatJson(surveyId) = basicJson + contactableCount + partialCount + totalAnswerByBrand.
func (h *Handler) buildCrowdFlatJSON(ctx context.Context, crowdID int64, surveyID *int64) map[string]any {
	base := h.buildCrowdBasicJSONForSurveyCrowd(ctx, crowdID)
	size := h.SurveyService.GetCrowdSize(ctx, crowdID)

	// Brand IDs for keyed counts
	brandIDs, _ := h.SurveyService.GetCrowdBrandIDs(ctx, crowdID)
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
			_ = h.SurveyService.CountSurveyCrowdAnswersByBrand(ctx, *surveyID, crowdID, bid, &count)
			totalAnswerByBrand[fmt.Sprintf("%d", bid)] = count
		}
		base["totalAnswerByBrand"] = totalAnswerByBrand
	}

	return base
}

// buildCrowdFlatICJSON produces crowd.flatICJson = basicJson + contactableCount + partialCount (non-contactable subtracted).
func (h *Handler) buildCrowdFlatICJSON(ctx context.Context, crowdID int64) map[string]any {
	base := h.buildCrowdBasicJSONForSurveyCrowd(ctx, crowdID)
	size := h.SurveyService.GetCrowdSize(ctx, crowdID)

	brandIDs, _ := h.SurveyService.GetCrowdBrandIDs(ctx, crowdID)
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
	base["validRespondersCount"] = map[string]int64{"0": h.SurveyService.GetCrowdSize(ctx, crowdID)}
	base["followupRules"] = []map[string]any{}
	base["hasRelatedSurveys"] = false
	base["attributes"] = h.SurveyService.GetCrowdAttributes(ctx, crowdID)

	// Available counts for survey
	base["availableCount"] = h.SurveyService.GetCrowdAvailableCount(ctx, surveyID, crowdID)
	base["availableEligibleCount"] = h.SurveyService.GetCrowdAvailableEligibleCount(ctx, surveyID, crowdID, false)
	base["availableEligibleFullMatchCount"] = h.SurveyService.GetCrowdAvailableEligibleCount(ctx, surveyID, crowdID, true)

	return base
}

// CloseSurvey closes a survey.
// CloseSurvey closes a survey.
// Contract-identical with legacy InCrowdAPI: PUT /v1/survey/:id/close
// Response: flat survey object (survey.refresh.adminJson)
func (h *Handler) CloseSurvey(w http.ResponseWriter, r *http.Request) {
	surveyID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.SurveyService.IrisAvailable() && support.ResolveSource(r) == "iris" {
		if err := h.SurveyService.CloseSurvey(r.Context(), surveyID); err != nil {
			slog.Error("close survey failed", "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to close survey"})
			return
		}
		support.WriteJSON(w, http.StatusOK, map[string]any{"closed": true, "id": surveyID, "source": "iris"})
		return
	}

	// QS: update status to 'closed'
	if h.SurveyService.QsAvailable() {
		if err := h.SurveyService.Update(r.Context(), surveyID, "", "closed", nil, nil); err != nil {
			slog.Error("qs close survey failed", "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to close survey"})
			return
		}
		support.WriteJSON(w, http.StatusOK, map[string]any{"closed": true, "id": surveyID, "source": "qs"})
		return
	}
	support.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

// ToggleSurveyFavorite toggles favorite status on a survey.
// Contract-identical with legacy InCrowdAPI: PUT /v1/survey/:id/favorite
// Request: {"favorite": true/false}  (userId derived from JWT)
// Response: full survey subscriberJson object
func (h *Handler) ToggleSurveyFavorite(w http.ResponseWriter, r *http.Request) {
	surveyID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
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

	if !h.SurveyService.IrisAvailable() {
		support.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "database unavailable"})
		return
	}

	// Resolve calling user's IRIS DB ID from JWT email
	var callerUserID int64
	user := middleware.GetUser(r)
	if user != nil && user.Email != "" && h.IrisUserRepo != nil {
		u, err := h.IrisUserRepo.GetByEmail(r.Context(), user.Email)
		if err == nil && u != nil {
			callerUserID = u.ID
		}
	}
	if callerUserID == 0 {
		support.WriteJSON(w, http.StatusUnauthorized, map[string]any{"error": "could not resolve user identity"})
		return
	}

	// Fetch survey (verify it exists)
	s, err := h.SurveyService.GetSurvey(r.Context(), surveyID)
	if err != nil {
		slog.Error("get survey failed", "id", surveyID, "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
		return
	}
	if s == nil {
		support.WriteJSON(w, http.StatusNotFound, map[string]any{"error": fmt.Sprintf("survey not found: %d", surveyID)})
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
		canRead, _ := h.SurveyService.UserCanReadProject(r.Context(), callerUserID, s.ProjectID)
		if !canRead {
			support.WriteJSON(w, http.StatusForbidden, map[string]any{
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
	if err := h.SurveyService.ToggleFavorite(r.Context(), surveyID, callerUserID, req.Favorite); err != nil {
		slog.Error("toggle favorite failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to toggle favorite"})
		return
	}

	// Re-fetch survey (legacy calls survey.refresh)
	s, _ = h.SurveyService.GetSurvey(r.Context(), surveyID)

	// Build subscriberJson-equivalent response
	support.WriteJSON(w, http.StatusOK, h.buildSubscriberJSON(r, s, callerUserID))
}

// buildSubscriberJSON builds a legacy-compatible subscriberJson response for a survey.
func (h *Handler) buildSubscriberJSON(r *http.Request, s *iris.ICSurvey, callerUserID int64) map[string]any {
	ctx := r.Context()

	// Status object
	statusObj := map[string]any{"status": s.Status, "label": ""}
	if _, label, err := h.SurveyService.GetSurveyStatusLabel(ctx, s.Status); err == nil {
		statusObj["label"] = label
	}

	// Survey type
	var surveyType any
	if st, err := h.SurveyService.GetSurveyType(ctx, s.SurveyTypeID); err == nil {
		surveyType = st
	}

	// Subscription company
	subscriptionCompany := ""
	if s.SubscriptionID.Valid {
		if c, err := h.SurveyService.GetSubscriptionCompany(ctx, s.SubscriptionID.Int64); err == nil {
			subscriptionCompany = c
		}
	}

	// Project info
	projectName := ""
	projectTypeID := 1
	if pn, err := h.SurveyService.GetProjectName(ctx, s.ProjectID); err == nil {
		projectName = pn
	}
	if pt, err := h.SurveyService.GetProjectTypeID(ctx, s.ProjectID); err == nil {
		projectTypeID = pt
	}

	// Favorite check
	favorite := false
	if callerUserID > 0 {
		if f, err := h.SurveyService.IsSurveyFavoriteOf(ctx, s.ID, callerUserID); err == nil {
			favorite = f
		}
	}

	// Completions
	numCompletions := 0
	if cnt, err := h.SurveyService.CountSurveyCompletions(ctx, s.ID); err == nil {
		numCompletions = cnt
	}

	// Questions
	numQuestions := 0
	if cnt, err := h.SurveyService.CountSurveyQuestions(ctx, s.ID); err == nil {
		numQuestions = cnt
	}

	// Crowds count
	numCrowds := 0
	if cnt, err := h.SurveyService.CountSurveyCrowds(ctx, s.ID); err == nil {
		numCrowds = cnt
	}

	// Has crowd screening (at least one crowd)
	hasCrowdScreening := numCrowds > 0

	// Pricing
	pricing := map[string]any{"surveyPricingTypeId": 0, "freeScreeners": 0, "fixedRate": nil}
	if p, err := h.SurveyService.GetSurveyPricing(ctx, s.ID); err == nil {
		pricing = p
	}

	// Permissions
	permissions := map[string]any{
		"userId": callerUserID, "projectId": s.ProjectID,
		"canWrite": false, "canRead": false, "favorite": favorite,
	}
	if perms, err := h.SurveyService.GetUserProjectPermissions(ctx, callerUserID, s.ProjectID); err == nil {
		perms["favorite"] = favorite
		permissions = perms
	}

	// Qual crowd name (first crowd)
	qualCrowdName := ""
	if name, err := h.SurveyService.GetFirstSurveyCrowdName(ctx, s.ID); err == nil {
		qualCrowdName = name
	}

	// Build the full response matching legacy subscriberJson structure
	// minimalJson fields
	result := map[string]any{
		"id":                  s.ID,
		"projectId":           s.ProjectID,
		"subscriptionId":      dto.NullInt64(s.SubscriptionID),
		"subscriptionCompany": subscriptionCompany,
		"surveyTypeId":        s.SurveyTypeID,
		"projectTypeId":       projectTypeID,
		"languageId":          s.LanguageID,
		"namePublic":          s.NamePublic,
		"namePrivate":         dto.NullStr(s.NamePrivate),
		"topicName":           dto.NullStr(s.TopicName),
		"isArchived":          s.IsArchived,
		"salesforceProjectId": dto.NullStr(s.SalesforceProjectID),
		"createdOn":           s.CreatedOn.Format(time.RFC3339),
		"createdBy":           s.CreatedBy,
		"modifiedOn":          dto.NullTime(s.ModifiedOn),
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
		"lengthOfInterview": dto.NullInt64(s.LengthOfInterview),
		// pricing
		"surveyPricingTypeId": pricing["surveyPricingTypeId"],
		"freeScreeners":       pricing["freeScreeners"],
		"fixedRate":           pricing["fixedRate"],
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
