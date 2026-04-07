package service

import (
	"github.com/InCrowd/unified-qual-api/internal/config"
	"github.com/InCrowd/unified-qual-api/internal/database"
	"github.com/InCrowd/unified-qual-api/internal/integration"
	"github.com/InCrowd/unified-qual-api/internal/repository/adapter"
	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
)

// Deps holds shared dependencies injected into all domain handlers.
// Handlers access business logic exclusively through service fields.
// DB is retained solely for the Health endpoint.
type Deps struct {
	Cfg      *config.Config
	DB       *database.DBPair // used only by Health endpoint
	Adapters *adapter.Adapters

	AuthService         *AuthService
	ProjectService      *ProjectService
	ParticipantService  *ParticipantService
	BookingService      *BookingService
	TranslationService  *TranslationService
	AdminService        *AdminService
	ConferenceService   *ConferenceService
	PaymentService      *PaymentService
	NotificationService *NotificationService
	MediaService        *MediaService
	UserService         *UserService
	InterviewService    *InterviewService
	SurveyService       *SurveyService
	ModeratorService    *ModeratorService
	SubscriptionService *SubscriptionService
}

// Services holds all service layer instances.
type Services struct {
	Auth         *AuthService
	Project      *ProjectService
	Participant  *ParticipantService
	Booking      *BookingService
	Translation  *TranslationService
	Admin        *AdminService
	Conference   *ConferenceService
	Payment      *PaymentService
	Notification *NotificationService
	Media        *MediaService
	User         *UserService
	Interview    *InterviewService
	Survey       *SurveyService
	Moderator    *ModeratorService
	Subscription *SubscriptionService
	Adapters     *adapter.Adapters
}

// NewServices creates all service instances, wiring repositories,
// integration clients, and config values.
func NewServices(
	cfg *config.Config,
	qsRepos *qs.Repositories,
	irisRepos *iris.Repositories,
	clients *integration.ServiceClients,
) *Services {
	// Nil-safe repo access
	var (
		qsProject    qs.ProjectRepository
		qsUser       qs.UserRepository
		qsTimeSlot   qs.TimeSlotRepository
		qsRespondent qs.RespondentRepository
		qsSurvey     qs.SurveyRepository
		qsConference qs.ConferenceRepository
		qsAnswer     qs.AnswerRepository
		qsInterviews qs.InterviewsRepository
	)
	if qsRepos != nil {
		qsProject = qsRepos.Project
		qsUser = qsRepos.User
		qsTimeSlot = qsRepos.TimeSlot
		qsRespondent = qsRepos.Respondent
		qsSurvey = qsRepos.Survey
		qsConference = qsRepos.Conference
		qsAnswer = qsRepos.Answer
		qsInterviews = qsRepos.Interviews
	}

	var (
		irisProject iris.ProjectRepository
		irisUser    iris.UserRepository
		irisSurvey  iris.SurveyRepository
	)
	if irisRepos != nil {
		irisProject = irisRepos.Project
		irisUser = irisRepos.User
		irisSurvey = irisRepos.Survey
	}

	// Nil-safe integration client access
	var (
		icAuth       *integration.InCrowdAPIAuthClient
		s3Client     *integration.S3Client
		conference   *integration.ConferenceClient
		castingWords *integration.CastingWordsClient
		sms          *integration.SMSClient
		notification *integration.NotificationClient
		lambda       *integration.LambdaClient
		stepFn       *integration.StepFunctionsClient
		eventLog     *integration.EventLogClient
		decipher     *integration.DecipherClient
		googleCal    *integration.GoogleCalendarClient
		googleSheets *integration.GoogleSheetsClient
	)
	if clients != nil {
		icAuth = clients.ICAuth
		s3Client = clients.S3
		conference = clients.Conference
		castingWords = clients.CastingWords
		sms = clients.SMS
		notification = clients.Notification
		lambda = clients.Lambda
		stepFn = clients.StepFn
		eventLog = clients.EventLog
		decipher = clients.Decipher
		googleCal = clients.GoogleCal
		googleSheets = clients.GoogleSheets
	}

	return &Services{
		Auth:         NewAuthService(cfg, icAuth, qsUser),
		Project:      NewProjectService(irisProject, qsProject, s3Client),
		Participant:  NewParticipantService(qsRespondent, qsTimeSlot),
		Booking:      NewBookingService(qsTimeSlot),
		Translation:  NewTranslationService(qsAnswer, qsProject),
		Admin:        NewAdminService(qsUser, irisUser, qsTimeSlot),
		Conference:   NewConferenceService(qsConference, conference, castingWords, sms, notification),
		Payment:      NewPaymentService(qsAnswer, qsTimeSlot, qsProject, lambda, stepFn, cfg.AWS.Environment, cfg.AWS.ICApiURL),
		Notification: NewNotificationService(irisSurvey, qsAnswer),
		Media:        NewMediaService(irisSurvey, s3Client, cfg.S3.RecordingBucket),
		User:         NewUserService(qsUser, irisUser, eventLog, cfg.Cognito.Region, cfg.Cognito.AppClientID),
		Interview:    NewInterviewService(qsTimeSlot, qsInterviews),
		Survey:       NewSurveyService(qsSurvey, irisSurvey, decipher, eventLog),
		Moderator:    NewModeratorService(qsUser, qsTimeSlot, googleCal, googleSheets),
		Subscription: NewSubscriptionService(irisSurvey, s3Client, notification, cfg.InquiryEmailRecipient),
		Adapters:     adapter.NewAdapters(irisRepos, qsRepos),
	}
}
