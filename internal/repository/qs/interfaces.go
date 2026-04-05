package qs

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// AnswerRepository defines the contract for AnswerRepo.
type AnswerRepository interface {
	ListByTimeSlot(ctx context.Context, timeSlotID int64) ([]map[string]any, error)
	GetDetails(ctx context.Context, answerID int64) ([]AnswerDetail, error)
	ListTopics(ctx context.Context, projectID int64) ([]Topic, error)
	ListLocales(ctx context.Context) ([]LanguageLocalisation, error)
	ListHonorariumAmounts(ctx context.Context) ([]HonorariumAmount, error)
	ListPaymentHistory(ctx context.Context, timeSlotID int64) ([]PaymentHistory, error)
	CreatePaymentRecord(ctx context.Context, timeSlotID int64, amount int, paymentType, status string) (int64, error)
	CreateCustomHonorarium(ctx context.Context, timeSlotID int64, amount int, reason string) (int64, error)
	ListSalesforceProjects(ctx context.Context) ([]QSSalesforceProject, error)
	ListSalesforceAccounts(ctx context.Context) ([]QSSalesforceAccount, error)
	ListExternalClients(ctx context.Context) ([]ExternalClient, error)
	ListProjectsUsers(ctx context.Context, projectID int64) ([]map[string]any, error)
	GetCommunicationTemplate(ctx context.Context, name string) (*CommunicationTemplate, error)
	ListCommunicationTemplates(ctx context.Context) ([]CommunicationTemplate, error)
	GetParticipantEligibility(ctx context.Context, responderID, projectID int64) (map[string]any, error)
	ListNativeSurveysByProject(ctx context.Context, projectID int64) ([]NativeSurvey, error)
	UpdateTopicTranslation(ctx context.Context, projectID int64, topicID int64, languageCode, translatedName string) error
	DeleteTopicTranslation(ctx context.Context, topicID int64, languageCode string) error
	DeleteTranslation(ctx context.Context, projectID int64, languageCode string) error
	ListHonorariumReasons(ctx context.Context) ([]map[string]any, error)
	CreateExternalPayment(ctx context.Context, timeSlotID int64, amount int, paymentType, status, externalRef string) (int64, error)
	ListInterviewPaymentStatuses(ctx context.Context) ([]map[string]any, error)
}

// ConferenceRepository defines the contract for ConferenceRepo.
type ConferenceRepository interface {
	GetByHash(ctx context.Context, hash string) (*ConferenceInvitation, error)
	GetParticipants(ctx context.Context, timeSlotID int64) ([]map[string]any, error)
	GetAttendeesByMeetingID(ctx context.Context, meetingID string) ([]map[string]any, error)
	GetMeetingMetadata(ctx context.Context, conferenceHash string) (map[string]any, error)
	Login(ctx context.Context, conferenceHash, pin string) (map[string]any, error)
	CreateConferenceLink(ctx context.Context, timeSlotID, projectID int64, conferenceHash string) (int64, error)
	UpdateConferenceLink(ctx context.Context, timeSlotID int64, conferenceHash, pin string) error
	GetConferenceLinkByTimeSlotID(ctx context.Context, timeSlotID int64) (map[string]any, error)
	UpdateRecordingStatus(ctx context.Context, meetingID, status, bucket, key string) error
	AddConferenceLinkMRA(ctx context.Context, projectID int64, participantGroupID int64, userID int64, conferenceLink string, meetingInformation [][]any) (map[string]any, error)
	GetExistingMeetingLanguagesMRA(ctx context.Context, projectID int64) (map[string]bool, error)
	UpdateConferenceLinkMRA(ctx context.Context, projectID, participantGroupID int64, userID int64, conferenceLink string, meetingInformation [][]any, existingLangs map[string]bool) error
	GetPendingTimeSlotsCountMRA(ctx context.Context, projectID int64) (int64, error)
	GetConferenceLinkByProjectMRA(ctx context.Context, projectID int64) ([]map[string]any, error)
	GetConferenceLinkByTimeSlotMRA(ctx context.Context, timeSlotID int64) (map[string]any, error)
}

// InterviewsRepository defines the contract for InterviewsRepo.
type InterviewsRepository interface {
	GetAllInterviewsByOffsetAndActiveTab(
		ctx context.Context,
		clientID int64,
		externalClientsIDs []string,
		projectsIDs []string,
		search string,
		offset int,
		activeTab string,
		paymentStatusCode string,
	) ([]map[string]any, error)
}

