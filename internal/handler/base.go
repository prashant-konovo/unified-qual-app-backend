package handler

import (
	"github.com/InCrowd/unified-qual-api/internal/config"
	"github.com/InCrowd/unified-qual-api/internal/integration"
	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
)

// Deps holds shared dependencies injected into all domain handlers.
type Deps struct {
	cfg              *config.Config
	db               *config.DBPair
	services         *integration.ServiceClients
	irisProjectRepo  *iris.ProjectRepo
	qsProjectRepo   *qs.ProjectRepo
	irisUserRepo    *iris.UserRepo
	qsUserRepo      *qs.UserRepo
	qsTimeSlotRepo  *qs.TimeSlotRepo
	qsRespondentRepo *qs.RespondentRepo
	qsSurveyRepo    *qs.SurveyRepo
	irisSurveyRepo   *iris.SurveyRepo
	qsConferenceRepo *qs.ConferenceRepo
	qsAnswerRepo     *qs.AnswerRepo
	qsInterviewsRepo *qs.InterviewsRepo
}

// NewDeps creates the shared dependency container.
func NewDeps(cfg *config.Config, db *config.DBPair, services *integration.ServiceClients, irisRepo *iris.ProjectRepo, qsRepo *qs.ProjectRepo, irisUserRepo *iris.UserRepo, qsUserRepo *qs.UserRepo, qsTimeSlotRepo *qs.TimeSlotRepo, qsRespondentRepo *qs.RespondentRepo, qsSurveyRepo *qs.SurveyRepo, irisSurveyRepo *iris.SurveyRepo, qsConferenceRepo *qs.ConferenceRepo, qsAnswerRepo *qs.AnswerRepo) *Deps {
	d := &Deps{
		cfg:              cfg,
		db:               db,
		services:         services,
		irisProjectRepo:  irisRepo,
		qsProjectRepo:   qsRepo,
		irisUserRepo:    irisUserRepo,
		qsUserRepo:      qsUserRepo,
		qsTimeSlotRepo:  qsTimeSlotRepo,
		qsRespondentRepo: qsRespondentRepo,
		qsSurveyRepo:    qsSurveyRepo,
		irisSurveyRepo:   irisSurveyRepo,
		qsConferenceRepo: qsConferenceRepo,
		qsAnswerRepo:     qsAnswerRepo,
	}
	if db.QS != nil {
		d.qsInterviewsRepo = qs.NewInterviewsRepo(db.QS)
	}
	return d
}

// Handlers groups all domain handlers for router registration.
type Handlers struct {
	Auth      *AuthHandler
	Project   *ProjectHandler
	Survey    *SurveyHandler
	Interview *InterviewHandler
	Handler   *Handler // remaining non-domain-specific handlers
}

// NewHandlers creates all domain handlers from shared deps.
func NewHandlers(d *Deps) *Handlers {
	return &Handlers{
		Auth:      &AuthHandler{d},
		Project:   &ProjectHandler{d},
		Survey:    &SurveyHandler{d},
		Interview: &InterviewHandler{d},
		Handler:   &Handler{d},
	}
}
