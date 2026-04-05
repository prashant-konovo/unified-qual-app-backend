package service

import (
	"context"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/integration"
	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
)

// ModeratorService encapsulates moderator and availability logic.
type ModeratorService struct {
	qsUserRepo   qs.UserRepository
	timeSlotRepo qs.TimeSlotRepository
	googleCal    *integration.GoogleCalendarClient
	googleSheets *integration.GoogleSheetsClient
}

// NewModeratorService creates a new ModeratorService.
func NewModeratorService(qsUserRepo qs.UserRepository, timeSlotRepo qs.TimeSlotRepository, googleCal *integration.GoogleCalendarClient, googleSheets *integration.GoogleSheetsClient) *ModeratorService {
	return &ModeratorService{qsUserRepo: qsUserRepo, timeSlotRepo: timeSlotRepo, googleCal: googleCal, googleSheets: googleSheets}
}

// QsUserAvailable returns true if the QS user repository is configured.
func (s *ModeratorService) QsUserAvailable() bool { return s.qsUserRepo != nil }

// TimeSlotAvailable returns true if the QS time slot repository is configured.
func (s *ModeratorService) TimeSlotAvailable() bool { return s.timeSlotRepo != nil }

func (s *ModeratorService) GoogleCalConfigured() bool {
	return s.googleCal != nil && s.googleCal.Configured()
}

func (s *ModeratorService) ListGoogleCalEvents(ctx context.Context, calendarID string, from, to time.Time) ([]integration.CalendarEvent, error) {
	return s.googleCal.ListEvents(ctx, calendarID, from, to)
}

func (s *ModeratorService) DeleteGoogleCalEvent(ctx context.Context, calendarID, eventID string) error {
	return s.googleCal.DeleteEvent(ctx, calendarID, eventID)
}

func (s *ModeratorService) CreateGoogleCalEvent(ctx context.Context, calendarID string, event *integration.CalendarEvent) (*integration.CalendarEvent, error) {
	return s.googleCal.CreateEvent(ctx, calendarID, event)
}

func (s *ModeratorService) GoogleSheetsConfigured() bool {
	return s.googleSheets != nil && s.googleSheets.Configured()
}

func (s *ModeratorService) UpdateGoogleSheetFirstDate(ctx context.Context, sheetName, cellRange, value string) error {
	return s.googleSheets.UpdateFirstDate(ctx, sheetName, cellRange, value)
}

func (s *ModeratorService) UpdateFirstDateCalendar(ctx context.Context) error {
	return s.googleSheets.UpdateFirstDateCalendar(ctx)
}

// --- QsUserRepo delegation methods ---

func (s *ModeratorService) Create(ctx context.Context, firstName, lastName, email, timeZone string, roleIDs []int) (int64, error) {
	return s.qsUserRepo.Create(ctx, firstName, lastName, email, timeZone, roleIDs)
}

func (s *ModeratorService) GetByID(ctx context.Context, id int64) (*qs.UserWithRoles, error) {
	return s.qsUserRepo.GetByID(ctx, id)
}

func (s *ModeratorService) GetModerators(ctx context.Context) ([]qs.UserListRow, error) {
	return s.qsUserRepo.GetModerators(ctx)
}

func (s *ModeratorService) UpdateUser(ctx context.Context, id int64, firstName, lastName, timeZone string) error {
	return s.qsUserRepo.Update(ctx, id, firstName, lastName, timeZone)
}

func (s *ModeratorService) SoftDelete(ctx context.Context, id int64) error {
	return s.qsUserRepo.SoftDelete(ctx, id)
}

func (s *ModeratorService) ListModeratorAvailability(ctx context.Context, moderatorID int64, clientID *int64, startDate, endDate string) ([]qs.ModeratorAvailability, error) {
	return s.qsUserRepo.ListModeratorAvailability(ctx, moderatorID, clientID, startDate, endDate)
}

func (s *ModeratorService) CreateModeratorAvailability(ctx context.Context, moderatorID, clientID int64, startTime, endTime time.Time) (*qs.ModeratorAvailability, error) {
	return s.qsUserRepo.CreateModeratorAvailability(ctx, moderatorID, clientID, startTime, endTime)
}

func (s *ModeratorService) UpdateModeratorAvailability(ctx context.Context, id int64, startTime, endTime time.Time) error {
	return s.qsUserRepo.UpdateModeratorAvailability(ctx, id, startTime, endTime)
}

func (s *ModeratorService) DeleteModeratorAvailability(ctx context.Context, id int64) error {
	return s.qsUserRepo.DeleteModeratorAvailability(ctx, id)
}

