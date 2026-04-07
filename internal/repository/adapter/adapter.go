// Package adapter provides brand-aware repository adapters that route
// calls to the correct underlying repository (IRIS for LS, QS for MRA)
// based on a Brand parameter.
//
// # Usage Pattern
//
// Each adapter wraps both the IRIS and QS repository interfaces for a
// given domain (e.g., Project, User, Moderator). Methods accept a Brand
// parameter and delegate to the appropriate repo.
//
// Example (from service layer — NOT wired yet, for future devs):
//
//	type ProjectService struct {
//	    projects *adapter.ProjectAdapter
//	}
//
//	func (s *ProjectService) List(ctx context.Context, brand adapter.Brand, ...) {
//	    return s.projects.List(ctx, brand, page, pageSize, statusID, search)
//	}
//
// # Adding a New Adapter
//
//  1. Create a new file: `<domain>_adapter.go`
//  2. Define a struct holding the IRIS and QS repo interfaces
//  3. Add a constructor: `New<Domain>Adapter(irisRepo, qsRepo)`
//  4. Add methods that accept Brand and route to the correct repo
//  5. Register the adapter in the Adapters container below
package adapter

import (
	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
)

// Adapters holds all brand-aware repository adapter instances.
// Add new adapters here as they are created.
type Adapters struct {
	Project *ProjectAdapter
	// User     *UserAdapter       // TODO: add when needed
	// Moderator *ModeratorAdapter // TODO: add when needed
	// TimeSlot *TimeSlotAdapter   // TODO: add when needed
	// Survey   *SurveyAdapter     // TODO: add when needed
}

// NewAdapters creates all adapter instances from the IRIS and QS repository containers.
func NewAdapters(irisRepos *iris.Repositories, qsRepos *qs.Repositories) *Adapters {
	return &Adapters{
		Project: NewProjectAdapter(irisRepos.Project, qsRepos.Project),
	}
}
