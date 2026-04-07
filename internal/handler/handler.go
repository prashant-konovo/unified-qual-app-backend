package handler

import (
	"github.com/InCrowd/unified-qual-api/internal/config"
	"github.com/InCrowd/unified-qual-api/internal/handler/ls"
	"github.com/InCrowd/unified-qual-api/internal/handler/mra"
	"github.com/InCrowd/unified-qual-api/internal/handler/shared"
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

// NewHandlers creates all domain handlers from the Services container.
func NewHandlers(cfg *config.Config, d *service.Services) *Handlers {
	d.Cfg = cfg

	return &Handlers{
		Auth:      &shared.AuthHandler{Services: d},
		Project:   &shared.ProjectHandler{Services: d},
		Survey:    &shared.SurveyHandler{Services: d},
		Interview: &shared.InterviewHandler{Services: d},
		LS:        &ls.Handler{Services: d},
		MRA:       &mra.Handler{Services: d},
		Handler:   &shared.Handler{Services: d},
	}
}