func (s *ModeratorService) FetchUserInfoByUserIdMRA(ctx context.Context, userID int64) (int, int64, error) {
	return s.qsUserRepo.FetchUserInfoByUserIdMRA(ctx, userID)
}

func (s *ModeratorService) UpdateModeratorBufferMRA(ctx context.Context, userID int64, buffer int) error {
	return s.qsUserRepo.UpdateModeratorBufferMRA(ctx, userID, buffer)
}

func (s *ModeratorService) GetFutureModeratorTimeslotsMRA(ctx context.Context, moderatorID, clientID int64) ([]qs.ModeratorTimeslotMRA, error) {
	return s.qsUserRepo.GetFutureModeratorTimeslotsMRA(ctx, moderatorID, clientID)
}

func (s *ModeratorService) GetFutureAvailsWithProximityMRA(ctx context.Context, moderatorID, clientID int64) ([]qs.AvailWithProximityMRA, error) {
	return s.qsUserRepo.GetFutureAvailsWithProximityMRA(ctx, moderatorID, clientID)
}

func (s *ModeratorService) GetFutureImportedAvailsWithProximityMRA(ctx context.Context, moderatorID, clientID int64) ([]qs.AvailWithProximityMRA, error) {
	return s.qsUserRepo.GetFutureImportedAvailsWithProximityMRA(ctx, moderatorID, clientID)
}

func (s *ModeratorService) GetModeratorAvailabilityLengthMRA(ctx context.Context, availID int64) (*qs.AvailLengthMRA, error) {
	return s.qsUserRepo.GetModeratorAvailabilityLengthMRA(ctx, availID)
}

func (s *ModeratorService) GetImportedModeratorAvailabilityLengthMRA(ctx context.Context, availID int64) (*qs.AvailLengthMRA, error) {
	return s.qsUserRepo.GetImportedModeratorAvailabilityLengthMRA(ctx, availID)
}

func (s *ModeratorService) DeleteModeratorAvailabilityByIdMRA(ctx context.Context, id int64) error {
	return s.qsUserRepo.DeleteModeratorAvailabilityByIdMRA(ctx, id)
}

func (s *ModeratorService) DeleteImportedModeratorAvailabilityByIdMRA(ctx context.Context, id int64) error {
	return s.qsUserRepo.DeleteImportedModeratorAvailabilityByIdMRA(ctx, id)
}

func (s *ModeratorService) UpdateModeratorAvailabilityStartTimeMRA(ctx context.Context, id int64, startTime time.Time) error {
	return s.qsUserRepo.UpdateModeratorAvailabilityStartTimeMRA(ctx, id, startTime)
}

func (s *ModeratorService) UpdateModeratorAvailabilityEndTimeMRA(ctx context.Context, id int64, endTime time.Time) error {
	return s.qsUserRepo.UpdateModeratorAvailabilityEndTimeMRA(ctx, id, endTime)
}

func (s *ModeratorService) UpdateImportedModeratorAvailabilityStartTimeMRA(ctx context.Context, id int64, startTime time.Time) error {
	return s.qsUserRepo.UpdateImportedModeratorAvailabilityStartTimeMRA(ctx, id, startTime)
}

func (s *ModeratorService) UpdateImportedModeratorAvailabilityEndTimeMRA(ctx context.Context, id int64, endTime time.Time) error {
	return s.qsUserRepo.UpdateImportedModeratorAvailabilityEndTimeMRA(ctx, id, endTime)
}

func (s *ModeratorService) CleanUpAvailabilitiesByModeratorIdMRA(ctx context.Context, moderatorID int64) error {
	return s.qsUserRepo.CleanUpAvailabilitiesByModeratorIdMRA(ctx, moderatorID)
}

func (s *ModeratorService) RemoveNestedAvailabilitiesMRA(ctx context.Context, moderatorID int64) error {
	return s.qsUserRepo.RemoveNestedAvailabilitiesMRA(ctx, moderatorID)
}

func (s *ModeratorService) GetModeratorBufferMRA(ctx context.Context, moderatorID int64) (int, error) {
	return s.qsUserRepo.GetModeratorBufferMRA(ctx, moderatorID)
}

func (s *ModeratorService) GetModExternalCalendarStatusMRA(ctx context.Context, moderatorID int64) (string, error) {
	return s.qsUserRepo.GetModExternalCalendarStatusMRA(ctx, moderatorID)
}

