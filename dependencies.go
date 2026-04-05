package qualapi

import (
	"github.com/InCrowd/unified-qual-api/internal/config"
	"github.com/InCrowd/unified-qual-api/internal/integration"
	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
	"github.com/InCrowd/unified-qual-api/internal/service"
)

// Deps holds shared dependencies injected into all domain handlers.
// Repository fields use interfaces for testability (mock injection).
// Fields are exported so subpackages (ls/, mra/, shared/) can access them.
type Deps struct {
	Cfg              *config.Config
	DB               *config.DBPair
	Services         *integration.ServiceClients
	IrisProjectRepo  iris.ProjectRepository
	QsProjectRepo   qs.ProjectRepository
	IrisUserRepo    iris.UserRepository
	QsUserRepo      qs.UserRepository
	QsTimeSlotRepo  qs.TimeSlotRepository
	QsRespondentRepo qs.RespondentRepository
	QsSurveyRepo    qs.SurveyRepository
	IrisSurveyRepo   iris.SurveyRepository
	QsConferenceRepo qs.ConferenceRepository
	QsAnswerRepo     qs.AnswerRepository
	QsInterviewsRepo qs.InterviewsRepository
	AuthService      *service.AuthService
	ProjectService   *service.ProjectService
}

// NewDeps creates the shared dependency container.
func NewDeps(
	cfg *config.Config,
	db *config.DBPair,
	services *integration.ServiceClients,
	irisRepo iris.ProjectRepository,
	qsRepo qs.ProjectRepository,
	irisUserRepo iris.UserRepository,
	qsUserRepo qs.UserRepository,
	qsTimeSlotRepo qs.TimeSlotRepository,
	qsRespondentRepo qs.RespondentRepository,
	qsSurveyRepo qs.SurveyRepository,
	irisSurveyRepo iris.SurveyRepository,
	qsConferenceRepo qs.ConferenceRepository,
	qsAnswerRepo qs.AnswerRepository,
) *Deps {
	d := &Deps{
		Cfg:              cfg,
		DB:               db,
		Services:         services,
		IrisProjectRepo:  irisRepo,
		QsProjectRepo:   qsRepo,
		IrisUserRepo:    irisUserRepo,
		QsUserRepo:      qsUserRepo,
		QsTimeSlotRepo:  qsTimeSlotRepo,
		QsRespondentRepo: qsRespondentRepo,
		QsSurveyRepo:    qsSurveyRepo,
		IrisSurveyRepo:   irisSurveyRepo,
		QsConferenceRepo: qsConferenceRepo,
		QsAnswerRepo:     qsAnswerRepo,
	}
	if db.QS != nil {
		d.QsInterviewsRepo = qs.NewInterviewsRepo(db.QS)
	}
	return d
}
