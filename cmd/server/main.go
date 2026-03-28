package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/InCrowd/unified-qual-api/internal/config"
	"github.com/InCrowd/unified-qual-api/internal/handler"
	"github.com/InCrowd/unified-qual-api/internal/router"
)

func main() {
	cfg := config.Load()

	db, err := config.ConnectDatabases(cfg)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer db.Close()

	h := handler.New(cfg, db)
	r := router.New(h)

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("unified-qual-api starting on %s (env=%s, dummy=%v)", addr, cfg.Environment, cfg.IsDummy())
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