func (s *ModeratorService) IsValidAvailabilityMRA(ctx context.Context, moderatorID int64, startTime, endTime string, buffer int) (int64, error) {
	return s.qsUserRepo.IsValidAvailabilityMRA(ctx, moderatorID, startTime, endTime, buffer)
}

func (s *ModeratorService) OverlappingAvailabilitiesMRA(ctx context.Context, moderatorID int64, startTime, endTime string) ([]map[string]any, error) {
	return s.qsUserRepo.OverlappingAvailabilitiesMRA(ctx, moderatorID, startTime, endTime)
}

func (s *ModeratorService) GetAllModeratorAvailabilityWithUserMRA(ctx context.Context, moderatorID, clientID int64) ([]map[string]any, error) {
	return s.qsUserRepo.GetAllModeratorAvailabilityWithUserMRA(ctx, moderatorID, clientID)
}

func (s *ModeratorService) GetNonOverlappingManualAvailabilityMRA(ctx context.Context, moderatorID, clientID int64) ([]map[string]any, error) {
	return s.qsUserRepo.GetNonOverlappingManualAvailabilityMRA(ctx, moderatorID, clientID)
}

func (s *ModeratorService) GetNonOverlappingImportedAvailabilityMRA(ctx context.Context, moderatorID, clientID int64) ([]map[string]any, error) {
	return s.qsUserRepo.GetNonOverlappingImportedAvailabilityMRA(ctx, moderatorID, clientID)
}

func (s *ModeratorService) GetAllOverlappingManualAvailabilityMRA(ctx context.Context, moderatorID, clientID int64) ([]map[string]any, error) {
	return s.qsUserRepo.GetAllOverlappingManualAvailabilityMRA(ctx, moderatorID, clientID)
}

func (s *ModeratorService) GetAllOverlappingImportedAvailabilityMRA(ctx context.Context, moderatorID, clientID int64) ([]map[string]any, error) {
	return s.qsUserRepo.GetAllOverlappingImportedAvailabilityMRA(ctx, moderatorID, clientID)
}

func (s *ModeratorService) FindModeratorAvailabilityByIdMRA(ctx context.Context, id int64) (map[string]any, error) {
	return s.qsUserRepo.FindModeratorAvailabilityByIdMRA(ctx, id)
}

func (s *ModeratorService) GetModeratorsInfoByAvailabilityIdMRA(ctx context.Context, availabilityID int64) (map[string]any, error) {
	return s.qsUserRepo.GetModeratorsInfoByAvailabilityIdMRA(ctx, availabilityID)
}

func (s *ModeratorService) GetOverlappingImportedAvailabilityMRA(ctx context.Context, moderatorID, clientID int64, startTime, endTime string) ([]map[string]any, error) {
	return s.qsUserRepo.GetOverlappingImportedAvailabilityMRA(ctx, moderatorID, clientID, startTime, endTime)
}

func (s *ModeratorService) AddModeratorAvailabilityFromImportedMRA(ctx context.Context, moderatorID, clientID int64, startTime, endTime string) error {
	return s.qsUserRepo.AddModeratorAvailabilityFromImportedMRA(ctx, moderatorID, clientID, startTime, endTime)
}

func (s *ModeratorService) GetAllModeratorsAvailabilityPerClientMRA(ctx context.Context, clientID, projectID int64) ([]map[string]any, error) {
	return s.qsUserRepo.GetAllModeratorsAvailabilityPerClientMRA(ctx, clientID, projectID)
}

func (s *ModeratorService) GetModeratorsListMRA(ctx context.Context, projectID int64) ([]map[string]any, error) {
	return s.qsUserRepo.GetModeratorsListMRA(ctx, projectID)
}

func (s *ModeratorService) GetAllModeratorsListMRA(ctx context.Context, clientID int64) ([]map[string]any, error) {
	return s.qsUserRepo.GetAllModeratorsListMRA(ctx, clientID)
}

func (s *ModeratorService) UpdateModExternalCalendarUrlMRA(ctx context.Context, moderatorID int64, url, key string) error {
	return s.qsUserRepo.UpdateModExternalCalendarUrlMRA(ctx, moderatorID, url, key)
}

func (s *ModeratorService) UpdateModExternalCalendarStatusMRA(ctx context.Context, moderatorID int64, status string) error {
	return s.qsUserRepo.UpdateModExternalCalendarStatusMRA(ctx, moderatorID, status)
}

func (s *ModeratorService) DeleteImportedModeratorAvailabilityByModeratorMRA(ctx context.Context, moderatorID, clientID int64) error {
	return s.qsUserRepo.DeleteImportedModeratorAvailabilityByModeratorMRA(ctx, moderatorID, clientID)
}

