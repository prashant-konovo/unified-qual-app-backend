package service

import (
	"context"

	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
)

// UserService encapsulates user profile and preference operations.
type UserService struct {
	qsUserRepo   qs.UserRepository
	irisUserRepo iris.UserRepository
}

// NewUserService creates a new UserService.
func NewUserService(qsUserRepo qs.UserRepository, irisUserRepo iris.UserRepository) *UserService {
	return &UserService{qsUserRepo: qsUserRepo, irisUserRepo: irisUserRepo}
}

// QsAvailable returns true if the QS user repository is configured.
func (s *UserService) QsAvailable() bool { return s.qsUserRepo != nil }

// IrisAvailable returns true if the IRIS user repository is configured.
func (s *UserService) IrisAvailable() bool { return s.irisUserRepo != nil }

// --- QsUserRepo methods ---

func (s *UserService) GetByID(ctx context.Context, id int64) (*qs.UserWithRoles, error) {
	return s.qsUserRepo.GetByID(ctx, id)
}

func (s *UserService) Update(ctx context.Context, id int64, firstName, lastName, timeZone string) error {
	return s.qsUserRepo.Update(ctx, id, firstName, lastName, timeZone)
}

func (s *UserService) GetEmailByCognitoID(ctx context.Context, cognitoID string) (string, error) {
	return s.qsUserRepo.GetEmailByCognitoID(ctx, cognitoID)
}

func (s *UserService) UpdateTimeZone(ctx context.Context, userID int64, timeZone string) error {
	return s.qsUserRepo.UpdateTimeZone(ctx, userID, timeZone)
}

func (s *UserService) CheckUserIsQsToolAndI2(ctx context.Context, email string) (map[string]any, error) {
	return s.qsUserRepo.CheckUserIsQsToolAndI2(ctx, email)
}

func (s *UserService) GetUserCommPreference(ctx context.Context, userID int64) ([]map[string]any, error) {
	return s.qsUserRepo.GetUserCommPreference(ctx, userID)
}

func (s *UserService) UpdateUserCommPreference(ctx context.Context, cognitoUserID, pmUserID string, allowContactByEmail int) ([]map[string]any, error) {
	return s.qsUserRepo.UpdateUserCommPreference(ctx, cognitoUserID, pmUserID, allowContactByEmail)
}

// --- IrisUserRepo methods ---

func (s *UserService) GetIrisUserByID(ctx context.Context, id int64) (*iris.ICUserWithRoles, error) {
	return s.irisUserRepo.GetByID(ctx, id)
}
