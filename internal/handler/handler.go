package handler

import (
	"github.com/InCrowd/unified-qual-api/internal/config"
	"github.com/InCrowd/unified-qual-api/internal/handler/ls"
	"github.com/InCrowd/unified-qual-api/internal/handler/mra"
	"github.com/InCrowd/unified-qual-api/internal/handler/shared"
	"github.com/InCrowd/unified-qual-api/internal/handler/support"
	"github.com/InCrowd/unified-qual-api/internal/service"
)

// Handlers groups all domain handlers for router registration.
type Handlers struct {
	Auth      *shared.AuthHandler
	Project   *shared.ProjectHandler
	Survey    *shared.SurveyHandler
	Interview *shared.InterviewHandler
	LS        *ls.Handler
	MRA       *mra.Handler
	Handler   *shared.Handler // shared non-brand-specific handlers
}

// NewHandlers creates all domain handlers by wiring config, DB, and services
// into the shared Deps container.
func NewHandlers(cfg *config.Config, db *config.DBPair, svcs *service.Services) *Handlers {
	d := &support.Deps{
		Cfg:                 cfg,
		DB:                  db,
		AuthService:         svcs.Auth,
		ProjectService:      svcs.Project,
		ParticipantService:  svcs.Participant,
		BookingService:      svcs.Booking,
		TranslationService:  svcs.Translation,
		AdminService:        svcs.Admin,
		ConferenceService:   svcs.Conference,
		PaymentService:      svcs.Payment,
		NotificationService: svcs.Notification,
		MediaService:        svcs.Media,
		UserService:         svcs.User,
		InterviewService:    svcs.Interview,
		SurveyService:       svcs.Survey,
		ModeratorService:    svcs.Moderator,
		SubscriptionService: svcs.Subscription,
	}
	return &Handlers{
		Auth:      &shared.AuthHandler{Deps: d},
		Project:   &shared.ProjectHandler{Deps: d},
		Survey:    &shared.SurveyHandler{Deps: d},
		Interview: &shared.InterviewHandler{Deps: d},
		LS:        &ls.Handler{Deps: d},
		MRA:       &mra.Handler{Deps: d},
		Handler:   &shared.Handler{Deps: d},
	}
}
