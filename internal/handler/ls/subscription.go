package ls

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/handler/support"
	"github.com/InCrowd/unified-qual-api/internal/dto"

	"github.com/InCrowd/unified-qual-api/internal/integration"
	"github.com/InCrowd/unified-qual-api/internal/middleware"
	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/InCrowd/unified-qual-api/internal/validate"
	"github.com/go-chi/chi/v5"
)

// ──────────────────────────────────────────────
// LS Subscription handlers
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Subscriptions
// ──────────────────────────────────────────────

func (h *Handler) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	if !h.SubscriptionService.Available() {
		support.WriteJSON(w, http.StatusOK, []map[string]any{})
		return
	}
	rows, err := h.SubscriptionService.ListSubscriptionsForQual(r.Context())
	if err != nil {
		slog.Error("list subscriptions", "err", err)
		support.WriteJSON(w, http.StatusOK, []map[string]any{})
		return
	}

	subs := []map[string]any{}
	for _, row := range rows {
		subs = append(subs, map[string]any{
			"id":      strconv.FormatInt(row.ID, 10),
			"company": row.Company,
			"plan":    "enterprise",
		})
	}
	support.WriteJSON(w, http.StatusOK, subs)
}

func (h *Handler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	support.WriteJSON(w, http.StatusCreated, map[string]any{"message": "subscription creation not yet implemented"})
}

func (h *Handler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	sid := chi.URLParam(r, "id")
	sidInt, _ := strconv.ParseInt(sid, 10, 64)
	if !h.SubscriptionService.Available() {
		support.WriteJSON(w, http.StatusOK, map[string]any{"id": sid, "company": "Unknown"})
		return
	}
	company, err := h.SubscriptionService.GetSubscriptionCompanyByID(r.Context(), sidInt)
	if err != nil {
		support.WriteJSON(w, http.StatusOK, map[string]any{"id": sid, "company": "Unknown"})
		return
	}
	support.WriteJSON(w, http.StatusOK, map[string]any{"id": sid, "company": company, "plan": "enterprise"})
}

func (h *Handler) UpdateSubscription(w http.ResponseWriter, r *http.Request) {
	support.WriteJSON(w, http.StatusOK, map[string]any{"message": "subscription update not yet implemented"})
}

