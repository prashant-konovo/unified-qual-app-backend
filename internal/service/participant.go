package service

import (
	"context"

	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
)

// ParticipantService encapsulates participant/respondent business logic.
type ParticipantService struct {
	respondentRepo qs.RespondentRepository
	timeSlotRepo   qs.TimeSlotRepository
}

// NewParticipantService creates a new ParticipantService.
func NewParticipantService(respondentRepo qs.RespondentRepository, timeSlotRepo qs.TimeSlotRepository) *ParticipantService {
	return &ParticipantService{respondentRepo: respondentRepo, timeSlotRepo: timeSlotRepo}
}

// Available returns true if the underlying respondent repository is configured.
func (s *ParticipantService) Available() bool { return s.respondentRepo != nil }

// TimeSlotAvailable returns true if the underlying timeslot repository is configured.
func (s *ParticipantService) TimeSlotAvailable() bool { return s.timeSlotRepo != nil }

func (s *ParticipantService) List(ctx context.Context, page, pageSize int, search string) ([]qs.RespondentListRow, int, error) {
	return s.respondentRepo.List(ctx, page, pageSize, search)
}

func (s *ParticipantService) Create(ctx context.Context, resp *qs.Respondent) (int64, error) {
	return s.respondentRepo.Create(ctx, resp)
}

func (s *ParticipantService) CreateCommunicationAddress(ctx context.Context, responderID int64, transportTypeID int, address string) (int64, error) {
	return s.respondentRepo.CreateCommunicationAddress(ctx, responderID, transportTypeID, address)
}

func (s *ParticipantService) GetByID(ctx context.Context, id int64) (*qs.Respondent, error) {
	return s.respondentRepo.GetByID(ctx, id)
}

func (s *ParticipantService) GetCommunicationAddresses(ctx context.Context, responderID int64) ([]qs.RespondentCommunicationAddress, error) {
	return s.respondentRepo.GetCommunicationAddresses(ctx, responderID)
}

func (s *ParticipantService) CreateRespondentMRA(ctx context.Context, firstName, lastName, title, extID, sessKey, tz, tzAbbr, lang string) (int64, error) {
	return s.respondentRepo.CreateRespondentMRA(ctx, firstName, lastName, title, extID, sessKey, tz, tzAbbr, lang)
}

func (s *ParticipantService) UpsertEligibilityStatusMRA(ctx context.Context, participantID string, isEligible bool, reason, updatedBy string) error {
	return s.timeSlotRepo.UpsertEligibilityStatusMRA(ctx, participantID, isEligible, reason, updatedBy)
}

func (s *ParticipantService) ResetIneligibleMailSentMRA(ctx context.Context, externalResponderID string) error {
	return s.timeSlotRepo.ResetIneligibleMailSentMRA(ctx, externalResponderID)
}
