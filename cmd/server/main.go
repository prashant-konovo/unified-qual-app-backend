package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/InCrowd/unified-qual-api/internal/config"
	"github.com/InCrowd/unified-qual-api/internal/handler"
	"github.com/InCrowd/unified-qual-api/internal/middleware"
	"github.com/InCrowd/unified-qual-api/internal/router"
)

func main() {
	cfg := config.Load()

	db := config.ConnectDatabases(cfg)
	defer db.Close()

	jwtAuth := middleware.NewJWTAuth(
		cfg.Cognito.Region,
		cfg.Cognito.UserPoolID,
		cfg.Cognito.AppClientID,
	)

	h := handler.New(cfg, db)
	r := router.New(h, jwtAuth)

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("unified-qual-api starting on %s (env=%s, dummy=%v)", addr, cfg.Environment, cfg.IsDummy())
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
