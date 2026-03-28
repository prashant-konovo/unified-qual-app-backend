package repository

import (
	"context"

	"github.com/InCrowd/unified-qual-api/internal/model"
)

// ProjectRepository defines the interface for project data access.
// Implementations exist for both IRIS and QS databases.
type ProjectRepository interface {
	List(ctx context.Context, page, pageSize int) ([]model.Project, int, error)
	GetByID(ctx context.Context, id string) (*model.Project, error)
	Create(ctx context.Context, p *model.Project) error
	Update(ctx context.Context, p *model.Project) error
	Delete(ctx context.Context, id string) error
}

// SurveyRepository defines the interface for survey data access.
type SurveyRepository interface {
	List(ctx context.Context, page, pageSize int) ([]model.Survey, int, error)
	GetByID(ctx context.Context, id string) (*model.Survey, error)
	Create(ctx context.Context, s *model.Survey) error
	Update(ctx context.Context, s *model.Survey) error
	Delete(ctx context.Context, id string) error
	UpdateStatus(ctx context.Context, id, status string) error
}

// TimeslotRepository defines the interface for timeslot data access.
type TimeslotRepository interface {
	List(ctx context.Context, page, pageSize int) ([]model.Timeslot, int, error)
	GetByID(ctx context.Context, id string) (*model.Timeslot, error)
	Create(ctx context.Context, t *model.Timeslot) error
	Update(ctx context.Context, t *model.Timeslot) error
	Delete(ctx context.Context, id string) error
	ListByModerator(ctx context.Context, moderatorID string) ([]model.Timeslot, error)
}

// ModeratorRepository defines the interface for moderator data access.
type ModeratorRepository interface {
	List(ctx context.Context, page, pageSize int) ([]model.Moderator, int, error)
	GetByID(ctx context.Context, id string) (*model.Moderator, error)
	Create(ctx context.Context, m *model.Moderator) error
	Update(ctx context.Context, m *model.Moderator) error
	Delete(ctx context.Context, id string) error
}

// ParticipantRepository defines the interface for participant data access.
type ParticipantRepository interface {
	List(ctx context.Context, page, pageSize int) ([]model.Participant, int, error)
	GetByID(ctx context.Context, id string) (*model.Participant, error)
	Create(ctx context.Context, p *model.Participant) error
	Update(ctx context.Context, p *model.Participant) error
}

// BookingRepository defines the interface for booking data access.
type BookingRepository interface {
	List(ctx context.Context, page, pageSize int) ([]model.Booking, int, error)
	GetByID(ctx context.Context, id string) (*model.Booking, error)
	GetByUserID(ctx context.Context, userID string) ([]model.Booking, error)
	Create(ctx context.Context, b *model.Booking) error
	Update(ctx context.Context, b *model.Booking) error
	Delete(ctx context.Context, id string) error
}