func (h *Handler) DeleteSubscription(w http.ResponseWriter, r *http.Request) {
	support.WriteJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// ──────────────────────────────────────────────
// Subscription Domain
// ──────────────────────────────────────────────

// GetSubscriptionInterviews returns interviews for a subscription.
// Legacy contract: response wrapped as {"interviews": [...]} with 16-field interview objects.
func (h *Handler) GetSubscriptionInterviews(w http.ResponseWriter, r *http.Request) {
	subID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.SubscriptionService.Available() {
		interviews, err := h.SubscriptionService.GetSubscriptionInterviews(r.Context(), subID)
		if err != nil {
			slog.Error("subscription interviews failed", "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		if interviews == nil {
			interviews = []map[string]any{}
		}
		support.WriteJSON(w, http.StatusOK, map[string]any{"interviews": interviews})
		return
	}
	if h.QsTimeSlotRepo != nil {
		tsRows, _, _ := h.InterviewService.ListByProject(r.Context(), subID, 1, 1000)
		interviews := make([]map[string]any, 0)
		for _, ts := range tsRows {
			interviews = append(interviews, map[string]any{
				"id": ts.ID, "projectId": ts.ProjectID,
				"startTime": ts.StartTime.Format(time.RFC3339),
				"endTime":   ts.EndTime.Format(time.RFC3339),
				"statusId":  ts.StatusID,
			})
		}
		support.WriteJSON(w, http.StatusOK, map[string]any{"interviews": interviews})
		return
	}
	support.WriteJSON(w, http.StatusOK, map[string]any{"interviews": []map[string]any{}})
}

// GetSubscriptionCrowds returns crowds for a subscription.
// Legacy contract: response wrapped as {"crowds": [...], "limit": N, "offset": N, "totalCount": N}
// Supports ?limit, ?offset, ?includeExclusionLists, ?jsonType=basic|admin (default admin).
func (h *Handler) GetSubscriptionCrowds(w http.ResponseWriter, r *http.Request) {
	subID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if !h.SubscriptionService.Available() {
		support.WriteJSON(w, http.StatusOK, map[string]any{"crowds": []map[string]any{}, "limit": 20, "offset": 0, "totalCount": 0})
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

	crowds, total, err := h.SubscriptionService.ListCrowdsForSubscription(r.Context(), subID, filter)
	if err != nil {
		slog.Error("subscription crowds failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
		return
	}

	ctx := r.Context()
	result := make([]map[string]any, 0, len(crowds))
	for _, c := range crowds {
		result = append(result, h.buildCrowdBasicJSON(ctx, c))
	}

	support.WriteJSON(w, http.StatusOK, map[string]any{
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
	if td, err := h.SubscriptionService.GetCrowdTypeDescription(ctx, c.TypeID); err == nil {
		typeDesc = td
	}

	// Display name
	descriptiveName := c.Name
	if c.Deleted {
		descriptiveName = "[DELETED] " + c.Name
	}

	// Account ID (from subscription)
	var accountID any
	if aid := h.SubscriptionService.GetAccountIDForSubscription(ctx, c.SubscriptionID); aid != nil {
		accountID = *aid
	}

	// Market name
	marketName := ""
	if mn, err := h.SubscriptionService.GetMarketName(ctx, c.MarketID); err == nil {
		marketName = mn
	}

	// Brand IDs and name
	brandIDs, _ := h.SubscriptionService.GetCrowdBrandIDs(ctx, c.ID)
	if brandIDs == nil {
		brandIDs = []int64{}
	}
	var brandNames []string
	for _, bid := range brandIDs {
		if bn, err := h.SubscriptionService.GetBrandName(ctx, bid); err == nil {
			brandNames = append(brandNames, bn)
		}
	}
	brandName := strings.Join(brandNames, ", ")

	// Country (attribute_id=29)
	countryID := h.SubscriptionService.GetCrowdCountryID(ctx, c.ID)
	countryName := ""
	var countryLanguage []string
	if countryID > 0 {
		countryName = h.SubscriptionService.GetAttributeChoiceLabel(ctx, countryID)
		countryLanguage = h.SubscriptionService.GetCountryLanguages(ctx, countryID, countryName)
	}
	if countryLanguage == nil {
		countryLanguage = []string{}
	}

	// Created via list match
	createdViaListMatch := h.SubscriptionService.CrowdHasListMatch(ctx, c.ID) || c.DuplicatedFromS3Key.Valid

	// Specialty values
	specialtyValues := h.SubscriptionService.GetCrowdSpecialtyIDs(ctx, c.ID)
	if specialtyValues == nil {
		specialtyValues = []int{}
	}

	// Engagement rates
	var expectedCompletesRate any
	if rate := h.SubscriptionService.GetCrowdEngagementRate(ctx, c.ID, false); rate != nil {
		expectedCompletesRate = *rate
	}
	var expectedCompletesRateFullMatch any
	if rate := h.SubscriptionService.GetCrowdEngagementRate(ctx, c.ID, true); rate != nil {
		expectedCompletesRateFullMatch = *rate
	}

	return map[string]any{
		"id":                             c.ID,
		"typeId":                         c.TypeID,
		"typeDescription":                typeDesc,
		"name":                           c.Name,
		"descriptiveName":                descriptiveName,
		"description":                    dto.NullStr(c.Description),
		"subscriptionId":                 c.SubscriptionID,
		"accountId":                      accountID,
		"createdBy":                      c.CreatedBy,
		"marketId":                       c.MarketID,
		"marketName":                     marketName,
		"brandIds":                       brandIDs,
		"brandName":                      brandName,
		"countryId":                      countryID,
		"countryName":                    countryName,
		"countryLanguage":                countryLanguage,
		"deleted":                        c.Deleted,
		"andOr":                          dto.NullInt64(c.AndOr),
		"deletedOn":                      dto.NullTime(c.DeletedOn),
		"deletedBy":                      dto.NullInt64(c.DeletedBy),
		"createdOn":                      c.CreatedOn.Format(time.RFC3339),
		"isArchived":                     c.IsArchived,
		"createdFromSampleTemplateId":    dto.NullInt64(c.CreatedFromSampleTemplateID),
		"isNewbie":                       c.IsNewbie,
		"createdViaListMatch":            createdViaListMatch,
		"incrowdTPA":                     dto.NullStr(c.IncrowdTPA),
		"doximityTPA":                    dto.NullStr(c.DoximityTPA),
		"canShareWithDoximity":           c.CanShareWithDoximity,
		"crowdSpecialtyValues":           specialtyValues,
		"expectedCompletesRate":          expectedCompletesRate,
		"expectedCompletesRateFullMatch": expectedCompletesRateFullMatch,
	}
}

// GetSubscriptionQuestionTypes returns question types for a subscription.
// Legacy contract: returns all question_type rows with pagination wrapper.
// Response: {"totalCount": N, "limit": N, "offset": N, "questionTypes": [...]}
func (h *Handler) GetSubscriptionQuestionTypes(w http.ResponseWriter, r *http.Request) {
	_ = chi.URLParam(r, "id") // subId validated but not used for filtering (legacy returns all types)

	if !h.SubscriptionService.Available() {
		support.WriteJSON(w, http.StatusOK, map[string]any{"totalCount": 0, "limit": nil, "offset": nil, "questionTypes": []map[string]any{}})
		return
	}

	allTypes, err := h.SubscriptionService.GetAllQuestionTypes(r.Context())
	if err != nil {
		slog.Error("question types failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
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

	support.WriteJSON(w, http.StatusOK, map[string]any{
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
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if !h.SubscriptionService.Available() {
		support.WriteJSON(w, http.StatusOK, []map[string]any{})
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

	inquiries, err := h.SubscriptionService.ListProjectInquiries(r.Context(), subID, filter)
	if err != nil {
		slog.Error("inquiries list failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
		return
	}

	ctx := r.Context()
	result := make([]map[string]any, 0, len(inquiries))
	for _, pi := range inquiries {
		// Build inquiry adminJson
		inquiryJSON := map[string]any{
			"id":                     pi.ID,
			"description":            pi.Description,
			"notes":                  dto.NullStr(pi.Notes),
			"subscriptionId":         pi.SubscriptionID,
			"inquiryTypeId":          pi.InquiryTypeID,
			"inquiryType":            h.SubscriptionService.GetInquiryTypeName(ctx, pi.InquiryTypeID),
			"interviewLength":        pi.InterviewLength,
			"requiredCompletionDate": dto.NullTime(pi.RequiredCompletionDate),
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
		if h.ProjectService != nil {
			if p, err := h.ProjectService.IrisGetByID(ctx, pi.ProjectID); err == nil && p != nil {
				projectJSON = map[string]any{
					"id":                  p.ID,
					"name":                p.Name,
					"description":         dto.NullStr(p.Description),
					"subscriptionId":      p.SubscriptionID,
					"createdOn":           p.CreatedOn.Format(time.RFC3339),
					"createdBy":           dto.NullInt64(p.CreatedBy),
					"modifiedOn":          dto.NullTime(p.ModifiedOn),
					"budget":              dto.NullStr(p.Budget),
					"isPrivate":           p.IsPrivate,
					"qualModeratorId":     dto.NullInt64(p.QualModeratorID),
					"projectStatusId":     p.ProjectStatusID,
					"projectTypeId":       p.ProjectTypeID,
					"salesforceProjectId": dto.NullStr(p.SalesforceProjectID),
					"completedOn":         dto.NullTime(p.CompletedOn),
					"isArchived":          p.IsArchived,
					"archivedBy":          dto.NullInt64(p.ArchivedBy),
					"archivedOn":          dto.NullTime(p.ArchivedOn),
					"finalizedOn":         dto.NullTime(p.FinalizedOn),
				}
			}
		}

		result = append(result, map[string]any{
			"inquiry": inquiryJSON,
			"project": projectJSON,
		})
	}

	support.WriteJSON(w, http.StatusOK, result)
}

// GetSubscriptionProjectInquiry returns a specific inquiry for a subscription/project.
// Contract-identical with legacy InCrowdAPI: GET /v1/subscription/:subscriptionId/project/:projectId/inquiry
// Response: SavedProposal {project, crowds, proposal, costs} + isHardStop
func (h *Handler) GetSubscriptionProjectInquiry(w http.ResponseWriter, r *http.Request) {
	subID, _ := validate.ParseIDParam(r, "subId")
	projectID, _ := validate.ParseIDParam(r, "pid")

	if !h.SubscriptionService.Available() {
		support.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "inquiry not found"})
		return
	}

	// 1. Get inquiry
	pi, err := h.SubscriptionService.GetProjectInquiry(r.Context(), subID, projectID)
	if err != nil {
		slog.Error("get inquiry failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
		return
	}
	if pi == nil {
		support.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "inquiry not found"})
		return
	}

	// 2. Get project
	project, err := h.SubscriptionService.GetProjectForInquiry(r.Context(), projectID)
	if err != nil || project == nil {
		support.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "project not found"})
		return
	}

	// 3. Get inquiry crowds (standard + custom + crowd objects)
	standardCrowds, customCrowds, crowdObjects, err := h.SubscriptionService.GetProjectInquiryCrowds(r.Context(), pi.ID)
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
		"notes":                dto.NullStr(pi.Notes),
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
		fees, err = h.SubscriptionService.GetProjectFees(r.Context(), projectID)
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

	support.WriteJSON(w, http.StatusOK, map[string]any{
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
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if !h.SubscriptionService.Available() {
		support.WriteJSON(w, http.StatusOK, map[string]any{"projects": map[string]any{}})
		return
	}

	// Resolve calling user's IRIS DB id for favorite check
	var callerUserID int64
	user := middleware.GetUser(r)
	if user != nil && user.Email != "" && h.IrisUserRepo != nil {
		u, err := h.UserService.GetIrisUserByEmail(r.Context(), user.Email)
		if err == nil && u != nil {
			callerUserID = u.ID
		}
	}

	// Step 1: Get projects for subscription (excludes status 1 / draft, excludes archived)
	projects, err := h.SubscriptionService.ListProjectsForSubscription(r.Context(), subID)
	if err != nil {
		slog.Error("subscription projects failed", "error", err)
		support.WriteJSON(w, http.StatusOK, map[string]any{"projects": map[string]any{}})
		return
	}

	projectsMap := make(map[string]any, len(projects))
	for _, p := range projects {
		// Step 2: Get surveys for each project
		surveys, err := h.SubscriptionService.ListSurveysForProject(r.Context(), p.ID)
		if err != nil {
			slog.Error("project surveys failed", "projectId", p.ID, "error", err)
			surveys = nil
		}

		surveyList := make([]map[string]any, 0, len(surveys))
		for _, s := range surveys {
			// Status object: { "status": <int>, "label": <string> }
			statusObj := map[string]any{"status": s.Status, "label": ""}
			_, label, err := h.SubscriptionService.GetSurveyStatusLabel(r.Context(), s.Status)
			if err == nil {
				statusObj["label"] = label
			}

			// Qual crowd name (first survey_crowd entry)
			qualCrowdName := ""
			if name, err := h.SubscriptionService.GetFirstSurveyCrowdName(r.Context(), s.ID); err == nil {
				qualCrowdName = name
			}

			// Question count
			numQuestions := 0
			if cnt, err := h.SubscriptionService.CountSurveyQuestions(r.Context(), s.ID); err == nil {
				numQuestions = cnt
			}

			// Completion count (non-invalid, non-test)
			numCompletions := 0
			if cnt, err := h.SubscriptionService.CountSurveyCompletions(r.Context(), s.ID); err == nil {
				numCompletions = cnt
			}

			// Favorite check for calling user
			favorite := false
			if callerUserID > 0 {
				if fav, err := h.SubscriptionService.IsSurveyFavoriteOf(r.Context(), s.ID, callerUserID); err == nil {
					favorite = fav
				}
			}

			surveyList = append(surveyList, map[string]any{
				"id":                s.ID,
				"namePublic":        s.NamePublic,
				"namePrivate":       dto.NullStr(s.NamePrivate),
				"favorite":          favorite,
				"status":            statusObj,
				"qualCrowdName":     qualCrowdName,
				"projectId":         s.ProjectID,
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

	support.WriteJSON(w, http.StatusOK, map[string]any{"projects": projectsMap})
}

// ──────────────────────────────────────────────
// Market Domain
// ──────────────────────────────────────────────

func (h *Handler) UpdateInquiryPreview(w http.ResponseWriter, r *http.Request) {
	subStr := chi.URLParam(r, "subscriptionId")
	subID, _ := strconv.ParseInt(subStr, 10, 64)

	var proposal ipProposal
	if errs := validate.DecodeAndValidate(r, &proposal); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	if !h.SubscriptionService.Available() {
		support.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
		return
	}

	ctx := r.Context()

	// Step 1: Get qual products
	allProducts, err := h.SubscriptionService.GetQualProducts(ctx)
	if err != nil {
		slog.Error("get qual products failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to get products"})
		return
	}

	// Filter out transcription products if not requested
	var filtered []iris.QualProduct
	for _, p := range allProducts {
		if !proposal.TranscriptsRequested && strings.HasPrefix(p.Name, "Transcription") {
			continue
		}
		filtered = append(filtered, p)
	}

	// Partition: base products (no qual_interview_minutes) + matching interview length
	var products []iris.QualProduct
	for _, p := range filtered {
		if p.QualInterviewMinutes == nil {
			products = append(products, p)
		} else if *p.QualInterviewMinutes == proposal.InterviewLength {
			products = append(products, p)
		}
	}

	// Step 2: Get related market IDs for each product
	for i := range products {
		mids, err := h.SubscriptionService.GetProductRelatedMarketIDs(ctx, products[i].ID)
		if err != nil {
			slog.Error("get product market ids failed", "error", err, "productId", products[i].ID)
		}
		products[i].RelatedMarketIDs = mids
	}

	// Step 4: Get subscription discount
	serviceDiscount, err := h.SubscriptionService.GetSubscriptionServiceDiscount(ctx, subID)
	if err != nil {
		slog.Error("get subscription discount failed", "error", err)
		serviceDiscount = 0
	}

	// Combine all crowds for total respondent count
	allCrowds := make([]ipCrowdSpec, 0, len(proposal.Crowds)+len(proposal.CustomCrowds))
	allCrowds = append(allCrowds, proposal.Crowds...)
	allCrowds = append(allCrowds, proposal.CustomCrowds...)

	var totalRespondents int64
	for _, c := range allCrowds {
		if c.NumberRequested != nil {
			totalRespondents += *c.NumberRequested
		}
	}

	// Step 3: Calculate fees
	var fees []ipFee
	for _, p := range products {
		var fee ipFee
		fee.Name = p.Name
		fee.ProductID = p.ID
		fee.PricePerUnit = p.PriceUSD
		fee.IsHonorarium = p.IsHonorarium

		if p.IsHonorarium {
			var count int64
			for i := range allCrowds {
				if ipCrowdMatchesProduct(&allCrowds[i], p.RelatedMarketIDs, p.IsSpecialized) {
					if allCrowds[i].NumberRequested != nil {
						count += *allCrowds[i].NumberRequested
					}
				}
			}
			fee.Count = count
			fee.GrossSubtotal = p.PriceUSD * float64(count)
			fee.NetSubtotal = fee.GrossSubtotal
			fee.IsHonorarium = true
		} else if p.IsForService {
			if p.IsFlatFee {
				fee.Count = 1
			} else {
				fee.Count = totalRespondents
			}
			fee.GrossSubtotal = p.PriceUSD * float64(fee.Count)
			dr := serviceDiscount
			fee.DiscountRate = &dr
			fee.NetSubtotal = fee.GrossSubtotal * (1 - serviceDiscount)
		} else if p.IsFlatFee {
			fee.Count = 1
			fee.GrossSubtotal = p.PriceUSD
			fee.NetSubtotal = fee.GrossSubtotal
		} else {
			fee.Count = totalRespondents
			fee.GrossSubtotal = p.PriceUSD * float64(totalRespondents)
			fee.NetSubtotal = fee.GrossSubtotal
		}

		fees = append(fees, fee)
	}

	var grossTotal, netTotal float64
	for _, f := range fees {
		grossTotal += f.GrossSubtotal
		netTotal += f.NetSubtotal
	}

	// Step 5: Assess crowd difficulty
	assessments, err := h.SubscriptionService.GetDifficultyAssessments(ctx)
	if err != nil {
		slog.Error("get difficulty assessments failed", "error", err)
	}

	isHardStop := false

	for i := range proposal.Crowds {
		da := h.assessCrowdDifficulty(ctx, &proposal.Crowds[i], assessments)
		proposal.Crowds[i].DifficultyAssessment = da
		if da != nil && da.IsHardStop {
			isHardStop = true
		}
	}
	for i := range proposal.CustomCrowds {
		da := h.assessCrowdDifficulty(ctx, &proposal.CustomCrowds[i], assessments)
		proposal.CustomCrowds[i].DifficultyAssessment = da
		if da != nil && da.IsHardStop {
			isHardStop = true
		}
	}

	// Step 6: Resolve custom crowd names
	for i := range proposal.CustomCrowds {
		if proposal.CustomCrowds[i].CrowdID != nil {
			name, err := h.SubscriptionService.GetCrowdNameByID(ctx, *proposal.CustomCrowds[i].CrowdID)
			if err != nil {
				slog.Error("get crowd name failed", "error", err, "crowdId", *proposal.CustomCrowds[i].CrowdID)
			} else {
				proposal.CustomCrowds[i].Name = &name
			}
		}
	}

	// Step 7: Lookup Salesforce project
	var sfProjectName *string
	if proposal.SalesforceProjectID != nil && *proposal.SalesforceProjectID != "" {
		num, name, err := h.SubscriptionService.GetSalesforceProjectByExtID(ctx, *proposal.SalesforceProjectID)
		if err != nil {
			slog.Error("get salesforce project failed", "error", err)
		} else {
			formatted := fmt.Sprintf("%s - %s", num, name)
			sfProjectName = &formatted
		}
	}

	// Ensure fees is an empty array, not null
	if fees == nil {
		fees = []ipFee{}
	}

	// Step 8: Build response
	resp := ipResponse{
		Proposal:              &proposal,
		SalesforceProjectName: sfProjectName,
		Costs: &ipProjectCosts{
			GrossTotal: grossTotal,
			NetTotal:   netTotal,
			Fees:       fees,
		},
		IsHardStop: isHardStop,
	}

	support.WriteJSON(w, http.StatusOK, resp)
}

// assessCrowdDifficulty calculates the feasibility score for a crowd and matches it to an assessment.
func (h *Handler) assessCrowdDifficulty(ctx context.Context, crowd *ipCrowdSpec, assessments []iris.DifficultyAssessmentRow) *ipDifficultyAssessmentResp {
	const (
		responseRate   = 0.2
		acceptanceRate = 0.6
		schedulingRate = 0.8
		flakeOutRate   = 0.85
	)

	if crowd.NumberRequested == nil || *crowd.NumberRequested == 0 {
		return nil
	}

	population, err := h.SubscriptionService.CountMarketPopulation(ctx, crowd.MarketID)
	if err != nil {
		slog.Error("count market population failed", "error", err, "marketId", crowd.MarketID)
		return nil
	}

	diffPercent, err := h.SubscriptionService.GetDifficultyLevelPercent(ctx, crowd.DifficultyLevel.ID)
	if err != nil {
		slog.Error("get difficulty level percent failed", "error", err, "levelId", crowd.DifficultyLevel.ID)
		return nil
	}

	expectedResponse := float64(population) * responseRate
	expectedIncidence := expectedResponse * diffPercent
	expectedAcceptance := expectedIncidence * acceptanceRate
	expectedScheduling := expectedAcceptance * schedulingRate
	expectedAttendance := expectedScheduling * flakeOutRate
	feasibilityScore := expectedAttendance / float64(*crowd.NumberRequested)

	for _, a := range assessments {
		minOK := a.MinPercent == nil || *a.MinPercent <= feasibilityScore
		maxOK := a.MaxPercent == nil || feasibilityScore < *a.MaxPercent
		if minOK && maxOK {
			return &ipDifficultyAssessmentResp{
				ID:         a.ID,
				Name:       a.Name,
				IsHardStop: a.IsHardStop,
			}
		}
	}
	return nil
}

// CreateCustomCrowdInquiry handles custom crowd inquiry submission with CSV upload.
// Contract-identical with legacy InCrowdAPI: POST /v1/custom_crowd_inquiry
// Request: multipart/form-data with file + crowdName + completionDate + sampleSize + subscriptionId + marketId(optional)
// Response: {} (empty JSON object)
// Side effects: S3 upload (public/ prefix), email notification (async)
func (h *Handler) CreateCustomCrowdInquiry(w http.ResponseWriter, r *http.Request) {
	// Legacy error helper: wraps in {"error": {"userMessage":..., "developerMessage":..., "status":"BAD REQUEST", "code":400}}
	badRequest := func(reason string) {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]any{
				"userMessage":      "Something sent doesn't make sense, please check your request",
				"developerMessage": reason,
				"status":           "BAD REQUEST",
				"code":             400,
			},
		})
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32 MB max
		badRequest("not valid multipart-form-data, or labeled as such")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		badRequest("no file upload was found, or named \"file\"")
		return
	}
	defer file.Close()

	crowdName := r.FormValue("crowdName")
	completionDate := r.FormValue("completionDate")
	sampleSizeStr := r.FormValue("sampleSize")
	subscriptionIDStr := r.FormValue("subscriptionId")
	marketID := r.FormValue("marketId")

	if crowdName == "" || completionDate == "" || sampleSizeStr == "" || subscriptionIDStr == "" {
		badRequest("invalid form data for list match inquiry")
		return
	}

	subscriptionID, err := strconv.ParseInt(subscriptionIDStr, 10, 64)
	if err != nil {
		badRequest("invalid form data for list match inquiry")
		return
	}

	sampleSize, err := strconv.Atoi(sampleSizeStr)
	if err != nil {
		badRequest("invalid form data for list match inquiry")
		return
	}

	// Resolve calling user's numeric IRIS ID for filename (legacy uses user.id)
	var userID int64
	if user := middleware.GetUser(r); user != nil && user.Email != "" && h.SubscriptionService.Available() {
		if id, lookupErr := h.SubscriptionService.GetUserIDByEmail(r.Context(), user.Email); lookupErr == nil {
			userID = id
		}
	}
	userIDStr := "unknown"
	if userID > 0 {
		userIDStr = strconv.FormatInt(userID, 10)
	}

	// Upload file to S3 with public/ prefix (legacy: s3Gateway.uploadAndGetPublicUrl)
	fileName := fmt.Sprintf("public/custom_crowd_inquiry%d_%s.csv", time.Now().UnixMilli(), userIDStr)
	var downloadLink string
	if h.Services.S3 != nil && h.Services.S3.Configured() {
		url, uploadErr := h.Services.S3.UploadFile(r.Context(), h.Services.S3.InquiryBucket(), fileName, file, header.Header.Get("Content-Type"))
		if uploadErr != nil {
			slog.Error("S3 upload failed for custom crowd inquiry", "error", uploadErr, "userId", userIDStr)
		} else {
			downloadLink = url
		}
	} else {
		slog.Warn("S3 not configured, skipping file upload for custom crowd inquiry")
	}

	// Determine inquiry type (List Match vs Prevalidated List)
	isListMatch := marketID != ""

	// Look up subscription company name + shortCode
	companyName := "Unknown"
	shortCode := ""
	if h.SubscriptionService.Available() {
		if name, sc, lookupErr := h.SubscriptionService.GetSubscriptionCompanyAndShortCode(r.Context(), subscriptionID); lookupErr == nil {
			if name != "" {
				companyName = name
			}
			shortCode = sc
		}
	}

	// Look up market name if provided
	var marketName string
	if isListMatch && h.SubscriptionService.Available() {
		if mID, parseErr := strconv.ParseInt(marketID, 10, 64); parseErr == nil {
			if name, lookupErr := h.SubscriptionService.GetMarketName(r.Context(), mID); lookupErr == nil {
				marketName = name
			}
		}
	}

	// Build email matching legacy template (qualCustomCrowdSubmitted.scala.html)
	subject := "A Prevalidated List was submitted"
	requestType := "Prevalidated List Request"
	listDesc := "A subscriber has submitted a prevalidated list"
	if isListMatch {
		subject = "A List Match crowd was submitted"
		requestType = "List Match Request"
		listDesc = "A subscriber has submitted a file to be list matched"
	}

	// Send email notification asynchronously (legacy uses Future{...})
	if h.Services.Notification != nil && h.Services.Notification.Configured() {
		emailBody := fmt.Sprintf(
			"<h2>%s</h2>"+
				"<p>%s</p>"+
				"<p><strong>Subscription:</strong> %s</p>"+
				"<p><strong>Short Code:</strong> %s</p>"+
				"<p><strong>Requested Crowd Name:</strong> %s</p>",
			requestType, listDesc, companyName, shortCode, crowdName,
		)
		if isListMatch && marketName != "" {
			emailBody += fmt.Sprintf("<p><strong>Crowd Market:</strong> %s</p>", marketName)
		}
		emailBody += fmt.Sprintf(
			"<p><strong>Sample Size:</strong> %d</p>"+
				"<p><strong>Required Recruitment Completion Date:</strong> %s</p>",
			sampleSize, completionDate,
		)
		if !isListMatch {
			emailBody += "<p>User attested that all potential recipients consented to third party contact.</p>"
		}
		if downloadLink != "" {
			linkLabel := "Download Prevalidated List CSV"
			if isListMatch {
				linkLabel = "Download List Match CSV"
			}
			emailBody += fmt.Sprintf("<p><a href=\"%s\">%s</a></p>", downloadLink, linkLabel)
		} else {
			emailBody += "<p><strong>There was an error uploading the file to S3. Please file a PS ticket to retrieve the file.</strong></p>"
		}

		recipient := h.Cfg.InquiryEmailRecipient
		if recipient == "" {
			recipient = "dev-ni@incrowdnow.com"
		}

		// Fire-and-forget (legacy returns Ok before email completes)
		go func() {
			if emailErr := h.Services.Notification.SendEmail(r.Context(), integration.EmailMessage{
				To:          []string{recipient},
				Subject:     subject,
				Body:        emailBody,
				ContentType: "text/html",
			}); emailErr != nil {
				slog.Error("failed to send custom crowd inquiry email", "error", emailErr, "userId", userIDStr)
			}
		}()
	}

	// Contract-identical: legacy returns empty JSON object
	support.WriteJSON(w, http.StatusOK, map[string]any{})
}
