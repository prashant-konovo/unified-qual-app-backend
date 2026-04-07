package service

import (
	"context"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
)

// BookingService encapsulates booking/timeslot business logic.
type BookingService struct {
	timeSlotRepo qs.TimeSlotRepository
}

// NewBookingService creates a new BookingService.
func NewBookingService(timeSlotRepo qs.TimeSlotRepository) *BookingService {
	return &BookingService{timeSlotRepo: timeSlotRepo}
}

// Available returns true if the underlying timeslot repository is configured.
func (s *BookingService) Available() bool { return s.timeSlotRepo != nil }

func (s *BookingService) ListTimeSlots(ctx context.Context, page, pageSize int, projectID *int64, statusID *int, moderatorID *int64, from *time.Time, to *time.Time) ([]qs.TimeSlotListRow, int, error) {
	return s.timeSlotRepo.List(ctx, page, pageSize, projectID, statusID, moderatorID, from, to)
}

func (s *BookingService) UpdateTimeSlot(ctx context.Context, id int64, fields map[string]any) error {
	return s.timeSlotRepo.Update(ctx, id, fields)
}

func (s *BookingService) UpsertReward(ctx context.Context, timeSlotID int64, points int, status string) error {
	return s.timeSlotRepo.UpsertReward(ctx, timeSlotID, points, status)
}
