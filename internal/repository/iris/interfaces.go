package iris

import (
	"context"
	"time"
)

// ProjectRepository defines the contract for ProjectRepo.
type ProjectRepository interface {
	List(ctx context.Context, page, pageSize int, statusID *int, search string) ([]ProjectListRow, int, error)
	GetByID(ctx context.Context, id int64) (*Project, error)
	Create(ctx context.Context, p *Project) (int64, error)
	Update(ctx context.Context, id int64, fields map[string]any) error
}

// SurveyRepository defines the contract for SurveyRepo.
type SurveyRepository interface {
	ListSurveysForProject(ctx context.Context, projectID int64) ([]ICSurvey, error)
	ListSurveysForSubscription(ctx context.Context, subscriptionID int64) ([]ICSurvey, error)
	GetSurvey(ctx context.Context, id int64) (*ICSurvey, error)
	CloseSurvey(ctx context.Context, id int64) error
	ToggleFavorite(ctx context.Context, surveyID, userID int64, favorite bool) error
	ValidateSurvey(ctx context.Context, id int64) ([]string, error)
	ValidateSurveyWarnings(ctx context.Context, surveyID int64) []string
	GetSurveyCrowds(ctx context.Context, surveyID int64) ([]ICSurveyCrowd, error)
	CountSurveyCrowdAnswers(ctx context.Context, surveyID, crowdID int64) int64
	GetSHCStatus(ctx context.Context, surveyCrowdID int64) string
	GetSurveyCrowdVendors(ctx context.Context, surveyCrowdID int64) []string
	GetCrowdGroupInfo(ctx context.Context, groupID int64) map[string]any
	GetCrowdCurrency(ctx context.Context, crowdID int64) string
	GetSHCHonorariumLevel(ctx context.Context, surveyCrowdID int64) *string
	GetCrowdMarketHonoGroups(ctx context.Context, crowdID, surveyID int64) []map[string]any
	GetSurveyCustomHonoReasonIDs(ctx context.Context, surveyID int64) []int64
	GetCrowdAttributesRemoved(ctx context.Context, surveyCrowdID int64) []map[string]any
	GetMultiProfessionHono(ctx context.Context, surveyCrowdID int64) []map[string]any
	GetCrowdSize(ctx context.Context, crowdID int64) int64
	GetCrowdAvailableCount(ctx context.Context, surveyID, crowdID int64) int64
	GetCrowdAvailableEligibleCount(ctx context.Context, surveyID, crowdID int64, fullMatch bool) int64
	ListCrowdsForSubscription(ctx context.Context, subscriptionID int64, f *CrowdFilter) ([]ICCrowd, int, error)
	GetCrowdTypeDescription(ctx context.Context, typeID int) (string, error)
	GetMarketName(ctx context.Context, marketID int64) (string, error)
	GetCrowdBrandIDs(ctx context.Context, crowdID int64) ([]int64, error)
	GetBrandName(ctx context.Context, brandID int64) (string, error)
	GetAccountIDForSubscription(ctx context.Context, subscriptionID int64) *int64
	GetCrowdCountryID(ctx context.Context, crowdID int64) int64
	GetAttributeChoiceLabel(ctx context.Context, choiceID int64) string
	GetCountryLanguages(ctx context.Context, countryID int64, countryName string) []string
	CrowdHasListMatch(ctx context.Context, crowdID int64) bool
	GetCrowdSpecialtyIDs(ctx context.Context, crowdID int64) []int
	GetCrowdEngagementRate(ctx context.Context, crowdID int64, fullMatch bool) *float64
	GetCrowdByID(ctx context.Context, crowdID int64) (*ICCrowd, error)
	GetCrowdAttributes(ctx context.Context, crowdID int64) []map[string]any
	CountSurveyCrowdAnswersByBrand(ctx context.Context, surveyID, crowdID, brandID int64, count *int64) error
	ListMarkets(ctx context.Context, f *MarketFilter) ([]ICMarket, int, error)
	GetMarketNameTranslation(ctx context.Context, marketID int64, lang string) string
	GetMarketRollupTranslation(ctx context.Context, marketID int64, lang string) string
	GetSubscriptionIDForAccount(ctx context.Context, accountID int64) *int64
	ListMarketsWithNPI(ctx context.Context) ([]ICMarket, error)
	GetCrowdableAttributes(ctx context.Context, marketID int64) ([]map[string]any, error)
	ListObserversForProject(ctx context.Context, projectID int64) ([]ICObserver, error)
	ListObserversForTimeSlot(ctx context.Context, timeSlotID int64) ([]ICObserver, error)
	PutObserversForTimeSlot(ctx context.Context, projectID, timeSlotID int64, toAdd, toDelete []string) error
	ListMediaForProject(ctx context.Context, projectID int64) ([]ICInterviewMedia, error)
	GetMediaByID(ctx context.Context, projectID, mediaID int64) (*ICInterviewMedia, error)
	DeleteMedia(ctx context.Context, projectID, mediaID int64) error
	ListModeratorAvailability(ctx context.Context, moderatorID, subscriptionID int64) ([]ICModeratorAvailability, error)
	CreateModeratorAvailability(ctx context.Context, moderatorID, subscriptionID int64, startTime, endTime time.Time) (int64, error)
	UpdateModeratorAvailability(ctx context.Context, id int64, startTime, endTime time.Time) error
	DeleteModeratorAvailability(ctx context.Context, id int64) error
	GetModeratorsForTimeSlot(ctx context.Context, timeSlotID int64) ([]map[string]any, error)
	AssignModeratorToTimeSlot(ctx context.Context, timeSlotID, moderatorID int64, isHost bool) (int64, error)
	RemoveModeratorFromTimeSlot(ctx context.Context, timeSlotID, moderatorID int64) error
	ListUserProjects(ctx context.Context, projectID int64) ([]map[string]any, error)
	ListSalesforceProjects(ctx context.Context, f *SalesforceProjectFilter) ([]ICSalesforceProject, error)
	GetMonoProjectID(ctx context.Context, salesforceProjectID string) *int64
	ListProjectInquiries(ctx context.Context, subscriptionID int64, f *InquiryFilter) ([]ICProjectInquiry, error)
	GetInquiryTypeName(ctx context.Context, typeID int64) string
	GetProjectInquiry(ctx context.Context, subscriptionID, projectID int64) (*ICProjectInquiry, error)
	GetProjectDashboardInfo(ctx context.Context, projectID int64) (map[string]any, error)
	GetSchedulerModerators(ctx context.Context, projectID int64) ([]map[string]any, error)
	GetProjectAvailability(ctx context.Context, projectID int64) ([]ICModeratorAvailability, error)
	GetSubscriptionInterviews(ctx context.Context, subscriptionID int64) ([]map[string]any, error)
	GetAllQuestionTypes(ctx context.Context) ([]map[string]any, error)
	GetQualRescheduleBody(ctx context.Context, projectID int64) (string, error)
	GetNoShowCheck(ctx context.Context) (map[string]any, error)
	MarkNoShow(ctx context.Context, projectID, timeSlotID int64) error
	GetPossibleModeratorsForTimeSlot(ctx context.Context, timeSlotID int64) ([]map[string]any, error)
	UpdateInquiryPreview(ctx context.Context, subscriptionID, projectID int64, fields map[string]any) error
	CreateCustomCrowdInquiry(ctx context.Context, subscriptionID int64, description string, interviewLength int, inquiryTypeID int) (int64, error)
	ResetProjectModerators(ctx context.Context, projectID int64) (int64, error)
	GetEmailTemplateForProject(ctx context.Context, projectID int64) (map[string]any, error)
	ExportProjectData(ctx context.Context, projectID int64) ([]map[string]any, error)
	GetUnavailableModerators(ctx context.Context, projectID int64) ([]map[string]any, error)
	GetAvailableModeratorsCount(ctx context.Context, projectID int64) (int, error)
	ListProjectsForSubscription(ctx context.Context, subscriptionID int64) ([]ICProjectBasic, error)
	GetSurveyStatusLabel(ctx context.Context, statusCode int) (int, string, error)
	GetFirstSurveyCrowdName(ctx context.Context, surveyID int64) (string, error)
	CountSurveyQuestions(ctx context.Context, surveyID int64) (int, error)
	CountSurveyCompletions(ctx context.Context, surveyID int64) (int, error)
	IsSurveyFavoriteOf(ctx context.Context, surveyID, userID int64) (bool, error)
	GetSubscriptionCompanyAndShortCode(ctx context.Context, subscriptionID int64) (string, string, error)
	GetSubscriptionCompany(ctx context.Context, subscriptionID int64) (string, error)
	GetUserIDByEmail(ctx context.Context, email string) (int64, error)
	GetProjectName(ctx context.Context, projectID int64) (string, error)
	GetProjectTypeID(ctx context.Context, projectID int64) (int, error)
	GetSurveyType(ctx context.Context, typeID int) (map[string]any, error)
	GetSurveyPricing(ctx context.Context, surveyID int64) (map[string]any, error)
	GetUserProjectPermissions(ctx context.Context, userID, projectID int64) (map[string]any, error)
	UserCanReadProject(ctx context.Context, userID, projectID int64) (bool, error)
	CountSurveyCrowds(ctx context.Context, surveyID int64) (int, error)
	CountScreenOutCompletions(ctx context.Context, surveyID int64) (int, error)
	GetQualProducts(ctx context.Context) ([]QualProduct, error)
	GetProductRelatedMarketIDs(ctx context.Context, productID int64) ([]int64, error)
	GetSubscriptionServiceDiscount(ctx context.Context, subscriptionID int64) (float64, error)
	CountMarketPopulation(ctx context.Context, marketID int64) (int64, error)
	GetDifficultyLevelPercent(ctx context.Context, levelID int64) (float64, error)
	GetDifficultyAssessments(ctx context.Context) ([]DifficultyAssessmentRow, error)
	GetCrowdNameByID(ctx context.Context, crowdID int64) (string, error)
	GetSalesforceProjectByExtID(ctx context.Context, sfProjectID string) (string, string, error)
	GetProjectIDByConferenceHash(ctx context.Context, hash string) (int64, error)
	GetTimeSlotAdminJSON(ctx context.Context, timeSlotID int64) (map[string]any, error)
	GetProjectForInquiry(ctx context.Context, projectID int64) (map[string]any, error)
	GetProjectFees(ctx context.Context, projectID int64) ([]map[string]any, error)
	GetProjectInquiryCrowds(ctx context.Context, inquiryID int64) (
		standardCrowds []map[string]any, customCrowds []map[string]any,
		crowdObjects []map[string]any, err error)
	GetCrowdAttributeChoiceIDs(ctx context.Context, crowdAttributeID int64) ([]int64, error)

	// Hook side-effect methods (matching InCrowdAPI Scala lifecycle hooks)
	AssignConferenceHash(ctx context.Context, timeSlotID int64) error
	AssignConferencePin(ctx context.Context, timeSlotID int64) error
	GetTimeSlotEvents(ctx context.Context, timeSlotID int64, userID, observerID *int64, role int) ([]TimeSlotEvent, error)
	DeleteTimeSlotEvent(ctx context.Context, id int64) error
	DeleteConferenceInvitations(ctx context.Context, timeSlotID int64, userID, observerID *int64, role int) error
	GetObserverByEmail(ctx context.Context, projectID, timeSlotID int64, email string) (*ICObserver, error)
	CreateConferenceInvitation(ctx context.Context, timeSlotID, observerID int64) error
	CreateTimeSlotEvent(ctx context.Context, timeSlotID int64, gcalEventID string, role int, userID, observerID *int64) error
	GetTimeSlotTimes(ctx context.Context, timeSlotID int64) (start, end time.Time, err error)
	CalculateSLCompletions(ctx context.Context, surveyID int64) error
}

// UserRepository defines the contract for UserRepo.
type UserRepository interface {
	List(ctx context.Context, page, pageSize int, roleID *int64, search string) ([]ICUserListRow, int, error)
	GetByID(ctx context.Context, id int64) (*ICUserWithRoles, error)
	GetByEmail(ctx context.Context, email string) (*ICUserWithRoles, error)
	Update(ctx context.Context, id int64, firstName, lastName, timeZone string) error
}

// Compile-time interface satisfaction checks.
var (
	_ ProjectRepository = (*ProjectRepo)(nil)
	_ SurveyRepository  = (*SurveyRepo)(nil)
	_ UserRepository    = (*UserRepo)(nil)
)