// ProjectRepository defines the contract for ProjectRepo.
type ProjectRepository interface {
	List(ctx context.Context, page, pageSize int, statusID *int, search string) ([]ProjectListRow, int, error)
	GetByID(ctx context.Context, id int64) (*Project, error)
	GetTopics(ctx context.Context, projectID int64) ([]Topic, error)
	Create(ctx context.Context, p *Project) (int64, error)
	CreateProjectFull(ctx context.Context, req map[string]any) (map[string]any, error)
	GetSalesForceJobNumberText(ctx context.Context, sfProjectID string) (string, error)
	GetProjectDetailsMRA(ctx context.Context, projectID int64) (map[string]any, error)
	GetProjectsMRA(ctx context.Context, creatorID, status int, sort, search string, externalClientIDs []string) ([]map[string]any, error)
	GetProjectsForModsMRA(ctx context.Context, clientID int64) ([]map[string]any, error)
	SaveUserSelection(ctx context.Context, userID int64, accountIDs, clientIDs []string) error
	UpdateSchedulerGenerated(ctx context.Context, projectID int64) error
	UpdatePostScreenInBuffer(ctx context.Context, projectID int64, buffer float64) error
	UpdateModeratorBufferMRA(ctx context.Context, projectID int64, buffer float64) error
	UpdateExternalSurveyID(ctx context.Context, projectID int64, surveyID string) error
	GetProjectModeratorIDs(ctx context.Context, projectID int64) ([]int64, error)
	ResetProjectModeratorsMRA(ctx context.Context, projectID int64, newIDs, existingIDs []int64) error
	UnassignModeratorFromProject(ctx context.Context, userID, projectID int64) error
	GetModeratorsList(ctx context.Context, projectID int64) ([]map[string]any, error)
	GetEmailTemplateMRA(ctx context.Context, typeID int, language string) (map[string]any, error)
	HandleProjectExportMRA(ctx context.Context, projectID int64, pmTimeZone, pmTimeZoneAbbr, rescheduleLinkPrefix string) ([][]string, error)
	HandleProjectNoTimeslotExportMRA(ctx context.Context, projectID int64, pmTimeZone, pmTimeZoneAbbr string) ([][]string, error)
	GetProjectName(ctx context.Context, projectID int64) (string, error)
	Update(ctx context.Context, id int64, fields map[string]any) error
	TimeSlotCounts(ctx context.Context, projectID int64) (scheduled, completed int, err error)
	UpdateSampleSizeMRA(ctx context.Context, projectID int64, sampleSize int64) error
	UpdateSampleSizeProjectStatusMRA(ctx context.Context, projectID int64, sampleSize int64, projectStatusID int64) error
	GetModeratorsTimeRangePerProject(ctx context.Context, projectID int64) ([]ModeratorTimeRange, error)
	GetAllModeratorsAvailabilityPerClient(ctx context.Context, clientID int64, projectID int64) ([]ModeratorAvailability, error)
	GetProjectStatusByID(ctx context.Context, projectID int64) (int64, error)
	UpsertModeratorTimeRangePerProject(ctx context.Context, projectID, moderatorID int64, startTime, endTime, timezone string) error
	GetAllModeratorsAvailabilityPerRole(ctx context.Context, projectID int64) ([]ModeratorAvailability, error)
	GetModeratorsTimeRangePerProjectMRA(ctx context.Context, projectID int64) ([]map[string]any, error)
	GetProjectsForModeratorMRA(ctx context.Context, clientID, moderatorID int64) ([]map[string]any, error)
	GetAllAccountsMRA(ctx context.Context) ([]map[string]any, error)
	GetSalesforceClientsMRA(ctx context.Context) ([]map[string]any, error)
	GetSalesforceProjectsMRA(ctx context.Context, salesforceClientID string) ([]map[string]any, error)
	GetSalesforceClientsWithFilterMRA(ctx context.Context, projectAccountID int) ([]map[string]any, error)
	GetTopicsByProjectMRA(ctx context.Context, projectID int64) ([]map[string]any, error)
	GetProjectStatusByIdMRA(ctx context.Context, projectID int64) (int64, error)
	GetRespondersLanguagesByProjectIdMRA(ctx context.Context, projectID int64) ([]string, error)
	GetTopicsByProjectIdAndLanguageIdMRA(ctx context.Context, projectID, languageID int64) (bool, error)
	AddTopicByProjectAndLanguageMRA(ctx context.Context, topicName string, languageID, projectID int64, userID any) error
	UpdateTopicByProjectAndLanguageMRA(ctx context.Context, topicName string, languageID, projectID int64, userID any) error
	DeleteTopicByProjectAndLanguageMRA(ctx context.Context, projectID, languageID int64) error
	GetAllLanguageLocalisationsMRA(ctx context.Context) ([]map[string]any, error)
	GetDataFromLanguageLocalisationsMRA(ctx context.Context) ([]map[string]any, error)
	GetCountriesWithLocalisationsMRA(ctx context.Context) ([]map[string]any, error)
	DeleteMeetingInformationTranslationMRA(ctx context.Context, projectID, languageID int64) (map[string]any, error)
	AddHonorariumAmountMRA(ctx context.Context, projectID int64, honorarium int64, currency, sessKey string, extProjectID, extUserSurveyID, extUserID, extCreditOrderID, extCountryID string) error
	GetHonoValueUpdateReasonListMRA(ctx context.Context) ([]map[string]any, error)
}

