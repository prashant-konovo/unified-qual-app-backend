package service

import (
	"context"

	"github.com/InCrowd/unified-qual-api/internal/integration"
	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
)

// ConferenceService encapsulates conference/meeting management logic.
type ConferenceService struct {
	repo         qs.ConferenceRepository
	conference   *integration.ConferenceClient
	castingWords *integration.CastingWordsClient
	sms          *integration.SMSClient
	notification *integration.NotificationClient
}

// NewConferenceService creates a new ConferenceService.
func NewConferenceService(repo qs.ConferenceRepository, conference *integration.ConferenceClient, castingWords *integration.CastingWordsClient, sms *integration.SMSClient, notification *integration.NotificationClient) *ConferenceService {
	return &ConferenceService{repo: repo, conference: conference, castingWords: castingWords, sms: sms, notification: notification}
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

// --- Integration client wrappers ---

func (s *ConferenceService) ConferenceConfigured() bool {
	return s.conference != nil && s.conference.Configured()
}

func (s *ConferenceService) GetConferenceMetadata(ctx context.Context, bearerToken string) (map[string]any, error) {
	return s.conference.GetMetadata(ctx, bearerToken)
}

func (s *ConferenceService) GetConferenceAttendees(ctx context.Context, meetingID, bearerToken string) ([]map[string]any, error) {
	return s.conference.GetAttendees(ctx, meetingID, bearerToken)
}

func (s *ConferenceService) GetConferenceRecordingStatus(ctx context.Context, meetingID, bearerToken string) (map[string]any, error) {
	return s.conference.GetRecordingStatus(ctx, meetingID, bearerToken)
}

func (s *ConferenceService) CreateConferenceMeeting(ctx context.Context, req integration.MeetingCreateRequest, bearerToken string) (*integration.MeetingCreateResponse, error) {
	return s.conference.CreateMeeting(ctx, req, bearerToken)
}

func (s *ConferenceService) EndConferenceMeeting(ctx context.Context, meetingID, bearerToken string) error {
	return s.conference.EndMeeting(ctx, meetingID, bearerToken)
}

func (s *ConferenceService) StartConferenceRecording(ctx context.Context, meetingID, bearerToken string) error {
	return s.conference.StartRecording(ctx, meetingID, bearerToken)
}

func (s *ConferenceService) ConferenceUniversalJoin(ctx context.Context, meetingID, bearerToken string) (map[string]any, error) {
	return s.conference.UniversalJoin(ctx, meetingID, bearerToken)
}

func (s *ConferenceService) CastingWordsConfigured() bool {
	return s.castingWords != nil && s.castingWords.Configured()
}

func (s *ConferenceService) CreateTranscriptionOrder(ctx context.Context, audioURL string) (*integration.TranscriptionOrder, error) {
	return s.castingWords.CreateOrder(ctx, audioURL)
}

func (s *ConferenceService) GetTranscriptionStatus(ctx context.Context, orderID string) (*integration.TranscriptionOrder, error) {
	return s.castingWords.GetOrderStatus(ctx, orderID)
}

func (s *ConferenceService) GetTranscript(ctx context.Context, orderID string) (string, error) {
	return s.castingWords.GetTranscript(ctx, orderID)
}

func (s *ConferenceService) SMSConfigured() bool {
	return s.sms != nil && s.sms.Configured()
}

func (s *ConferenceService) SendSMS(ctx context.Context, to, from, message string) error {
	return s.sms.SendSMS(ctx, to, from, message)
}

func (s *ConferenceService) NotificationConfigured() bool {
	return s.notification != nil && s.notification.Configured()
}

func (s *ConferenceService) SendNotificationEmail(ctx context.Context, msg integration.EmailMessage) error {
	return s.notification.SendEmail(ctx, msg)
}
