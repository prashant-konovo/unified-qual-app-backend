package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/dto"
	"github.com/InCrowd/unified-qual-api/internal/integration"
	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
	"github.com/InCrowd/unified-qual-api/internal/utilities"
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
	s3              *integration.S3Client
}

// NewProjectService creates a new ProjectService.
func NewProjectService(irisProjectRepo iris.ProjectRepository, qsProjectRepo qs.ProjectRepository, s3 *integration.S3Client) *ProjectService {
	return &ProjectService{
		irisProjectRepo: irisProjectRepo,
		qsProjectRepo:   qsProjectRepo,
		s3:              s3,
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
			Description:         utilities.ToNullStr(req.Description),
			SubscriptionID:      req.SubscriptionID,
			SalesforceProjectID: utilities.ToNullStr(req.SalesforceProjectID),
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
			SalesforceJobNumber: utilities.ToNullStr(req.SalesforceJobNumber),
			SampleSize:          utilities.ToNullInt64(req.SampleSize),
			InterviewLength:     utilities.ToNullInt64(req.InterviewLength),
			ClientID:            utilities.ToNullInt64(req.ClientID),
			PostScreeninBuffer:  utilities.ToNullStr(fmt.Sprintf("%.2f", req.PostScreeninBuffer)),
			ModeratorBuffer:     utilities.ToNullStr(fmt.Sprintf("%.2f", req.ModeratorBuffer)),
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
func (s *ProjectService) UpdateProject(ctx context.Context, projectID int64, req dto.UpdateProjectRequest) (*utilities.MutationResult, error) {
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
		return &utilities.MutationResult{ID: projectID, Updated: true, Source: "iris"}, nil
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
		return &utilities.MutationResult{ID: projectID, Updated: true, Source: "qs"}, nil
	}

	return nil, fmt.Errorf("no database available")
}

// ── Delete (soft) ───────────────────────────────────────────────────────────

// DeleteProject soft-deletes a project (QS: status=5 Canceled, IRIS: is_archived=true).
func (s *ProjectService) DeleteProject(ctx context.Context, projectID int64, source string) (*utilities.MutationResult, error) {
	if source == "qs" && s.qsProjectRepo != nil {
		if err := s.qsProjectRepo.Update(ctx, projectID, map[string]any{"project_status_id": 5}); err != nil {
			return nil, fmt.Errorf("qs project archive: %w", err)
		}
		return &utilities.MutationResult{ID: projectID, Archived: true, Source: "qs"}, nil
	}

	if s.irisProjectRepo != nil {
		if err := s.irisProjectRepo.Update(ctx, projectID, map[string]any{"is_archived": true}); err != nil {
			return nil, fmt.Errorf("iris project archive: %w", err)
		}
		return &utilities.MutationResult{ID: projectID, Archived: true, Source: "iris"}, nil
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

// ── QS Project passthrough ─────────────────────────────────────────────────

func (s *ProjectService) CreateProjectFull(ctx context.Context, req map[string]any) (map[string]any, error) {
	return s.qsProjectRepo.CreateProjectFull(ctx, req)
}

func (s *ProjectService) GetSalesForceJobNumberText(ctx context.Context, sfProjectID string) (string, error) {
	return s.qsProjectRepo.GetSalesForceJobNumberText(ctx, sfProjectID)
}

func (s *ProjectService) GetProjectDetailsMRA(ctx context.Context, projectID int64) (map[string]any, error) {
	return s.qsProjectRepo.GetProjectDetailsMRA(ctx, projectID)
}

func (s *ProjectService) GetProjectsMRA(ctx context.Context, creatorID, status int, sort, search string, externalClientIDs []string) ([]map[string]any, error) {
	return s.qsProjectRepo.GetProjectsMRA(ctx, creatorID, status, sort, search, externalClientIDs)
}

func (s *ProjectService) GetProjectsForModsMRA(ctx context.Context, clientID int64) ([]map[string]any, error) {
	return s.qsProjectRepo.GetProjectsForModsMRA(ctx, clientID)
}

func (s *ProjectService) SaveUserSelection(ctx context.Context, userID int64, accountIDs, clientIDs []string) error {
	return s.qsProjectRepo.SaveUserSelection(ctx, userID, accountIDs, clientIDs)
}

func (s *ProjectService) UpdateSchedulerGenerated(ctx context.Context, projectID int64) error {
	return s.qsProjectRepo.UpdateSchedulerGenerated(ctx, projectID)
}

func (s *ProjectService) UpdatePostScreenInBuffer(ctx context.Context, projectID int64, buffer float64) error {
	return s.qsProjectRepo.UpdatePostScreenInBuffer(ctx, projectID, buffer)
}

func (s *ProjectService) UpdateModeratorBufferMRA(ctx context.Context, projectID int64, buffer float64) error {
	return s.qsProjectRepo.UpdateModeratorBufferMRA(ctx, projectID, buffer)
}

func (s *ProjectService) UpdateExternalSurveyID(ctx context.Context, projectID int64, surveyID string) error {
	return s.qsProjectRepo.UpdateExternalSurveyID(ctx, projectID, surveyID)
}

func (s *ProjectService) GetProjectModeratorIDs(ctx context.Context, projectID int64) ([]int64, error) {
	return s.qsProjectRepo.GetProjectModeratorIDs(ctx, projectID)
}

func (s *ProjectService) ResetProjectModeratorsMRA(ctx context.Context, projectID int64, newIDs, existingIDs []int64) error {
	return s.qsProjectRepo.ResetProjectModeratorsMRA(ctx, projectID, newIDs, existingIDs)
}

func (s *ProjectService) UnassignModeratorFromProject(ctx context.Context, userID, projectID int64) error {
	return s.qsProjectRepo.UnassignModeratorFromProject(ctx, userID, projectID)
}

func (s *ProjectService) GetModeratorsList(ctx context.Context, projectID int64) ([]map[string]any, error) {
	return s.qsProjectRepo.GetModeratorsList(ctx, projectID)
}

func (s *ProjectService) GetEmailTemplateMRA(ctx context.Context, typeID int, language string) (map[string]any, error) {
	return s.qsProjectRepo.GetEmailTemplateMRA(ctx, typeID, language)
}

func (s *ProjectService) HandleProjectExportMRA(ctx context.Context, projectID int64, pmTimeZone, pmTimeZoneAbbr, rescheduleLinkPrefix string) ([][]string, error) {
	return s.qsProjectRepo.HandleProjectExportMRA(ctx, projectID, pmTimeZone, pmTimeZoneAbbr, rescheduleLinkPrefix)
}

func (s *ProjectService) HandleProjectNoTimeslotExportMRA(ctx context.Context, projectID int64, pmTimeZone, pmTimeZoneAbbr string) ([][]string, error) {
	return s.qsProjectRepo.HandleProjectNoTimeslotExportMRA(ctx, projectID, pmTimeZone, pmTimeZoneAbbr)
}

func (s *ProjectService) GetProjectName(ctx context.Context, projectID int64) (string, error) {
	return s.qsProjectRepo.GetProjectName(ctx, projectID)
}

func (s *ProjectService) UpdateQsProject(ctx context.Context, id int64, fields map[string]any) error {
	return s.qsProjectRepo.Update(ctx, id, fields)
}

func (s *ProjectService) UpdateSampleSizeMRA(ctx context.Context, projectID int64, sampleSize int64) error {
	return s.qsProjectRepo.UpdateSampleSizeMRA(ctx, projectID, sampleSize)
}

func (s *ProjectService) UpdateSampleSizeProjectStatusMRA(ctx context.Context, projectID int64, sampleSize int64, projectStatusID int64) error {
	return s.qsProjectRepo.UpdateSampleSizeProjectStatusMRA(ctx, projectID, sampleSize, projectStatusID)
}

func (s *ProjectService) GetModeratorsTimeRangePerProject(ctx context.Context, projectID int64) ([]qs.ModeratorTimeRange, error) {
	return s.qsProjectRepo.GetModeratorsTimeRangePerProject(ctx, projectID)
}

func (s *ProjectService) GetAllModeratorsAvailabilityPerClient(ctx context.Context, clientID int64, projectID int64) ([]qs.ModeratorAvailability, error) {
	return s.qsProjectRepo.GetAllModeratorsAvailabilityPerClient(ctx, clientID, projectID)
}

func (s *ProjectService) GetProjectStatusByID(ctx context.Context, projectID int64) (int64, error) {
	return s.qsProjectRepo.GetProjectStatusByID(ctx, projectID)
}

func (s *ProjectService) UpsertModeratorTimeRangePerProject(ctx context.Context, projectID, moderatorID int64, startTime, endTime, timezone string) error {
	return s.qsProjectRepo.UpsertModeratorTimeRangePerProject(ctx, projectID, moderatorID, startTime, endTime, timezone)
}

func (s *ProjectService) GetAllModeratorsAvailabilityPerRole(ctx context.Context, projectID int64) ([]qs.ModeratorAvailability, error) {
	return s.qsProjectRepo.GetAllModeratorsAvailabilityPerRole(ctx, projectID)
}

func (s *ProjectService) GetModeratorsTimeRangePerProjectMRA(ctx context.Context, projectID int64) ([]map[string]any, error) {
	return s.qsProjectRepo.GetModeratorsTimeRangePerProjectMRA(ctx, projectID)
}

func (s *ProjectService) GetProjectsForModeratorMRA(ctx context.Context, clientID, moderatorID int64) ([]map[string]any, error) {
	return s.qsProjectRepo.GetProjectsForModeratorMRA(ctx, clientID, moderatorID)
}

func (s *ProjectService) GetAllAccountsMRA(ctx context.Context) ([]map[string]any, error) {
	return s.qsProjectRepo.GetAllAccountsMRA(ctx)
}

func (s *ProjectService) GetSalesforceClientsMRA(ctx context.Context) ([]map[string]any, error) {
	return s.qsProjectRepo.GetSalesforceClientsMRA(ctx)
}

func (s *ProjectService) GetSalesforceProjectsMRA(ctx context.Context, salesforceClientID string) ([]map[string]any, error) {
	return s.qsProjectRepo.GetSalesforceProjectsMRA(ctx, salesforceClientID)
}

func (s *ProjectService) GetSalesforceClientsWithFilterMRA(ctx context.Context, projectAccountID int) ([]map[string]any, error) {
	return s.qsProjectRepo.GetSalesforceClientsWithFilterMRA(ctx, projectAccountID)
}

func (s *ProjectService) QsGetByID(ctx context.Context, id int64) (*qs.Project, error) {
	if s.qsProjectRepo == nil {
		return nil, nil
	}
	return s.qsProjectRepo.GetByID(ctx, id)
}

func (s *ProjectService) TimeSlotCounts(ctx context.Context, projectID int64) (scheduled, completed int, err error) {
	return s.qsProjectRepo.TimeSlotCounts(ctx, projectID)
}

func (s *ProjectService) GetTopics(ctx context.Context, projectID int64) ([]qs.Topic, error) {
	return s.qsProjectRepo.GetTopics(ctx, projectID)
}

// ── IRIS Project passthrough ────────────────────────────────────────────────

func (s *ProjectService) IrisGetByID(ctx context.Context, id int64) (*iris.Project, error) {
	if s.irisProjectRepo == nil {
		return nil, nil
	}
	return s.irisProjectRepo.GetByID(ctx, id)
}

// QsProjectAvailable reports whether the QS project repository is configured.
func (s *ProjectService) QsProjectAvailable() bool {
	return s.qsProjectRepo != nil
}

// --- S3 delegation methods ---

func (s *ProjectService) S3ExportConfigured() bool {
	return s.s3 != nil && s.s3.ExportBucket() != ""
}

func (s *ProjectService) ExportBucket() string {
	return s.s3.ExportBucket()
}

func (s *ProjectService) UploadFileToS3(ctx context.Context, bucket, key string, body io.Reader, contentType string) (string, error) {
	return s.s3.UploadFile(ctx, bucket, key, body, contentType)
}

func (s *ProjectService) GetS3PresignedURL(ctx context.Context, bucket, key string, expires time.Duration) (string, error) {
	return s.s3.GetPresignedURL(ctx, bucket, key, expires)
}
