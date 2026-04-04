package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/InCrowd/unified-qual-api/internal/dto"
	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
)

// irisStatusName maps IRIS project_status_id to human-readable name.
var irisStatusName = map[int]string{
	1: "Inquiry", 2: "Defining", 3: "In Progress", 4: "Complete", 6: "Paused", 7: "Finalizing",
}

// ProjectService encapsulates project business logic,
// including dual-DB operations (IRIS + QS).
type ProjectService struct {
	irisProjectRepo iris.ProjectRepository
	qsProjectRepo   qs.ProjectRepository
}

// NewProjectService creates a new ProjectService.
func NewProjectService(irisProjectRepo iris.ProjectRepository, qsProjectRepo qs.ProjectRepository) *ProjectService {
	return &ProjectService{
		irisProjectRepo: irisProjectRepo,
		qsProjectRepo:   qsProjectRepo,
	}
}

// ── Result types ────────────────────────────────────────────────────────────

// ProjectListResult is the result of a project list query.
type ProjectListResult struct {
	Projects []map[string]any
	Total    int
}

// ProjectCreateResult is the result of creating a project.
type ProjectCreateResult struct {
	ID              int64
	Source          string
	ServiceCategory string
}

// ProjectGetResult is the result of fetching a single project.
type ProjectGetResult struct {
	Found   bool
	Project map[string]any
}

// ── List ────────────────────────────────────────────────────────────────────

// ListProjects queries IRIS and/or QS project repos based on source filter.
func (s *ProjectService) ListProjects(ctx context.Context, page, pageSize int, source, search string, statusID *int) *ProjectListResult {
	log := slog.With("service", "ProjectService.ListProjects")
	var result []map[string]any

	if (source == "" || source == "iris") && s.irisProjectRepo != nil {
		irisProjects, _, err := s.irisProjectRepo.List(ctx, page, pageSize, statusID, search)
		if err != nil {
			log.Error("iris project list failed", "error", err)
		} else {
			for _, p := range irisProjects {
				result = append(result, dto.ProjectFromIRISList(p))
			}
		}
	}

	if (source == "" || source == "qs") && s.qsProjectRepo != nil {
		qsProjects, _, err := s.qsProjectRepo.List(ctx, page, pageSize, statusID, search)
		if err != nil {
			log.Error("qs project list failed", "error", err)
		} else {
			for _, p := range qsProjects {
				result = append(result, dto.ProjectFromQSList(p))
			}
		}
	}

	if result == nil {
		result = []map[string]any{}
	}
	return &ProjectListResult{Projects: result, Total: len(result)}
}

// ── Get ─────────────────────────────────────────────────────────────────────

// GetProject fetches a project by ID from QS (preferred) or IRIS.
func (s *ProjectService) GetProject(ctx context.Context, projectID int64, source string) (*ProjectGetResult, error) {
	// Try QS first (or if source=qs)
	if (source == "" || source == "qs") && s.qsProjectRepo != nil {
		p, err := s.qsProjectRepo.GetByID(ctx, projectID)
		if err != nil {
			return nil, fmt.Errorf("qs project get: %w", err)
		}
		if p != nil {
			scheduled, completed, _ := s.qsProjectRepo.TimeSlotCounts(ctx, projectID)
			topics, _ := s.qsProjectRepo.GetTopics(ctx, projectID)
			topicNames := make([]string, 0, len(topics))
			for _, t := range topics {
				topicNames = append(topicNames, t.TopicName)
			}
			return &ProjectGetResult{
				Found:   true,
				Project: dto.ProjectDetailFromQS(p, scheduled, completed, topicNames),
			}, nil
		}
	}

	// Try IRIS (or if source=iris)
	if (source == "" || source == "iris") && s.irisProjectRepo != nil {
		p, err := s.irisProjectRepo.GetByID(ctx, projectID)
		if err != nil {
			return nil, fmt.Errorf("iris project get: %w", err)
		}
		if p != nil {
			return &ProjectGetResult{
				Found:   true,
				Project: dto.ProjectDetailFromIRIS(p, irisStatusName[p.ProjectStatusID]),
			}, nil
		}
	}

	return &ProjectGetResult{Found: false}, nil
}

// ── Create ──────────────────────────────────────────────────────────────────

