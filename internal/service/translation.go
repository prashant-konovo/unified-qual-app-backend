package service

import (
	"context"

	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
)

// TranslationService encapsulates translation and localisation business logic.
type TranslationService struct {
	answerRepo  qs.AnswerRepository
	projectRepo qs.ProjectRepository
}

// NewTranslationService creates a new TranslationService.
func NewTranslationService(answerRepo qs.AnswerRepository, projectRepo qs.ProjectRepository) *TranslationService {
	return &TranslationService{answerRepo: answerRepo, projectRepo: projectRepo}
}

// AnswerRepoAvailable returns true if the QS answer repository is configured.
func (s *TranslationService) AnswerRepoAvailable() bool { return s.answerRepo != nil }

// ProjectRepoAvailable returns true if the QS project repository is configured.
func (s *TranslationService) ProjectRepoAvailable() bool { return s.projectRepo != nil }

// --- QsAnswerRepo methods ---

func (s *TranslationService) ListLocales(ctx context.Context) ([]qs.LanguageLocalisation, error) {
	return s.answerRepo.ListLocales(ctx)
}

func (s *TranslationService) UpdateTopicTranslation(ctx context.Context, projectID, topicID int64, languageCode, translatedName string) error {
	return s.answerRepo.UpdateTopicTranslation(ctx, projectID, topicID, languageCode, translatedName)
}

func (s *TranslationService) DeleteTopicTranslation(ctx context.Context, topicID int64, languageCode string) error {
	return s.answerRepo.DeleteTopicTranslation(ctx, topicID, languageCode)
}

func (s *TranslationService) DeleteTranslation(ctx context.Context, projectID int64, languageCode string) error {
	return s.answerRepo.DeleteTranslation(ctx, projectID, languageCode)
}

// --- QsProjectRepo methods (MRA translation operations) ---

func (s *TranslationService) GetTopicsByProjectMRA(ctx context.Context, projectID int64) ([]map[string]any, error) {
	return s.projectRepo.GetTopicsByProjectMRA(ctx, projectID)
}

func (s *TranslationService) GetProjectStatusByIdMRA(ctx context.Context, projectID int64) (int64, error) {
	return s.projectRepo.GetProjectStatusByIdMRA(ctx, projectID)
}

func (s *TranslationService) GetRespondersLanguagesByProjectIdMRA(ctx context.Context, projectID int64) ([]string, error) {
	return s.projectRepo.GetRespondersLanguagesByProjectIdMRA(ctx, projectID)
}

func (s *TranslationService) GetTopicsByProjectIdAndLanguageIdMRA(ctx context.Context, projectID, languageID int64) (bool, error) {
	return s.projectRepo.GetTopicsByProjectIdAndLanguageIdMRA(ctx, projectID, languageID)
}

func (s *TranslationService) UpdateTopicByProjectAndLanguageMRA(ctx context.Context, topicName string, languageID, projectID int64, userID any) error {
	return s.projectRepo.UpdateTopicByProjectAndLanguageMRA(ctx, topicName, languageID, projectID, userID)
}

func (s *TranslationService) AddTopicByProjectAndLanguageMRA(ctx context.Context, topicName string, languageID, projectID int64, userID any) error {
	return s.projectRepo.AddTopicByProjectAndLanguageMRA(ctx, topicName, languageID, projectID, userID)
}

func (s *TranslationService) DeleteTopicByProjectAndLanguageMRA(ctx context.Context, projectID, languageID int64) error {
	return s.projectRepo.DeleteTopicByProjectAndLanguageMRA(ctx, projectID, languageID)
}

func (s *TranslationService) GetAllLanguageLocalisationsMRA(ctx context.Context) ([]map[string]any, error) {
	return s.projectRepo.GetAllLanguageLocalisationsMRA(ctx)
}

func (s *TranslationService) GetDataFromLanguageLocalisationsMRA(ctx context.Context) ([]map[string]any, error) {
	return s.projectRepo.GetDataFromLanguageLocalisationsMRA(ctx)
}

func (s *TranslationService) GetCountriesWithLocalisationsMRA(ctx context.Context) ([]map[string]any, error) {
	return s.projectRepo.GetCountriesWithLocalisationsMRA(ctx)
}

func (s *TranslationService) DeleteMeetingInformationTranslationMRA(ctx context.Context, projectID, languageID int64) (map[string]any, error) {
	return s.projectRepo.DeleteMeetingInformationTranslationMRA(ctx, projectID, languageID)
}

// --- QsAnswerRepo methods (used in ls/project.go, ls/integration.go) ---

func (s *TranslationService) ListNativeSurveysByProject(ctx context.Context, projectID int64) ([]qs.NativeSurvey, error) {
	return s.answerRepo.ListNativeSurveysByProject(ctx, projectID)
}

func (s *TranslationService) ListProjectsUsers(ctx context.Context, projectID int64) ([]map[string]any, error) {
	return s.answerRepo.ListProjectsUsers(ctx, projectID)
}

func (s *TranslationService) GetParticipantEligibility(ctx context.Context, responderID, projectID int64) (map[string]any, error) {
	return s.answerRepo.GetParticipantEligibility(ctx, responderID, projectID)
}

func (s *TranslationService) GetCommunicationTemplate(ctx context.Context, name string) (*qs.CommunicationTemplate, error) {
	return s.answerRepo.GetCommunicationTemplate(ctx, name)
}
