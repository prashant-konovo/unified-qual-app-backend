package handler

import (
	"github.com/InCrowd/unified-qual-api/internal/config"
	"github.com/InCrowd/unified-qual-api/internal/integration"
	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
)

// Deps holds shared dependencies injected into all domain handlers.
// Repository fields use interfaces for testability (mock injection).
type Deps struct {
	cfg              *config.Config
	db               *config.DBPair
	services         *integration.ServiceClients
	irisProjectRepo  iris.ProjectRepository
	qsProjectRepo   qs.ProjectRepository
	irisUserRepo    iris.UserRepository
	qsUserRepo      qs.UserRepository
	qsTimeSlotRepo  qs.TimeSlotRepository
	qsRespondentRepo qs.RespondentRepository
	qsSurveyRepo    qs.SurveyRepository
	irisSurveyRepo   iris.SurveyRepository
	qsConferenceRepo qs.ConferenceRepository
	qsAnswerRepo     qs.AnswerRepository
	qsInterviewsRepo qs.InterviewsRepository
}

// NewDeps creates the shared dependency container.
func NewDeps(cfg *config.Config, db *config.DBPair, services *integration.ServiceClients, irisRepo iris.ProjectRepository, qsRepo qs.ProjectRepository, irisUserRepo iris.UserRepository, qsUserRepo qs.UserRepository, qsTimeSlotRepo qs.TimeSlotRepository, qsRespondentRepo qs.RespondentRepository, qsSurveyRepo qs.SurveyRepository, irisSurveyRepo iris.SurveyRepository, qsConferenceRepo qs.ConferenceRepository, qsAnswerRepo qs.AnswerRepository) *Deps {
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

// LSHandler handles LS (Life Sciences / IRIS) brand-specific endpoints.
type LSHandler struct{ *Deps }

// MRAHandler handles MRA (Market Research & Analysis / QS) brand-specific endpoints.
type MRAHandler struct{ *Deps }

// Handlers groups all domain handlers for router registration.
type Handlers struct {
	Auth      *AuthHandler
	Project   *ProjectHandler
	Survey    *SurveyHandler
	Interview *InterviewHandler
	LS        *LSHandler
	MRA       *MRAHandler
	Handler   *Handler // shared non-brand-specific handlers
}

// NewHandlers creates all domain handlers from shared deps.
func NewHandlers(d *Deps) *Handlers {
	return &Handlers{
		Auth:      &AuthHandler{d},
		Project:   &ProjectHandler{d},
		Survey:    &SurveyHandler{d},
		Interview: &InterviewHandler{d},
		LS:        &LSHandler{d},
		MRA:       &MRAHandler{d},
		Handler:   &Handler{d},
	}
}