// RespondentRepository defines the contract for RespondentRepo.
type RespondentRepository interface {
	List(ctx context.Context, page, pageSize int, search string) ([]RespondentListRow, int, error)
	GetByID(ctx context.Context, id int64) (*Respondent, error)
	GetCommunicationAddresses(ctx context.Context, responderID int64) ([]RespondentCommunicationAddress, error)
	Create(ctx context.Context, r *Respondent) (int64, error)
	CreateCommunicationAddress(ctx context.Context, responderID int64, transportTypeID int, address string) (int64, error)
	Update(ctx context.Context, id int64, fields map[string]any) error
	GetRespondentByExternalIdMRA(ctx context.Context, externalResponderID string, projectID int64) ([]map[string]any, error)
	GetRescheduleTokenMRA(ctx context.Context, projectID, responderID int64) (string, error)
	GetParticipantIdMRA(ctx context.Context, timeslotID int64) ([]map[string]any, error)
	CreateRespondentMRA(ctx context.Context, firstName, lastName, title, extID, sessKey, tz, tzAbbr, lang string) (int64, error)
}

// SurveyRepository defines the contract for SurveyRepo.
type SurveyRepository interface {
	EnsureTable(ctx context.Context) error
	List(ctx context.Context, search string) ([]SurveyRow, error)
	GetByID(ctx context.Context, id int64) (*SurveyRow, error)
	Create(ctx context.Context, projectID *int64, title, status string, questions, rules json.RawMessage) (int64, error)
	Update(ctx context.Context, id int64, title, status string, questions, rules json.RawMessage) error
	Delete(ctx context.Context, id int64) error
	GetSurveyByIdMRA(ctx context.Context, surveyID int64) (map[string]any, error)
}

