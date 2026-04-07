package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/integration"
	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
)

// SurveyService encapsulates survey, crowd, and market logic.
type SurveyService struct {
	qsRepo   qs.SurveyRepository
	irisRepo iris.SurveyRepository
	decipher *integration.DecipherClient
	eventLog *integration.EventLogClient
}

// NewSurveyService creates a new SurveyService.
func NewSurveyService(qsRepo qs.SurveyRepository, irisRepo iris.SurveyRepository, decipher *integration.DecipherClient, eventLog *integration.EventLogClient) *SurveyService {
	return &SurveyService{qsRepo: qsRepo, irisRepo: irisRepo, decipher: decipher, eventLog: eventLog}
}

// QsAvailable returns true if the QS survey repository is configured.
func (s *SurveyService) QsAvailable() bool { return s.qsRepo != nil }

// IrisAvailable returns true if the IRIS survey repository is configured.
func (s *SurveyService) IrisAvailable() bool { return s.irisRepo != nil }

// --- QsSurveyRepo methods ---

func (s *SurveyService) List(ctx context.Context, search string) ([]qs.SurveyRow, error) {
	return s.qsRepo.List(ctx, search)
}

func (s *SurveyService) GetByID(ctx context.Context, id int64) (*qs.SurveyRow, error) {
	return s.qsRepo.GetByID(ctx, id)
}

func (s *SurveyService) Create(ctx context.Context, projectID *int64, title, status string, questions, rules json.RawMessage) (int64, error) {
	return s.qsRepo.Create(ctx, projectID, title, status, questions, rules)
}

func (s *SurveyService) Update(ctx context.Context, id int64, title, status string, questions, rules json.RawMessage) error {
	return s.qsRepo.Update(ctx, id, title, status, questions, rules)
}

func (s *SurveyService) Delete(ctx context.Context, id int64) error {
	return s.qsRepo.Delete(ctx, id)
}

// --- IrisSurveyRepo methods ---

func (s *SurveyService) GetSurvey(ctx context.Context, id int64) (*iris.ICSurvey, error) {
	return s.irisRepo.GetSurvey(ctx, id)
}

func (s *SurveyService) CloseSurvey(ctx context.Context, id int64) error {
	return s.irisRepo.CloseSurvey(ctx, id)
}

func (s *SurveyService) ToggleFavorite(ctx context.Context, surveyID, userID int64, favorite bool) error {
	return s.irisRepo.ToggleFavorite(ctx, surveyID, userID, favorite)
}

func (s *SurveyService) ValidateSurvey(ctx context.Context, id int64) ([]string, error) {
	return s.irisRepo.ValidateSurvey(ctx, id)
}

func (s *SurveyService) ValidateSurveyWarnings(ctx context.Context, surveyID int64) []string {
	return s.irisRepo.ValidateSurveyWarnings(ctx, surveyID)
}

func (s *SurveyService) CalculateSLCompletions(ctx context.Context, surveyID int64) error {
	return s.irisRepo.CalculateSLCompletions(ctx, surveyID)
}

func (s *SurveyService) GetSurveyCrowds(ctx context.Context, surveyID int64) ([]iris.ICSurveyCrowd, error) {
	return s.irisRepo.GetSurveyCrowds(ctx, surveyID)
}

func (s *SurveyService) CountSurveyCrowdAnswers(ctx context.Context, surveyID, crowdID int64) int64 {
	return s.irisRepo.CountSurveyCrowdAnswers(ctx, surveyID, crowdID)
}

func (s *SurveyService) GetSHCStatus(ctx context.Context, surveyCrowdID int64) string {
	return s.irisRepo.GetSHCStatus(ctx, surveyCrowdID)
}

func (s *SurveyService) GetSHCHonorariumLevel(ctx context.Context, surveyCrowdID int64) *string {
	return s.irisRepo.GetSHCHonorariumLevel(ctx, surveyCrowdID)
}

func (s *SurveyService) GetCrowdGroupInfo(ctx context.Context, groupID int64) map[string]any {
	return s.irisRepo.GetCrowdGroupInfo(ctx, groupID)
}

func (s *SurveyService) GetSurveyCrowdVendors(ctx context.Context, surveyCrowdID int64) []string {
	return s.irisRepo.GetSurveyCrowdVendors(ctx, surveyCrowdID)
}

