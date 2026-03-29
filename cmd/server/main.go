package main

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/InCrowd/unified-qual-api/internal/config"
	"github.com/InCrowd/unified-qual-api/internal/handler"
	"github.com/InCrowd/unified-qual-api/internal/logger"
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

	// Repository layer
	var irisProjectRepo *iris.ProjectRepo
	if db.IRIS != nil {
		irisProjectRepo = iris.NewProjectRepo(db.IRIS, db.IRISReadOnly)
	}
	var qsProjectRepo *qs.ProjectRepo
	if db.QS != nil {
		qsProjectRepo = qs.NewProjectRepo(db.QS)
	}

	h := handler.New(cfg, db, irisProjectRepo, qsProjectRepo)
	r := router.New(h, jwtAuth)

	addr := fmt.Sprintf(":%s", cfg.Port)
	slog.Info("server starting", "addr", addr, "env", cfg.Environment, "dummy", cfg.IsDummy())
	if err := http.ListenAndServe(addr, r); err != nil {
		slog.Error("server error", "error", err)
	}
}
