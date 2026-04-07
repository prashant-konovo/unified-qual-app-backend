package adapter

import (
	"context"
	"fmt"

	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
)

// ProjectAdapter routes project repository calls to the correct backend
// (IRIS for LS, QS for MRA) based on the Brand parameter.
//
// This is a boilerplate example — add more methods as needed following
// the same switch-on-brand pattern.
type ProjectAdapter struct {
	irisRepo iris.ProjectRepository
	qsRepo   qs.ProjectRepository
}

// NewProjectAdapter creates a ProjectAdapter with both brand repos.
func NewProjectAdapter(irisRepo iris.ProjectRepository, qsRepo qs.ProjectRepository) *ProjectAdapter {
	return &ProjectAdapter{
		irisRepo: irisRepo,
		qsRepo:   qsRepo,
	}
}

// ── Example adapter methods — extend as needed ──────────────────────────────

// ListResult is a brand-agnostic project list result.
type ListResult struct {
	Projects []map[string]any
	Total    int
}

// List retrieves paginated projects from the brand-appropriate repository.
func (a *ProjectAdapter) List(ctx context.Context, brand Brand, page, pageSize int, statusID *int, search string) (*ListResult, error) {
	switch brand {
	case BrandLS:
		rows, total, err := a.irisRepo.List(ctx, page, pageSize, statusID, search)
		if err != nil {
			return nil, fmt.Errorf("iris project list: %w", err)
		}
		projects := make([]map[string]any, len(rows))
		for i, r := range rows {
			projects[i] = map[string]any{
				"id":              r.ID,
				"name":            r.Name,
				"serviceCategory": "LS",
			}
		}
		return &ListResult{Projects: projects, Total: total}, nil

	case BrandMRA:
		rows, total, err := a.qsRepo.List(ctx, page, pageSize, statusID, search)
		if err != nil {
			return nil, fmt.Errorf("qs project list: %w", err)
		}
		projects := make([]map[string]any, len(rows))
		for i, r := range rows {
			projects[i] = map[string]any{
				"id":              r.ID,
				"name":            r.Name,
				"serviceCategory": "MRA",
			}
		}
		return &ListResult{Projects: projects, Total: total}, nil

	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedBrand, brand)
	}
}

// GetByID retrieves a single project by ID from the brand-appropriate repository.
func (a *ProjectAdapter) GetByID(ctx context.Context, brand Brand, id int64) (map[string]any, error) {
	switch brand {
	case BrandLS:
		p, err := a.irisRepo.GetByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("iris project get: %w", err)
		}
		if p == nil {
			return nil, nil
		}
		return map[string]any{
			"id":              p.ID,
			"name":            p.Name,
			"serviceCategory": "LS",
		}, nil

	case BrandMRA:
		p, err := a.qsRepo.GetByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("qs project get: %w", err)
		}
		if p == nil {
			return nil, nil
		}
		return map[string]any{
			"id":              p.ID,
			"name":            p.Name,
			"serviceCategory": "MRA",
		}, nil

	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedBrand, brand)
	}
}