func (s *SurveyService) GetCrowdMarketHonoGroups(ctx context.Context, crowdID, surveyID int64) []map[string]any {
	return s.irisRepo.GetCrowdMarketHonoGroups(ctx, crowdID, surveyID)
}

func (s *SurveyService) GetSurveyCustomHonoReasonIDs(ctx context.Context, surveyID int64) []int64 {
	return s.irisRepo.GetSurveyCustomHonoReasonIDs(ctx, surveyID)
}

func (s *SurveyService) GetMultiProfessionHono(ctx context.Context, surveyCrowdID int64) []map[string]any {
	return s.irisRepo.GetMultiProfessionHono(ctx, surveyCrowdID)
}

func (s *SurveyService) GetCrowdCurrency(ctx context.Context, crowdID int64) string {
	return s.irisRepo.GetCrowdCurrency(ctx, crowdID)
}

func (s *SurveyService) GetCrowdAttributesRemoved(ctx context.Context, surveyCrowdID int64) []map[string]any {
	return s.irisRepo.GetCrowdAttributesRemoved(ctx, surveyCrowdID)
}

func (s *SurveyService) UserCanReadProject(ctx context.Context, userID, projectID int64) (bool, error) {
	return s.irisRepo.UserCanReadProject(ctx, userID, projectID)
}

func (s *SurveyService) GetCrowdByID(ctx context.Context, crowdID int64) (*iris.ICCrowd, error) {
	return s.irisRepo.GetCrowdByID(ctx, crowdID)
}

func (s *SurveyService) GetCrowdSize(ctx context.Context, crowdID int64) int64 {
	return s.irisRepo.GetCrowdSize(ctx, crowdID)
}

func (s *SurveyService) GetCrowdBrandIDs(ctx context.Context, crowdID int64) ([]int64, error) {
	return s.irisRepo.GetCrowdBrandIDs(ctx, crowdID)
}

func (s *SurveyService) CountSurveyCrowdAnswersByBrand(ctx context.Context, surveyID, crowdID, brandID int64, count *int64) error {
	return s.irisRepo.CountSurveyCrowdAnswersByBrand(ctx, surveyID, crowdID, brandID, count)
}

func (s *SurveyService) GetCrowdAttributes(ctx context.Context, crowdID int64) []map[string]any {
	return s.irisRepo.GetCrowdAttributes(ctx, crowdID)
}

func (s *SurveyService) GetCrowdAvailableCount(ctx context.Context, surveyID, crowdID int64) int64 {
	return s.irisRepo.GetCrowdAvailableCount(ctx, surveyID, crowdID)
}

func (s *SurveyService) GetCrowdAvailableEligibleCount(ctx context.Context, surveyID, crowdID int64, fullMatch bool) int64 {
	return s.irisRepo.GetCrowdAvailableEligibleCount(ctx, surveyID, crowdID, fullMatch)
}

func (s *SurveyService) GetSurveyStatusLabel(ctx context.Context, statusCode int) (int, string, error) {
	return s.irisRepo.GetSurveyStatusLabel(ctx, statusCode)
}

func (s *SurveyService) GetSurveyType(ctx context.Context, typeID int) (map[string]any, error) {
	return s.irisRepo.GetSurveyType(ctx, typeID)
}

func (s *SurveyService) GetSubscriptionCompany(ctx context.Context, subscriptionID int64) (string, error) {
	return s.irisRepo.GetSubscriptionCompany(ctx, subscriptionID)
}

func (s *SurveyService) GetProjectName(ctx context.Context, projectID int64) (string, error) {
	return s.irisRepo.GetProjectName(ctx, projectID)
}

func (s *SurveyService) GetProjectTypeID(ctx context.Context, projectID int64) (int, error) {
	return s.irisRepo.GetProjectTypeID(ctx, projectID)
}

func (s *SurveyService) IsSurveyFavoriteOf(ctx context.Context, surveyID, userID int64) (bool, error) {
	return s.irisRepo.IsSurveyFavoriteOf(ctx, surveyID, userID)
}

func (s *SurveyService) CountSurveyCompletions(ctx context.Context, surveyID int64) (int, error) {
	return s.irisRepo.CountSurveyCompletions(ctx, surveyID)
}

