package support

import (
	"github.com/InCrowd/unified-qual-api/internal/config"
	"github.com/InCrowd/unified-qual-api/internal/service"
)

// Deps holds shared dependencies injected into all domain handlers.
// Handlers access business logic exclusively through service fields.
// DB is retained solely for the Health endpoint (shared/handler.go).
type Deps struct {
	Cfg *config.Config
	DB  *config.DBPair // used only by Health endpoint

	AuthService         *service.AuthService
	ProjectService      *service.ProjectService
	ParticipantService  *service.ParticipantService
	BookingService      *service.BookingService
	TranslationService  *service.TranslationService
	AdminService        *service.AdminService
	ConferenceService   *service.ConferenceService
	PaymentService      *service.PaymentService
	NotificationService *service.NotificationService
	MediaService        *service.MediaService
	UserService         *service.UserService
	InterviewService    *service.InterviewService
	SurveyService       *service.SurveyService
	ModeratorService    *service.ModeratorService
	SubscriptionService *service.SubscriptionService
}

// NewDeps creates the shared dependency container.
func NewDeps(cfg *config.Config, db *config.DBPair) *Deps {
	return &Deps{
		Cfg: cfg,
		DB:  db,
	}
}

// WireServices copies all service instances from a Services container into Deps.
func (d *Deps) WireServices(svcs *service.Services) {
	d.AuthService = svcs.Auth
	d.ProjectService = svcs.Project
	d.ParticipantService = svcs.Participant
	d.BookingService = svcs.Booking
	d.TranslationService = svcs.Translation
	d.AdminService = svcs.Admin
	d.ConferenceService = svcs.Conference
	d.PaymentService = svcs.Payment
	d.NotificationService = svcs.Notification
	d.MediaService = svcs.Media
	d.UserService = svcs.User
	d.InterviewService = svcs.Interview
	d.SurveyService = svcs.Survey
	d.ModeratorService = svcs.Moderator
	d.SubscriptionService = svcs.Subscription
}