// TimeSlotRepository defines the contract for TimeSlotRepo.
type TimeSlotRepository interface {
	List(ctx context.Context, page, pageSize int, projectID *int64, statusID *int, moderatorID *int64, from *time.Time, to *time.Time) ([]TimeSlotListRow, int, error)
	GetByID(ctx context.Context, id int64) (*TimeSlot, error)
	GetModerators(ctx context.Context, timeSlotID int64) ([]ModeratorTimeSlot, error)
	GetRespondent(ctx context.Context, timeSlotID int64) (*Respondent, error)
	Create(ctx context.Context, ts *TimeSlot) (int64, error)
	AssignModerator(ctx context.Context, moderatorID, timeSlotID int64, isHost bool) (int64, error)
	Update(ctx context.Context, id int64, fields map[string]any) error
	Delete(ctx context.Context, id int64) error
	ListByModerator(ctx context.Context, moderatorID int64, page, pageSize int) ([]TimeSlotListRow, int, error)
	ListByProject(ctx context.Context, projectID int64, page, pageSize int) ([]TimeSlotListRow, int, error)
	ListStatuses(ctx context.Context) ([]TimeSlotStatus, error)
	GetReward(ctx context.Context, timeSlotID int64) (*BookingReward, error)
	UpsertReward(ctx context.Context, timeSlotID int64, points int, status string) error
	InvalidateInterviewMRA(ctx context.Context, timeSlotID int64, reasonCode string, reasonText *string, invalidatedByUserID int64) error
	HasCompletedPaymentMRA(ctx context.Context, timeSlotID int64) (bool, error)
	GetInvalidTimeSlotStatusMRA(ctx context.Context, externalResponderID string, projectID int64) ([]map[string]any, error)
	GetInvalidTimeSlotStatusForRespRescMRA(ctx context.Context, externalResponderID string, projectID int64) ([]map[string]any, error)
	GetPendingTimeslotByProjectAndResponderMRA(ctx context.Context, projectID, responderID int64) ([]map[string]any, error)
	GetModeratorTimeSlotsMRA(ctx context.Context, moderatorID, clientID int64) ([]map[string]any, error)
	GetModeratorTimeSlotsWithFilterMRA(ctx context.Context, moderatorID, clientID int64, excludeProjectIDs []int64) ([]map[string]any, error)
	GetAllInterviewsMRA(ctx context.Context, moderatorID int64, search string, excludeProjectIDs []int64, paymentStatusCode string) ([]map[string]any, error)
	GetModeratorsForTimeSlotMRA(ctx context.Context, timeslotID int64) ([]map[string]any, error)
	GetStartEndTimeBySlotIdMRA(ctx context.Context, timeslotID int64) (string, string, error)
	GetModeratorsInfoForSlotMRA(ctx context.Context, timeslotID int64) ([]ModeratorInfoForSlotMRA, error)
	GetModeratorConflictForSlotMRA(ctx context.Context, moderatorID int64, startTime, endTime string, timeslotID int64) (int, error)
	GetPMTimeSlotsByClientIdMRA(ctx context.Context, clientID int64) ([]map[string]any, error)
	GetPMTimeSlotsByClientIdWithProjectFilterMRA(ctx context.Context, clientID int64, projectIDs []int64) ([]map[string]any, error)
	GetPMTimeSlotsByClientIdWithModeratorFilterMRA(ctx context.Context, clientID int64, moderatorIDs []int64) ([]map[string]any, error)
	GetAllPendingInterviewsPerProjectMRA(ctx context.Context, projectID, clientID int64) ([]map[string]any, error)
	UpsertEligibilityStatusMRA(ctx context.Context, participantID string, isEligible bool, reason, updatedBy string) error
	ResetIneligibleMailSentMRA(ctx context.Context, externalResponderID string) error
	GetPaymentInfoByTimeSlotIdsMRA(ctx context.Context, timeSlotIDs []int64) ([]map[string]any, error)
	GetTimeSlotPaymentTypeListMRA(ctx context.Context) ([]map[string]any, error)
	AddQSTimeSlotPaymentsMRA(ctx context.Context, payments []map[string]any) error
	AddExternalTimeSlotPaymentsMRA(ctx context.Context, payments []map[string]any) error
	GetUserByEmailMRA(ctx context.Context, email string) (int64, error)
	AddTimeSlotCustomHonorariumMRA(ctx context.Context, timeSlotID int64, oldValue, newValue float64, reasonID, createdBy int64) error
	GetExternalSurveyIdByTimeSlotIdMRA(ctx context.Context, timeSlotID int64) (string, error)
	GetTimeSlotPaymentStatusListMRA(ctx context.Context) ([]map[string]any, error)
	GetProjectModeratorsIdsMRA(ctx context.Context, projectID int64) ([]int64, error)
	ResetProjectModeratorsMRA(ctx context.Context, projectID int64, newModIDs, existingModIDs []int64) error
	UnassignModeratorFromProjectMRA(ctx context.Context, moderatorID, projectID int64) error
	GetPendingPaymentsMRA(ctx context.Context, tx *sql.Tx, timeSlotIDs []int64) ([]PendingPaymentRecord, error)
	UpdateCompletedPaymentHistoryMRA(ctx context.Context, tx *sql.Tx, timeSlotIDs []int64, paymentHistoryIDs []int64) error
	UpdateCanceledPaymentHistoryMRA(ctx context.Context, tx *sql.Tx, timeSlotIDs []int64, paymentHistoryIDs []int64) error
	UpdateFailedPaymentHistoryMRA(ctx context.Context, timeSlotIDs []int64) error
}