func (s *SurveyService) CountSurveyQuestions(ctx context.Context, surveyID int64) (int, error) {
	return s.irisRepo.CountSurveyQuestions(ctx, surveyID)
}

func (s *SurveyService) CountSurveyCrowds(ctx context.Context, surveyID int64) (int, error) {
	return s.irisRepo.CountSurveyCrowds(ctx, surveyID)
}

func (s *SurveyService) GetSurveyPricing(ctx context.Context, surveyID int64) (map[string]any, error) {
	return s.irisRepo.GetSurveyPricing(ctx, surveyID)
}

func (s *SurveyService) GetUserProjectPermissions(ctx context.Context, userID, projectID int64) (map[string]any, error) {
	return s.irisRepo.GetUserProjectPermissions(ctx, userID, projectID)
}

func (s *SurveyService) GetFirstSurveyCrowdName(ctx context.Context, surveyID int64) (string, error) {
	return s.irisRepo.GetFirstSurveyCrowdName(ctx, surveyID)
}

func (s *SurveyService) CountScreenOutCompletions(ctx context.Context, surveyID int64) (int, error) {
	return s.irisRepo.CountScreenOutCompletions(ctx, surveyID)
}

// --- Market methods (IrisSurveyRepo) ---

func (s *SurveyService) ListMarkets(ctx context.Context, f *iris.MarketFilter) ([]iris.ICMarket, int, error) {
	return s.irisRepo.ListMarkets(ctx, f)
}

func (s *SurveyService) GetMarketNameTranslation(ctx context.Context, marketID int64, lang string) string {
	return s.irisRepo.GetMarketNameTranslation(ctx, marketID, lang)
}

func (s *SurveyService) GetMarketRollupTranslation(ctx context.Context, marketID int64, lang string) string {
	return s.irisRepo.GetMarketRollupTranslation(ctx, marketID, lang)
}

func (s *SurveyService) GetSubscriptionIDForAccount(ctx context.Context, accountID int64) *int64 {
	return s.irisRepo.GetSubscriptionIDForAccount(ctx, accountID)
}

func (s *SurveyService) ListMarketsWithNPI(ctx context.Context) ([]iris.ICMarket, error) {
	return s.irisRepo.ListMarketsWithNPI(ctx)
}

func (s *SurveyService) GetCrowdableAttributes(ctx context.Context, marketID int64) ([]map[string]any, error) {
	return s.irisRepo.GetCrowdableAttributes(ctx, marketID)
}

// --- Salesforce methods (IrisSurveyRepo, used in shared/user.go) ---

func (s *SurveyService) ListSalesforceProjects(ctx context.Context, f *iris.SalesforceProjectFilter) ([]iris.ICSalesforceProject, error) {
	return s.irisRepo.ListSalesforceProjects(ctx, f)
}

func (s *SurveyService) GetMonoProjectID(ctx context.Context, salesforceProjectID string) *int64 {
	return s.irisRepo.GetMonoProjectID(ctx, salesforceProjectID)
}

// --- Observer methods (IrisSurveyRepo, used in ls/timeslot.go) ---

func (s *SurveyService) ListObserversForTimeSlot(ctx context.Context, timeSlotID int64) ([]iris.ICObserver, error) {
	return s.irisRepo.ListObserversForTimeSlot(ctx, timeSlotID)
}

func (s *SurveyService) GetObserverByEmail(ctx context.Context, projectID, timeSlotID int64, email string) (*iris.ICObserver, error) {
	return s.irisRepo.GetObserverByEmail(ctx, projectID, timeSlotID, email)
}

func (s *SurveyService) PutObserversForTimeSlot(ctx context.Context, projectID, timeSlotID int64, toAdd, toDelete []string) error {
	return s.irisRepo.PutObserversForTimeSlot(ctx, projectID, timeSlotID, toAdd, toDelete)
}

// --- Timeslot moderator methods (IrisSurveyRepo, used in ls/timeslot.go) ---

func (s *SurveyService) GetModeratorsForTimeSlot(ctx context.Context, timeSlotID int64) ([]map[string]any, error) {
	return s.irisRepo.GetModeratorsForTimeSlot(ctx, timeSlotID)
}

