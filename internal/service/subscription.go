package service

import (
	"context"

	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
)

// SubscriptionService encapsulates subscription, crowd, and inquiry logic.
type SubscriptionService struct {
	irisRepo iris.SurveyRepository
}

// NewSubscriptionService creates a new SubscriptionService.
func NewSubscriptionService(irisRepo iris.SurveyRepository) *SubscriptionService {
	return &SubscriptionService{irisRepo: irisRepo}
}

// Available returns true if the IRIS survey repository is configured.
func (s *SubscriptionService) Available() bool { return s.irisRepo != nil }

// --- IrisSurveyRepo delegation methods ---

func (s *SubscriptionService) ListSurveysForProject(ctx context.Context, projectID int64) ([]iris.ICSurvey, error) {
	return s.irisRepo.ListSurveysForProject(ctx, projectID)
}

func (s *SubscriptionService) ListProjectsForSubscription(ctx context.Context, subscriptionID int64) ([]iris.ICProjectBasic, error) {
	return s.irisRepo.ListProjectsForSubscription(ctx, subscriptionID)
}

func (s *SubscriptionService) ListCrowdsForSubscription(ctx context.Context, subscriptionID int64, f *iris.CrowdFilter) ([]iris.ICCrowd, int, error) {
	return s.irisRepo.ListCrowdsForSubscription(ctx, subscriptionID, f)
}

func (s *SubscriptionService) ListProjectInquiries(ctx context.Context, subscriptionID int64, f *iris.InquiryFilter) ([]iris.ICProjectInquiry, error) {
	return s.irisRepo.ListProjectInquiries(ctx, subscriptionID, f)
}

func (s *SubscriptionService) GetSubscriptionInterviews(ctx context.Context, subscriptionID int64) ([]map[string]any, error) {
	return s.irisRepo.GetSubscriptionInterviews(ctx, subscriptionID)
}

func (s *SubscriptionService) GetSubscriptionCompanyAndShortCode(ctx context.Context, subscriptionID int64) (string, string, error) {
	return s.irisRepo.GetSubscriptionCompanyAndShortCode(ctx, subscriptionID)
}

func (s *SubscriptionService) GetSubscriptionServiceDiscount(ctx context.Context, subscriptionID int64) (float64, error) {
	return s.irisRepo.GetSubscriptionServiceDiscount(ctx, subscriptionID)
}

func (s *SubscriptionService) GetAccountIDForSubscription(ctx context.Context, subscriptionID int64) *int64 {
	return s.irisRepo.GetAccountIDForSubscription(ctx, subscriptionID)
}

func (s *SubscriptionService) GetSurveyStatusLabel(ctx context.Context, statusCode int) (int, string, error) {
	return s.irisRepo.GetSurveyStatusLabel(ctx, statusCode)
}

func (s *SubscriptionService) GetFirstSurveyCrowdName(ctx context.Context, surveyID int64) (string, error) {
	return s.irisRepo.GetFirstSurveyCrowdName(ctx, surveyID)
}

func (s *SubscriptionService) CountSurveyQuestions(ctx context.Context, surveyID int64) (int, error) {
	return s.irisRepo.CountSurveyQuestions(ctx, surveyID)
}

func (s *SubscriptionService) CountSurveyCompletions(ctx context.Context, surveyID int64) (int, error) {
	return s.irisRepo.CountSurveyCompletions(ctx, surveyID)
}

func (s *SubscriptionService) IsSurveyFavoriteOf(ctx context.Context, surveyID, userID int64) (bool, error) {
	return s.irisRepo.IsSurveyFavoriteOf(ctx, surveyID, userID)
}

func (s *SubscriptionService) GetUserIDByEmail(ctx context.Context, email string) (int64, error) {
	return s.irisRepo.GetUserIDByEmail(ctx, email)
}

func (s *SubscriptionService) GetAllQuestionTypes(ctx context.Context) ([]map[string]any, error) {
	return s.irisRepo.GetAllQuestionTypes(ctx)
}

func (s *SubscriptionService) GetQualProducts(ctx context.Context) ([]iris.QualProduct, error) {
	return s.irisRepo.GetQualProducts(ctx)
}

func (s *SubscriptionService) GetProductRelatedMarketIDs(ctx context.Context, productID int64) ([]int64, error) {
	return s.irisRepo.GetProductRelatedMarketIDs(ctx, productID)
}

func (s *SubscriptionService) CountMarketPopulation(ctx context.Context, marketID int64) (int64, error) {
	return s.irisRepo.CountMarketPopulation(ctx, marketID)
}