// UserRepository defines the contract for UserRepo.
type UserRepository interface {
	List(ctx context.Context, page, pageSize int, roleID *int, search string) ([]UserListRow, int, error)
	GetByID(ctx context.Context, id int64) (*UserWithRoles, error)
	GetByEmail(ctx context.Context, email string) (*UserWithRoles, error)
	GetModerators(ctx context.Context) ([]UserListRow, error)
	ListModeratorAvailability(ctx context.Context, moderatorID int64, clientID *int64, startDate, endDate string) ([]ModeratorAvailability, error)
	CreateModeratorAvailability(ctx context.Context, moderatorID, clientID int64, startTime, endTime time.Time) (*ModeratorAvailability, error)
	DeleteModeratorAvailability(ctx context.Context, id int64) error
	Update(ctx context.Context, id int64, firstName, lastName, timeZone string) error
	UpdateTimeZone(ctx context.Context, userID int64, timeZone string) error
	UpdateModeratorBuffer(ctx context.Context, id int64, buffer int) error
	Create(ctx context.Context, firstName, lastName, email, timeZone string, roleIDs []int) (int64, error)
	SoftDelete(ctx context.Context, id int64) error
	AddRoles(ctx context.Context, userID int64, roleIDs []int) error
	DeleteRoles(ctx context.Context, userID int64, roleIDs []int) error
	GetRoles(ctx context.Context, userID int64) ([]int, error)
	GetUserCommPreference(ctx context.Context, userID int64) ([]map[string]any, error)
	UpdateUserCommPreference(ctx context.Context, cognitoUserID string, pmUserID string, allowContactByEmail int) ([]map[string]any, error)
	AcceptTerms(ctx context.Context, userID int64) error
	ListByRole(ctx context.Context, roleID int, page, pageSize int) ([]UserListRow, int, error)
	UpdateModeratorAvailability(ctx context.Context, id int64, startTime, endTime time.Time) error
	GetByEmailIncludeDeleted(ctx context.Context, email string) (*UserWithRoles, error)
	RestoreByEmail(ctx context.Context, email string) error
	AddUserClient(ctx context.Context, userID, clientID int64) error
	CreateUserCommPrefs(ctx context.Context, userID int64, email, cognitoUserID string) error
	CreateUserCommPrefsAdmin(ctx context.Context, userID int64, email string) error
	GetEmailByCognitoID(ctx context.Context, cognitoID string) (string, error)
	CheckUserIsQsToolAndI2(ctx context.Context, email string) (map[string]any, error)
	GetAllUsersAdmin(ctx context.Context, cognitoUserID string) ([]map[string]any, error)
	GetAllModeratorsListMRA(ctx context.Context, clientID int64) ([]map[string]any, error)
	FetchUserInfoByUserIdMRA(ctx context.Context, userID int64) (int, int64, error)
	UpdateModeratorBufferMRA(ctx context.Context, userID int64, buffer int) error
	GetFutureModeratorTimeslotsMRA(ctx context.Context, moderatorID, clientID int64) ([]ModeratorTimeslotMRA, error)
	GetFutureAvailsWithProximityMRA(ctx context.Context, moderatorID, clientID int64) ([]AvailWithProximityMRA, error)
	GetFutureImportedAvailsWithProximityMRA(ctx context.Context, moderatorID, clientID int64) ([]AvailWithProximityMRA, error)
	GetModeratorAvailabilityLengthMRA(ctx context.Context, availID int64) (*AvailLengthMRA, error)
	GetImportedModeratorAvailabilityLengthMRA(ctx context.Context, availID int64) (*AvailLengthMRA, error)
	DeleteModeratorAvailabilityByIdMRA(ctx context.Context, id int64) error
	DeleteImportedModeratorAvailabilityByIdMRA(ctx context.Context, id int64) error
	UpdateModeratorAvailabilityStartTimeMRA(ctx context.Context, id int64, startTime time.Time) error
	UpdateModeratorAvailabilityEndTimeMRA(ctx context.Context, id int64, endTime time.Time) error
	UpdateImportedModeratorAvailabilityStartTimeMRA(ctx context.Context, id int64, startTime time.Time) error
	UpdateImportedModeratorAvailabilityEndTimeMRA(ctx context.Context, id int64, endTime time.Time) error
	CleanUpAvailabilitiesByModeratorIdMRA(ctx context.Context, moderatorID int64) error
	RemoveNestedAvailabilitiesMRA(ctx context.Context, moderatorID int64) error
	GetModeratorBufferMRA(ctx context.Context, moderatorID int64) (int, error)
	GetModExternalCalendarStatusMRA(ctx context.Context, moderatorID int64) (string, error)
	IsValidAvailabilityMRA(ctx context.Context, moderatorID int64, startTime, endTime string, buffer int) (int64, error)
	OverlappingAvailabilitiesMRA(ctx context.Context, moderatorID int64, startTime, endTime string) ([]map[string]any, error)
	GetAllModeratorAvailabilityMRA(ctx context.Context, moderatorID, clientID int64) ([]map[string]any, error)
	GetNonOverlappingManualAvailabilityMRA(ctx context.Context, moderatorID, clientID int64) ([]map[string]any, error)
	GetNonOverlappingImportedAvailabilityMRA(ctx context.Context, moderatorID, clientID int64) ([]map[string]any, error)
	GetAllOverlappingManualAvailabilityMRA(ctx context.Context, moderatorID, clientID int64) ([]map[string]any, error)
	GetAllOverlappingImportedAvailabilityMRA(ctx context.Context, moderatorID, clientID int64) ([]map[string]any, error)
	FindModeratorAvailabilityByIdMRA(ctx context.Context, id int64) (map[string]any, error)
	GetModeratorsInfoByAvailabilityIdMRA(ctx context.Context, availabilityID int64) (map[string]any, error)
	GetAllModeratorAvailabilityWithUserMRA(ctx context.Context, moderatorID, clientID int64) ([]map[string]any, error)
	GetOverlappingImportedAvailabilityMRA(ctx context.Context, moderatorID, clientID int64, startTime, endTime string) ([]map[string]any, error)
	AddModeratorAvailabilityFromImportedMRA(ctx context.Context, moderatorID, clientID int64, startTime, endTime string) error
	GetAllModeratorsAvailabilityPerClientMRA(ctx context.Context, clientID, projectID int64) ([]map[string]any, error)
	GetModeratorsListMRA(ctx context.Context, projectID int64) ([]map[string]any, error)
	GetAllProjectManagersListMRA(ctx context.Context) ([]map[string]any, error)
	GetAllModeratorsAvailabilityForPMMRA(ctx context.Context, clientID int64) ([]map[string]any, error)
	GetAllModeratorsAvailabilityForPMWithProjectFilterMRA(ctx context.Context, clientID int64, projectIDs []int64) ([]map[string]any, error)
	GetAllModeratorsAvailabilityForPMWithModeratorFilterMRA(ctx context.Context, clientID int64, moderatorIDs []int64) ([]map[string]any, error)
	UpdateModExternalCalendarUrlMRA(ctx context.Context, moderatorID int64, url, key string) error
	UpdateModExternalCalendarStatusMRA(ctx context.Context, moderatorID int64, status string) error
	DeleteImportedModeratorAvailabilityByModeratorMRA(ctx context.Context, moderatorID, clientID int64) error
	GetImportedModeratorAvailabilityMRA(ctx context.Context, moderatorID, clientID int64) ([]map[string]any, error)
	GetModeratorExternalCalendarMRA(ctx context.Context, moderatorID int64) ([]map[string]any, error)
	GetRunningImportProcessCountMRA(ctx context.Context) (int64, error)
	DeleteExternalCalStatusMRA(ctx context.Context, moderatorID int64) error
	GetImportedModeratorAvailabilityListMRA(ctx context.Context, moderatorID, clientID int64) ([]map[string]any, error)
	GetOverlappingManualAvailabilityMRA(ctx context.Context, moderatorID, clientID int64, startTime, endTime string) ([]map[string]any, error)
	DeleteImportedAvailByModAndIdMRA(ctx context.Context, moderatorID, id int64) error
	DeleteManualAvailByModAndIdMRA(ctx context.Context, moderatorID, id int64) error
	AddManualAvailabilityMRA(ctx context.Context, moderatorID, clientID int64, startTime, endTime string) error
}

// Compile-time interface satisfaction checks.
var (
	_ AnswerRepository     = (*AnswerRepo)(nil)
	_ ConferenceRepository = (*ConferenceRepo)(nil)
	_ InterviewsRepository = (*InterviewsRepo)(nil)
	_ ProjectRepository    = (*ProjectRepo)(nil)
	_ RespondentRepository = (*RespondentRepo)(nil)
	_ SurveyRepository     = (*SurveyRepo)(nil)
	_ TimeSlotRepository   = (*TimeSlotRepo)(nil)
	_ UserRepository       = (*UserRepo)(nil)
)