func (s *SurveyService) GetPossibleModeratorsForTimeSlot(ctx context.Context, timeSlotID int64) ([]map[string]any, error) {
	return s.irisRepo.GetPossibleModeratorsForTimeSlot(ctx, timeSlotID)
}

func (s *SurveyService) AssignModeratorToTimeSlot(ctx context.Context, timeSlotID, moderatorID int64, isHost bool) (int64, error) {
	return s.irisRepo.AssignModeratorToTimeSlot(ctx, timeSlotID, moderatorID, isHost)
}

func (s *SurveyService) RemoveModeratorFromTimeSlot(ctx context.Context, timeSlotID, moderatorID int64) error {
	return s.irisRepo.RemoveModeratorFromTimeSlot(ctx, timeSlotID, moderatorID)
}

// --- Conference invitation / event methods (IrisSurveyRepo, used in ls/timeslot.go) ---

func (s *SurveyService) GetTimeSlotEvents(ctx context.Context, timeSlotID int64, userID, observerID *int64, role int) ([]iris.TimeSlotEvent, error) {
	return s.irisRepo.GetTimeSlotEvents(ctx, timeSlotID, userID, observerID, role)
}

func (s *SurveyService) DeleteTimeSlotEvent(ctx context.Context, id int64) error {
	return s.irisRepo.DeleteTimeSlotEvent(ctx, id)
}

func (s *SurveyService) DeleteConferenceInvitations(ctx context.Context, timeSlotID int64, userID, observerID *int64, role int) error {
	return s.irisRepo.DeleteConferenceInvitations(ctx, timeSlotID, userID, observerID, role)
}

func (s *SurveyService) CreateConferenceInvitation(ctx context.Context, timeSlotID, observerID int64) error {
	return s.irisRepo.CreateConferenceInvitation(ctx, timeSlotID, observerID)
}

func (s *SurveyService) CreateTimeSlotEvent(ctx context.Context, timeSlotID int64, gcalEventID string, role int, userID, observerID *int64) error {
	return s.irisRepo.CreateTimeSlotEvent(ctx, timeSlotID, gcalEventID, role, userID, observerID)
}

func (s *SurveyService) GetTimeSlotTimes(ctx context.Context, timeSlotID int64) (time.Time, time.Time, error) {
	return s.irisRepo.GetTimeSlotTimes(ctx, timeSlotID)
}

func (s *SurveyService) GetTimeSlotAdminJSON(ctx context.Context, timeSlotID int64) (map[string]any, error) {
	return s.irisRepo.GetTimeSlotAdminJSON(ctx, timeSlotID)
}

// --- Moderator availability methods (IrisSurveyRepo, used in ls/moderator.go) ---

func (s *SurveyService) ListModeratorAvailability(ctx context.Context, moderatorID, subscriptionID int64) ([]iris.ICModeratorAvailability, error) {
	return s.irisRepo.ListModeratorAvailability(ctx, moderatorID, subscriptionID)
}

func (s *SurveyService) CreateModeratorAvailability(ctx context.Context, moderatorID, subscriptionID int64, startTime, endTime time.Time) (int64, error) {
	return s.irisRepo.CreateModeratorAvailability(ctx, moderatorID, subscriptionID, startTime, endTime)
}

func (s *SurveyService) UpdateModeratorAvailability(ctx context.Context, id int64, startTime, endTime time.Time) error {
	return s.irisRepo.UpdateModeratorAvailability(ctx, id, startTime, endTime)
}

func (s *SurveyService) DeleteModeratorAvailability(ctx context.Context, id int64) error {
	return s.irisRepo.DeleteModeratorAvailability(ctx, id)
}

func (s *SurveyService) AssignConferenceHash(ctx context.Context, timeSlotID int64) error {
	return s.irisRepo.AssignConferenceHash(ctx, timeSlotID)
}

func (s *SurveyService) AssignConferencePin(ctx context.Context, timeSlotID int64) error {
	return s.irisRepo.AssignConferencePin(ctx, timeSlotID)
}

func (s *SurveyService) GetNoShowCheck(ctx context.Context) (map[string]any, error) {
	return s.irisRepo.GetNoShowCheck(ctx)
}

func (s *SurveyService) MarkNoShow(ctx context.Context, projectID, timeSlotID int64) error {
	return s.irisRepo.MarkNoShow(ctx, projectID, timeSlotID)
}

