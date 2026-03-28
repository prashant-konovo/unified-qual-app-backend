package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/InCrowd/unified-qual-api/internal/handler"
	"github.com/InCrowd/unified-qual-api/internal/router"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	h := handler.New()
	r := router.New(h)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("unified-qual-api starting on %s (env=%s)", addr, os.Getenv("ENVIRONMENT"))
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
