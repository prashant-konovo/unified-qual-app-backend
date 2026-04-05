package service

import (
	"context"

	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
)

// AdminService encapsulates admin user management and PM dashboard logic.
type AdminService struct {
	qsUserRepo   qs.UserRepository
	irisUserRepo iris.UserRepository
	timeSlotRepo qs.TimeSlotRepository
}

// NewAdminService creates a new AdminService.
func NewAdminService(qsUserRepo qs.UserRepository, irisUserRepo iris.UserRepository, timeSlotRepo qs.TimeSlotRepository) *AdminService {
	return &AdminService{qsUserRepo: qsUserRepo, irisUserRepo: irisUserRepo, timeSlotRepo: timeSlotRepo}
}

// QsAvailable returns true if the QS user repository is configured.
func (s *AdminService) QsAvailable() bool { return s.qsUserRepo != nil }

// IrisAvailable returns true if the IRIS user repository is configured.
func (s *AdminService) IrisAvailable() bool { return s.irisUserRepo != nil }

// TimeSlotAvailable returns true if the QS timeslot repository is configured.
func (s *AdminService) TimeSlotAvailable() bool { return s.timeSlotRepo != nil }

// --- QsUserRepo methods ---

func (s *AdminService) ListQsUsers(ctx context.Context, page, pageSize int, roleID *int, search string) ([]qs.UserListRow, int, error) {
	return s.qsUserRepo.List(ctx, page, pageSize, roleID, search)
}

func (s *AdminService) CreateQsUser(ctx context.Context, firstName, lastName, email, timeZone string, roleIDs []int) (int64, error) {
	return s.qsUserRepo.Create(ctx, firstName, lastName, email, timeZone, roleIDs)
}

func (s *AdminService) CreateUserCommPrefs(ctx context.Context, userID int64, email, cognitoUserID string) error {
	return s.qsUserRepo.CreateUserCommPrefs(ctx, userID, email, cognitoUserID)
}

func (s *AdminService) CreateUserCommPrefsAdmin(ctx context.Context, userID int64, email string) error {
	return s.qsUserRepo.CreateUserCommPrefsAdmin(ctx, userID, email)
}

func (s *AdminService) GetByEmailIncludeDeleted(ctx context.Context, email string) (*qs.UserWithRoles, error) {
	return s.qsUserRepo.GetByEmailIncludeDeleted(ctx, email)
}

func (s *AdminService) GetByEmail(ctx context.Context, email string) (*qs.UserWithRoles, error) {
	return s.qsUserRepo.GetByEmail(ctx, email)
}

func (s *AdminService) AddUserClient(ctx context.Context, userID, clientID int64) error {
	return s.qsUserRepo.AddUserClient(ctx, userID, clientID)
}

func (s *AdminService) RestoreByEmail(ctx context.Context, email string) error {
	return s.qsUserRepo.RestoreByEmail(ctx, email)
}

func (s *AdminService) AddRoles(ctx context.Context, userID int64, roleIDs []int) error {
	return s.qsUserRepo.AddRoles(ctx, userID, roleIDs)
}

func (s *AdminService) DeleteRoles(ctx context.Context, userID int64, roleIDs []int) error {
	return s.qsUserRepo.DeleteRoles(ctx, userID, roleIDs)
}

func (s *AdminService) SoftDelete(ctx context.Context, userID int64) error {
	return s.qsUserRepo.SoftDelete(ctx, userID)
}

func (s *AdminService) GetAllUsersAdmin(ctx context.Context, cognitoUserID string) ([]map[string]any, error) {
	return s.qsUserRepo.GetAllUsersAdmin(ctx, cognitoUserID)
}

func (s *AdminService) GetAllProjectManagersListMRA(ctx context.Context) ([]map[string]any, error) {
	return s.qsUserRepo.GetAllProjectManagersListMRA(ctx)
}

// --- IrisUserRepo methods ---

func (s *AdminService) ListIrisUsers(ctx context.Context, page, pageSize int, roleID *int64, search string) ([]iris.ICUserListRow, int, error) {
	return s.irisUserRepo.List(ctx, page, pageSize, roleID, search)
}

// --- QsTimeSlotRepo methods (PM dashboard) ---

func (s *AdminService) GetAllPendingInterviewsPerProjectMRA(ctx context.Context, projectID, clientID int64) ([]map[string]any, error) {
	return s.timeSlotRepo.GetAllPendingInterviewsPerProjectMRA(ctx, projectID, clientID)
}

func (s *AdminService) GetPMTimeSlotsByClientIdMRA(ctx context.Context, clientID int64) ([]map[string]any, error) {
	return s.timeSlotRepo.GetPMTimeSlotsByClientIdMRA(ctx, clientID)
}

func (s *AdminService) GetPMTimeSlotsByClientIdWithProjectFilterMRA(ctx context.Context, clientID int64, projectIDs []int64) ([]map[string]any, error) {
	return s.timeSlotRepo.GetPMTimeSlotsByClientIdWithProjectFilterMRA(ctx, clientID, projectIDs)
}

func (s *AdminService) GetPMTimeSlotsByClientIdWithModeratorFilterMRA(ctx context.Context, clientID int64, moderatorIDs []int64) ([]map[string]any, error) {
	return s.timeSlotRepo.GetPMTimeSlotsByClientIdWithModeratorFilterMRA(ctx, clientID, moderatorIDs)
}

func (s *AdminService) GetAllModeratorsAvailabilityForPMMRA(ctx context.Context, clientID int64) ([]map[string]any, error) {
	return s.qsUserRepo.GetAllModeratorsAvailabilityForPMMRA(ctx, clientID)
}

func (s *AdminService) GetAllModeratorsAvailabilityForPMWithProjectFilterMRA(ctx context.Context, clientID int64, projectIDs []int64) ([]map[string]any, error) {
	return s.qsUserRepo.GetAllModeratorsAvailabilityForPMWithProjectFilterMRA(ctx, clientID, projectIDs)
}

func (s *AdminService) GetAllModeratorsAvailabilityForPMWithModeratorFilterMRA(ctx context.Context, clientID int64, moderatorIDs []int64) ([]map[string]any, error) {
	return s.qsUserRepo.GetAllModeratorsAvailabilityForPMWithModeratorFilterMRA(ctx, clientID, moderatorIDs)
}