// --- Project-level IrisSurveyRepo methods (used in ls/project.go) ---

func (s *SurveyService) ListSurveysForProject(ctx context.Context, projectID int64) ([]iris.ICSurvey, error) {
	return s.irisRepo.ListSurveysForProject(ctx, projectID)
}

func (s *SurveyService) ListUserProjects(ctx context.Context, projectID int64) ([]map[string]any, error) {
	return s.irisRepo.ListUserProjects(ctx, projectID)
}

func (s *SurveyService) ListObserversForProject(ctx context.Context, projectID int64) ([]iris.ICObserver, error) {
	return s.irisRepo.ListObserversForProject(ctx, projectID)
}

func (s *SurveyService) GetQualRescheduleBody(ctx context.Context, projectID int64) (string, error) {
	return s.irisRepo.GetQualRescheduleBody(ctx, projectID)
}

func (s *SurveyService) GetProjectAvailability(ctx context.Context, projectID int64) ([]iris.ICModeratorAvailability, error) {
	return s.irisRepo.GetProjectAvailability(ctx, projectID)
}

func (s *SurveyService) GetSchedulerModerators(ctx context.Context, projectID int64) ([]map[string]any, error) {
	return s.irisRepo.GetSchedulerModerators(ctx, projectID)
}

func (s *SurveyService) GetProjectDashboardInfo(ctx context.Context, projectID int64) (map[string]any, error) {
	return s.irisRepo.GetProjectDashboardInfo(ctx, projectID)
}

func (s *SurveyService) ResetProjectModerators(ctx context.Context, projectID int64) (int64, error) {
	return s.irisRepo.ResetProjectModerators(ctx, projectID)
}

func (s *SurveyService) ExportProjectData(ctx context.Context, projectID int64) ([]map[string]any, error) {
	return s.irisRepo.ExportProjectData(ctx, projectID)
}

func (s *SurveyService) GetAvailableModeratorsCount(ctx context.Context, projectID int64) (int, error) {
	return s.irisRepo.GetAvailableModeratorsCount(ctx, projectID)
}

func (s *SurveyService) GetUnavailableModerators(ctx context.Context, projectID int64) ([]map[string]any, error) {
	return s.irisRepo.GetUnavailableModerators(ctx, projectID)
}

// --- QsSurveyRepo MRA method (used in mra/moderator.go) ---

func (s *SurveyService) GetSurveyByIdMRA(ctx context.Context, surveyID int64) (map[string]any, error) {
	return s.qsRepo.GetSurveyByIdMRA(ctx, surveyID)
}

// CreateIrisActivityLog inserts a row into the IRIS activity_log table.
func (s *SurveyService) CreateIrisActivityLog(ctx context.Context, eventType, description string, userID, projectID, timeSlotID int64, metaData string) error {
	return s.irisRepo.CreateActivityLog(ctx, eventType, description, userID, projectID, timeSlotID, metaData)
}

// CreateIrisActivityLogSimple inserts a simpler activity_log entry (no user/project/timeslot IDs).
func (s *SurveyService) CreateIrisActivityLogSimple(ctx context.Context, eventType, description, metaData string) error {
	return s.irisRepo.CreateActivityLog(ctx, eventType, description, 0, 0, 0, metaData)
}

// --- Decipher / EventLog delegation methods ---

func (s *SurveyService) DecipherConfigured() bool {
	return s.decipher != nil && s.decipher.Configured()
}

func (s *SurveyService) GetDecipherRespondentData(ctx context.Context, surveyID string) ([]integration.DecipherRespondent, error) {
	return s.decipher.GetRespondentData(ctx, surveyID)
}

func (s *SurveyService) GetDecipherRespondentByIdentifier(ctx context.Context, surveyID, identifier string) (*integration.DecipherRespondent, error) {
	return s.decipher.GetRespondentByIdentifier(ctx, surveyID, identifier)
}

func (s *SurveyService) EventLogConfigured() bool {
	return s.eventLog != nil && s.eventLog.Configured()
}

func (s *SurveyService) LogEvent(ctx context.Context, eventType, description string, metadata map[string]any) error {
	return s.eventLog.LogEvent(ctx, eventType, description, metadata)
}