// CreateProject creates a project in IRIS or QS based on the request source.
func (s *ProjectService) CreateProject(ctx context.Context, req dto.CreateProjectRequest) (*ProjectCreateResult, error) {
	if req.Source == "" {
		req.Source = "qs"
	}

	if req.Source == "iris" && s.irisProjectRepo != nil {
		p := &iris.Project{
			Name:                req.Name,
			Description:         dto.ToNullStr(req.Description),
			SubscriptionID:      req.SubscriptionID,
			SalesforceProjectID: dto.ToNullStr(req.SalesforceProjectID),
			ProjectStatusID:     2, // Defining
		}
		id, err := s.irisProjectRepo.Create(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("iris project create: %w", err)
		}
		return &ProjectCreateResult{ID: id, Source: "iris", ServiceCategory: "LS"}, nil
	}

	if s.qsProjectRepo != nil {
		p := &qs.Project{
			Name:                req.Name,
			SalesforceJobNumber: dto.ToNullStr(req.SalesforceJobNumber),
			SampleSize:          dto.ToNullInt64(req.SampleSize),
			InterviewLength:     dto.ToNullInt64(req.InterviewLength),
			ClientID:            dto.ToNullInt64(req.ClientID),
			PostScreeninBuffer:  dto.ToNullStr(fmt.Sprintf("%.2f", req.PostScreeninBuffer)),
			ModeratorBuffer:     dto.ToNullStr(fmt.Sprintf("%.2f", req.ModeratorBuffer)),
		}
		id, err := s.qsProjectRepo.Create(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("qs project create: %w", err)
		}
		return &ProjectCreateResult{ID: id, Source: "qs", ServiceCategory: "MRA"}, nil
	}

	return nil, fmt.Errorf("no database available")
}

// ── Update ──────────────────────────────────────────────────────────────────

// UpdateProject updates a project in IRIS or QS based on the request source.
func (s *ProjectService) UpdateProject(ctx context.Context, projectID int64, req dto.UpdateProjectRequest) (*dto.MutationResult, error) {
	fields := map[string]any{}
	if req.Name != "" {
		fields["name"] = req.Name
	}
	if req.StatusID != nil {
		fields["project_status_id"] = *req.StatusID
	}

	if req.Source == "iris" && s.irisProjectRepo != nil {
		if req.Description != "" {
			fields["description"] = req.Description
		}
		if req.SalesforceProjectID != "" {
			fields["salesforce_project_id"] = req.SalesforceProjectID
		}
		if req.IsArchived != nil {
			fields["is_archived"] = *req.IsArchived
		}
		if err := s.irisProjectRepo.Update(ctx, projectID, fields); err != nil {
			return nil, fmt.Errorf("iris project update: %w", err)
		}
		return &dto.MutationResult{ID: projectID, Updated: true, Source: "iris"}, nil
	}

	if s.qsProjectRepo != nil {
		if req.SampleSize != nil {
			fields["sample_size"] = *req.SampleSize
		}
		if req.InterviewLength != nil {
			fields["interview_length"] = *req.InterviewLength
		}
		if err := s.qsProjectRepo.Update(ctx, projectID, fields); err != nil {
			return nil, fmt.Errorf("qs project update: %w", err)
		}
		return &dto.MutationResult{ID: projectID, Updated: true, Source: "qs"}, nil
	}

	return nil, fmt.Errorf("no database available")
}

// ── Delete (soft) ───────────────────────────────────────────────────────────

// DeleteProject soft-deletes a project (QS: status=5 Canceled, IRIS: is_archived=true).
func (s *ProjectService) DeleteProject(ctx context.Context, projectID int64, source string) (*dto.MutationResult, error) {
	if source == "qs" && s.qsProjectRepo != nil {
		if err := s.qsProjectRepo.Update(ctx, projectID, map[string]any{"project_status_id": 5}); err != nil {
			return nil, fmt.Errorf("qs project archive: %w", err)
		}
		return &dto.MutationResult{ID: projectID, Archived: true, Source: "qs"}, nil
	}

	if s.irisProjectRepo != nil {
		if err := s.irisProjectRepo.Update(ctx, projectID, map[string]any{"is_archived": true}); err != nil {
			return nil, fmt.Errorf("iris project archive: %w", err)
		}
		return &dto.MutationResult{ID: projectID, Archived: true, Source: "iris"}, nil
	}

	return nil, fmt.Errorf("no database available")
}

// ResolveSource maps a serviceCategory string to a DB source.
func ResolveSource(source, serviceCategory string) string {
	if source != "" {
		return source
	}
	switch serviceCategory {
	case "LS":
		return "iris"
	case "MRA":
		return "qs"
	}
	return ""
}
