package service

import (
	"context"

	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
)

// NotificationService encapsulates email template and communication logic.
type NotificationService struct {
	irisRepo iris.SurveyRepository
	qsRepo   qs.AnswerRepository
}

// NewNotificationService creates a new NotificationService.
func NewNotificationService(irisRepo iris.SurveyRepository, qsRepo qs.AnswerRepository) *NotificationService {
	return &NotificationService{irisRepo: irisRepo, qsRepo: qsRepo}
}

// IrisAvailable returns true if the IRIS survey repository is configured.
func (s *NotificationService) IrisAvailable() bool { return s.irisRepo != nil }

// QsAvailable returns true if the QS answer repository is configured.
func (s *NotificationService) QsAvailable() bool { return s.qsRepo != nil }

func (s *NotificationService) GetEmailTemplateForProject(ctx context.Context, projectID int64) (map[string]any, error) {
	return s.irisRepo.GetEmailTemplateForProject(ctx, projectID)
}

func (s *NotificationService) GetCommunicationTemplate(ctx context.Context, name string) (*qs.CommunicationTemplate, error) {
	return s.qsRepo.GetCommunicationTemplate(ctx, name)
}
