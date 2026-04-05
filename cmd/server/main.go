package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/InCrowd/unified-qual-api/internal/config"
	"github.com/InCrowd/unified-qual-api/internal/handler"
	"github.com/InCrowd/unified-qual-api/internal/integration"
	"github.com/InCrowd/unified-qual-api/internal/jobs"
	"github.com/InCrowd/unified-qual-api/internal/logger"
	"github.com/InCrowd/unified-qual-api/internal/middleware"
	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
	"github.com/InCrowd/unified-qual-api/internal/router"
	"github.com/InCrowd/unified-qual-api/internal/service"
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

	// Repository layer — QS
	var qsRepos *qs.Repositories
	if db.QS != nil {
		qsRepos = qs.NewRepositories(db.QS)
		if err := qsRepos.Survey.EnsureTable(context.Background()); err != nil {
			slog.Warn("failed to ensure survey table", "error", err)
		}
	}

	// Repository layer — IRIS
	var irisRepos *iris.Repositories
	if db.IRIS != nil {
		irisRepos = iris.NewRepositories(db.IRIS, db.IRISReadOnly)
	}

	// Service + handler layer
	svcClients := integration.NewServiceClients(cfg)
	svcs := service.NewServices(cfg, qsRepos, irisRepos, svcClients)

	hs := handler.NewHandlers(cfg, db, svcs)
	r := router.New(hs, jwtAuth)

	// Start scheduled jobs (if enabled)
	if cfg.JobsEnabled && irisRepos != nil {
		jobDeps := &jobs.JobDeps{
			Cfg: cfg, DB: db,
			Services:        svcClients,
			IrisSurveyRepo:  irisRepos.Survey,
			IrisProjectRepo: irisRepos.Project,
			IrisUserRepo:    irisRepos.User,
			JobsRepo:        irisRepos.Jobs,
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
