package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/InCrowd/unified-qual-api/internal/config"
	"github.com/InCrowd/unified-qual-api/internal/handler"
	"github.com/InCrowd/unified-qual-api/internal/handler/support"
	"github.com/InCrowd/unified-qual-api/internal/integration"
	"github.com/InCrowd/unified-qual-api/internal/jobs"
	"github.com/InCrowd/unified-qual-api/internal/logger"
	"github.com/InCrowd/unified-qual-api/internal/service"
	"github.com/InCrowd/unified-qual-api/internal/middleware"
	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
	"github.com/InCrowd/unified-qual-api/internal/router"
)

func main() {
	cfg := config.Load()
	logger.Setup(cfg.LogLevel)

	db := config.ConnectDatabases(cfg)
	defer db.Close()

	jwtAuth := middleware.NewJWTAuth(
		cfg.Cognito.Region,
		cfg.Cognito.UserPoolID,
		cfg.Cognito.AllClientIDs,
	)

	// Repository layer — IRIS (read-write + read-only replica)
	var (
		irisProjectRepo iris.ProjectRepository
		irisUserRepo    iris.UserRepository
		irisSurveyRepo  iris.SurveyRepository
	)
	if db.IRIS != nil {
		irisProjectRepo = iris.NewProjectRepo(db.IRIS, db.IRISReadOnly)
		irisUserRepo = iris.NewUserRepo(db.IRIS, db.IRISReadOnly)
		irisSurveyRepo = iris.NewSurveyRepo(db.IRIS, db.IRISReadOnly)
	}

	// Repository layer — QS (single connection pool)
	var (
		qsProjectRepo    qs.ProjectRepository
		qsUserRepo       qs.UserRepository
		qsTimeSlotRepo   qs.TimeSlotRepository
		qsRespondentRepo qs.RespondentRepository
		qsSurveyRepo     *qs.SurveyRepo // concrete for EnsureTable
		qsConferenceRepo qs.ConferenceRepository
		qsAnswerRepo     qs.AnswerRepository
	)
	if db.QS != nil {
		qsProjectRepo = qs.NewProjectRepo(db.QS)
		qsUserRepo = qs.NewUserRepo(db.QS)
		qsTimeSlotRepo = qs.NewTimeSlotRepo(db.QS)
		qsRespondentRepo = qs.NewRespondentRepo(db.QS)
		qsSurveyRepo = qs.NewSurveyRepo(db.QS)
		qsConferenceRepo = qs.NewConferenceRepo(db.QS)
		qsAnswerRepo = qs.NewAnswerRepo(db.QS)
		if err := qsSurveyRepo.EnsureTable(context.Background()); err != nil {
			slog.Warn("failed to ensure survey table", "error", err)
		}
	}

	// Wire handlers + router
	svcClients := integration.NewServiceClients(cfg)
	deps := support.NewDeps(cfg, db, svcClients,
		irisProjectRepo, qsProjectRepo,
		irisUserRepo, qsUserRepo,
		qsTimeSlotRepo, qsRespondentRepo,
		qsSurveyRepo, irisSurveyRepo,
		qsConferenceRepo, qsAnswerRepo,
	)
	deps.AuthService = service.NewAuthService(cfg, svcClients.ICAuth, qsUserRepo)
	deps.ProjectService = service.NewProjectService(irisProjectRepo, qsProjectRepo)
	hs := handler.NewHandlers(deps)
	r := router.New(hs, jwtAuth)

	// Start scheduled jobs (if enabled)
	if cfg.JobsEnabled {
		var jobsRepo iris.JobsRepository
		if db.IRIS != nil {
			jobsRepo = iris.NewJobsRepo(db.IRIS, db.IRISReadOnly)
		}
		jobDeps := &jobs.JobDeps{
			Cfg: cfg, DB: db,
			Services:        svcClients,
			IrisSurveyRepo:  irisSurveyRepo,
			IrisProjectRepo: irisProjectRepo,
			IrisUserRepo:    irisUserRepo,
			JobsRepo:        jobsRepo,
		}
		scheduler := jobs.NewScheduler(jobDeps, cfg.Jobs, slog.Default())
		if err := scheduler.Start(); err != nil {
			slog.Error("failed to start job scheduler", "error", err)
		} else {
			defer scheduler.Stop()
		}
	}

	addr := fmt.Sprintf(":%s", cfg.Port)
	slog.Info("server starting", "addr", addr, "env", cfg.Environment, "dummy", cfg.IsDummy())
	if err := http.ListenAndServe(addr, r); err != nil {
		slog.Error("server error", "error", err)
	}
}