func (s *ModeratorService) GetImportedModeratorAvailabilityMRA(ctx context.Context, moderatorID, clientID int64) ([]map[string]any, error) {
	return s.qsUserRepo.GetImportedModeratorAvailabilityMRA(ctx, moderatorID, clientID)
}

func (s *ModeratorService) GetModeratorExternalCalendarMRA(ctx context.Context, moderatorID int64) ([]map[string]any, error) {
	return s.qsUserRepo.GetModeratorExternalCalendarMRA(ctx, moderatorID)
}

func (s *ModeratorService) GetRunningImportProcessCountMRA(ctx context.Context) (int64, error) {
	return s.qsUserRepo.GetRunningImportProcessCountMRA(ctx)
}

func (s *ModeratorService) DeleteExternalCalStatusMRA(ctx context.Context, moderatorID int64) error {
	return s.qsUserRepo.DeleteExternalCalStatusMRA(ctx, moderatorID)
}

// --- QsTimeSlotRepo delegation methods ---

func (s *ModeratorService) ListByModerator(ctx context.Context, moderatorID int64, page, pageSize int) ([]qs.TimeSlotListRow, int, error) {
	return s.timeSlotRepo.ListByModerator(ctx, moderatorID, page, pageSize)
}

func (s *ModeratorService) UpdateTimeSlot(ctx context.Context, id int64, fields map[string]any) error {
	return s.timeSlotRepo.Update(ctx, id, fields)
}

func (s *ModeratorService) GetAllInterviewsMRA(ctx context.Context, moderatorID int64, search string, excludeProjectIDs []int64, paymentStatusCode string) ([]map[string]any, error) {
	return s.timeSlotRepo.GetAllInterviewsMRA(ctx, moderatorID, search, excludeProjectIDs, paymentStatusCode)
}

func (s *ModeratorService) GetInvalidTimeSlotStatusMRA(ctx context.Context, externalResponderID string, projectID int64) ([]map[string]any, error) {
	return s.timeSlotRepo.GetInvalidTimeSlotStatusMRA(ctx, externalResponderID, projectID)
}

func (s *ModeratorService) GetInvalidTimeSlotStatusForRespRescMRA(ctx context.Context, externalResponderID string, projectID int64) ([]map[string]any, error) {
	return s.timeSlotRepo.GetInvalidTimeSlotStatusForRespRescMRA(ctx, externalResponderID, projectID)
}

func (s *ModeratorService) GetPendingTimeslotByProjectAndResponderMRA(ctx context.Context, projectID, responderID int64) ([]map[string]any, error) {
	return s.timeSlotRepo.GetPendingTimeslotByProjectAndResponderMRA(ctx, projectID, responderID)
}

func (s *ModeratorService) GetModeratorTimeSlotsMRA(ctx context.Context, moderatorID, clientID int64) ([]map[string]any, error) {
	return s.timeSlotRepo.GetModeratorTimeSlotsMRA(ctx, moderatorID, clientID)
}

func (s *ModeratorService) GetModeratorTimeSlotsWithFilterMRA(ctx context.Context, moderatorID, clientID int64, excludeProjectIDs []int64) ([]map[string]any, error) {
	return s.timeSlotRepo.GetModeratorTimeSlotsWithFilterMRA(ctx, moderatorID, clientID, excludeProjectIDs)
}

func (s *ModeratorService) GetModeratorsForTimeSlotMRA(ctx context.Context, timeslotID int64) ([]map[string]any, error) {
	return s.timeSlotRepo.GetModeratorsForTimeSlotMRA(ctx, timeslotID)
}

func (s *ModeratorService) GetModeratorsInfoForSlotMRA(ctx context.Context, timeslotID int64) ([]qs.ModeratorInfoForSlotMRA, error) {
	return s.timeSlotRepo.GetModeratorsInfoForSlotMRA(ctx, timeslotID)
}

func (s *ModeratorService) GetStartEndTimeBySlotIdMRA(ctx context.Context, timeslotID int64) (string, string, error) {
	return s.timeSlotRepo.GetStartEndTimeBySlotIdMRA(ctx, timeslotID)
}

func (s *ModeratorService) GetModeratorConflictForSlotMRA(ctx context.Context, moderatorID int64, startTime, endTime string, timeslotID int64) (int, error) {
	return s.timeSlotRepo.GetModeratorConflictForSlotMRA(ctx, moderatorID, startTime, endTime, timeslotID)
}
