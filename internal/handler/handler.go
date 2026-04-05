package handler

import (
qualapi "github.com/InCrowd/unified-qual-api"
"github.com/InCrowd/unified-qual-api/internal/handler/ls"
"github.com/InCrowd/unified-qual-api/internal/handler/mra"
"github.com/InCrowd/unified-qual-api/internal/handler/shared"
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

// NewHandlers creates all domain handlers from shared deps.
func NewHandlers(d *qualapi.Deps) *Handlers {
return &Handlers{
Auth:      &shared.AuthHandler{Deps: d},
Project:   &shared.ProjectHandler{Deps: d},
Survey:    &shared.SurveyHandler{Deps: d},
Interview: &shared.InterviewHandler{Deps: d},
LS:        &ls.Handler{Deps: d},
MRA:       &mra.Handler{Deps: d},
Handler:   &shared.Handler{Deps: d},
}
}
