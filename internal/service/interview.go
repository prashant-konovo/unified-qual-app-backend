package service

import (
	"context"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
)

// InterviewService encapsulates interview/timeslot scheduling logic.
type InterviewService struct {
	timeSlotRepo   qs.TimeSlotRepository
	interviewsRepo qs.InterviewsRepository
}

// NewInterviewService creates a new InterviewService.
func NewInterviewService(timeSlotRepo qs.TimeSlotRepository, interviewsRepo qs.InterviewsRepository) *InterviewService {
	return &InterviewService{timeSlotRepo: timeSlotRepo, interviewsRepo: interviewsRepo}
}

// TimeSlotAvailable returns true if the QS timeslot repository is configured.
func (s *InterviewService) TimeSlotAvailable() bool { return s.timeSlotRepo != nil }

// InterviewsAvailable returns true if the QS interviews repository is configured.
func (s *InterviewService) InterviewsAvailable() bool { return s.interviewsRepo != nil }

// --- QsTimeSlotRepo methods ---

func (s *InterviewService) ListTimeSlots(ctx context.Context, page, pageSize int, projectID *int64, statusID *int, moderatorID *int64, from, to *time.Time) ([]qs.TimeSlotListRow, int, error) {
	return s.timeSlotRepo.List(ctx, page, pageSize, projectID, statusID, moderatorID, from, to)
}

func (s *InterviewService) GetByID(ctx context.Context, id int64) (*qs.TimeSlot, error) {
	return s.timeSlotRepo.GetByID(ctx, id)
}

func (s *InterviewService) Create(ctx context.Context, ts *qs.TimeSlot) (int64, error) {
	return s.timeSlotRepo.Create(ctx, ts)
}

func (s *InterviewService) UpdateTimeSlot(ctx context.Context, id int64, fields map[string]any) error {
	return s.timeSlotRepo.Update(ctx, id, fields)
}

func (s *InterviewService) DeleteTimeSlot(ctx context.Context, id int64) error {
	return s.timeSlotRepo.Delete(ctx, id)
}

func (s *InterviewService) GetModerators(ctx context.Context, timeSlotID int64) ([]qs.ModeratorTimeSlot, error) {
	return s.timeSlotRepo.GetModerators(ctx, timeSlotID)
}

func (s *InterviewService) GetRespondent(ctx context.Context, timeSlotID int64) (*qs.Respondent, error) {
	return s.timeSlotRepo.GetRespondent(ctx, timeSlotID)
}

func (s *InterviewService) AssignModerator(ctx context.Context, moderatorID, timeSlotID int64, isHost bool) (int64, error) {
	return s.timeSlotRepo.AssignModerator(ctx, moderatorID, timeSlotID, isHost)
}

func (s *InterviewService) ListStatuses(ctx context.Context) ([]qs.TimeSlotStatus, error) {
	return s.timeSlotRepo.ListStatuses(ctx)
}

func (s *InterviewService) ListByProject(ctx context.Context, projectID int64, page, pageSize int) ([]qs.TimeSlotListRow, int, error) {
	return s.timeSlotRepo.ListByProject(ctx, projectID, page, pageSize)
}

func (s *InterviewService) HasCompletedPaymentMRA(ctx context.Context, timeSlotID int64) (bool, error) {
	return s.timeSlotRepo.HasCompletedPaymentMRA(ctx, timeSlotID)
}

func (s *InterviewService) InvalidateInterviewMRA(ctx context.Context, timeSlotID int64, reasonCode string, reasonText *string, invalidatedByUserID int64) error {
	return s.timeSlotRepo.InvalidateInterviewMRA(ctx, timeSlotID, reasonCode, reasonText, invalidatedByUserID)
}

// --- QsInterviewsRepo methods ---

func (s *InterviewService) GetAllInterviewsByOffsetAndActiveTab(ctx context.Context, clientID int64, externalClientsIDs, projectsIDs []string, search string, offset int, activeTab, paymentStatusCode string) ([]map[string]any, error) {
	return s.interviewsRepo.GetAllInterviewsByOffsetAndActiveTab(ctx, clientID, externalClientsIDs, projectsIDs, search, offset, activeTab, paymentStatusCode)
}