func (s *SubscriptionService) GetDifficultyLevelPercent(ctx context.Context, levelID int64) (float64, error) {
	return s.irisRepo.GetDifficultyLevelPercent(ctx, levelID)
}

func (s *SubscriptionService) GetDifficultyAssessments(ctx context.Context) ([]iris.DifficultyAssessmentRow, error) {
	return s.irisRepo.GetDifficultyAssessments(ctx)
}

func (s *SubscriptionService) GetCrowdTypeDescription(ctx context.Context, typeID int) (string, error) {
	return s.irisRepo.GetCrowdTypeDescription(ctx, typeID)
}

func (s *SubscriptionService) GetMarketName(ctx context.Context, marketID int64) (string, error) {
	return s.irisRepo.GetMarketName(ctx, marketID)
}

func (s *SubscriptionService) GetCrowdBrandIDs(ctx context.Context, crowdID int64) ([]int64, error) {
	return s.irisRepo.GetCrowdBrandIDs(ctx, crowdID)
}

func (s *SubscriptionService) GetBrandName(ctx context.Context, brandID int64) (string, error) {
	return s.irisRepo.GetBrandName(ctx, brandID)
}

func (s *SubscriptionService) GetCrowdCountryID(ctx context.Context, crowdID int64) int64 {
	return s.irisRepo.GetCrowdCountryID(ctx, crowdID)
}

func (s *SubscriptionService) GetAttributeChoiceLabel(ctx context.Context, choiceID int64) string {
	return s.irisRepo.GetAttributeChoiceLabel(ctx, choiceID)
}

func (s *SubscriptionService) GetCountryLanguages(ctx context.Context, countryID int64, countryName string) []string {
	return s.irisRepo.GetCountryLanguages(ctx, countryID, countryName)
}

func (s *SubscriptionService) CrowdHasListMatch(ctx context.Context, crowdID int64) bool {
	return s.irisRepo.CrowdHasListMatch(ctx, crowdID)
}

func (s *SubscriptionService) GetCrowdSpecialtyIDs(ctx context.Context, crowdID int64) []int {
	return s.irisRepo.GetCrowdSpecialtyIDs(ctx, crowdID)
}

func (s *SubscriptionService) GetCrowdEngagementRate(ctx context.Context, crowdID int64, fullMatch bool) *float64 {
	return s.irisRepo.GetCrowdEngagementRate(ctx, crowdID, fullMatch)
}

func (s *SubscriptionService) GetCrowdNameByID(ctx context.Context, crowdID int64) (string, error) {
	return s.irisRepo.GetCrowdNameByID(ctx, crowdID)
}

func (s *SubscriptionService) GetSalesforceProjectByExtID(ctx context.Context, sfProjectID string) (string, string, error) {
	return s.irisRepo.GetSalesforceProjectByExtID(ctx, sfProjectID)
}

func (s *SubscriptionService) GetInquiryTypeName(ctx context.Context, typeID int64) string {
	return s.irisRepo.GetInquiryTypeName(ctx, typeID)
}

func (s *SubscriptionService) GetProjectInquiry(ctx context.Context, subscriptionID, projectID int64) (*iris.ICProjectInquiry, error) {
	return s.irisRepo.GetProjectInquiry(ctx, subscriptionID, projectID)
}

func (s *SubscriptionService) GetProjectForInquiry(ctx context.Context, projectID int64) (map[string]any, error) {
	return s.irisRepo.GetProjectForInquiry(ctx, projectID)
}

func (s *SubscriptionService) GetProjectFees(ctx context.Context, projectID int64) ([]map[string]any, error) {
	return s.irisRepo.GetProjectFees(ctx, projectID)
}

func (s *SubscriptionService) GetProjectInquiryCrowds(ctx context.Context, inquiryID int64) (
	standardCrowds []map[string]any, customCrowds []map[string]any,
	crowdObjects []map[string]any, err error,
) {
	return s.irisRepo.GetProjectInquiryCrowds(ctx, inquiryID)
}

// ListSubscriptionsForQual returns subscriptions that have qual projects.
func (s *SubscriptionService) ListSubscriptionsForQual(ctx context.Context) ([]iris.SubscriptionRow, error) {
	return s.irisRepo.ListSubscriptionsForQual(ctx)
}

// GetSubscriptionCompanyByID returns the company name for a subscription.
func (s *SubscriptionService) GetSubscriptionCompanyByID(ctx context.Context, subscriptionID int64) (string, error) {
	return s.irisRepo.GetSubscriptionCompanyByID(ctx, subscriptionID)
}
