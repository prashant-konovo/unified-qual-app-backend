package service

import (
	"context"

	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
)

// ConferenceService encapsulates conference/meeting management logic.
type ConferenceService struct {
	repo qs.ConferenceRepository
}

// NewConferenceService creates a new ConferenceService.
func NewConferenceService(repo qs.ConferenceRepository) *ConferenceService {
	return &ConferenceService{repo: repo}
}

// Available returns true if the conference repository is configured.
func (s *ConferenceService) Available() bool { return s.repo != nil }

// --- Shared methods ---

func (s *ConferenceService) GetByHash(ctx context.Context, hash string) (*qs.ConferenceInvitation, error) {
	return s.repo.GetByHash(ctx, hash)
}

func (s *ConferenceService) GetParticipants(ctx context.Context, timeSlotID int64) ([]map[string]any, error) {
	return s.repo.GetParticipants(ctx, timeSlotID)
}

func (s *ConferenceService) GetAttendeesByMeetingID(ctx context.Context, meetingID string) ([]map[string]any, error) {
	return s.repo.GetAttendeesByMeetingID(ctx, meetingID)
}

func (s *ConferenceService) GetMeetingMetadata(ctx context.Context, conferenceHash string) (map[string]any, error) {
	return s.repo.GetMeetingMetadata(ctx, conferenceHash)
}

func (s *ConferenceService) Login(ctx context.Context, conferenceHash, pin string) (map[string]any, error) {
	return s.repo.Login(ctx, conferenceHash, pin)
}

func (s *ConferenceService) CreateConferenceLink(ctx context.Context, timeSlotID, projectID int64, conferenceHash string) (int64, error) {
	return s.repo.CreateConferenceLink(ctx, timeSlotID, projectID, conferenceHash)
}

func (s *ConferenceService) UpdateConferenceLink(ctx context.Context, timeSlotID int64, conferenceHash, pin string) error {
	return s.repo.UpdateConferenceLink(ctx, timeSlotID, conferenceHash, pin)
}

func (s *ConferenceService) GetConferenceLinkByTimeSlotID(ctx context.Context, timeSlotID int64) (map[string]any, error) {
	return s.repo.GetConferenceLinkByTimeSlotID(ctx, timeSlotID)
}

func (s *ConferenceService) UpdateRecordingStatus(ctx context.Context, meetingID, status, bucket, key string) error {
	return s.repo.UpdateRecordingStatus(ctx, meetingID, status, bucket, key)
}

// --- MRA methods ---

func (s *ConferenceService) AddConferenceLinkMRA(ctx context.Context, projectID, participantGroupID, userID int64, conferenceLink string, meetingInformation [][]any) (map[string]any, error) {
	return s.repo.AddConferenceLinkMRA(ctx, projectID, participantGroupID, userID, conferenceLink, meetingInformation)
}

func (s *ConferenceService) GetExistingMeetingLanguagesMRA(ctx context.Context, projectID int64) (map[string]bool, error) {
	return s.repo.GetExistingMeetingLanguagesMRA(ctx, projectID)
}

func (s *ConferenceService) UpdateConferenceLinkMRA(ctx context.Context, projectID, participantGroupID, userID int64, conferenceLink string, meetingInformation [][]any, existingLangs map[string]bool) error {
	return s.repo.UpdateConferenceLinkMRA(ctx, projectID, participantGroupID, userID, conferenceLink, meetingInformation, existingLangs)
}

func (s *ConferenceService) GetPendingTimeSlotsCountMRA(ctx context.Context, projectID int64) (int64, error) {
	return s.repo.GetPendingTimeSlotsCountMRA(ctx, projectID)
}

func (s *ConferenceService) GetConferenceLinkByProjectMRA(ctx context.Context, projectID int64) ([]map[string]any, error) {
	return s.repo.GetConferenceLinkByProjectMRA(ctx, projectID)
}

func (s *ConferenceService) GetConferenceLinkByTimeSlotMRA(ctx context.Context, timeSlotID int64) (map[string]any, error) {
	return s.repo.GetConferenceLinkByTimeSlotMRA(ctx, timeSlotID)
}
