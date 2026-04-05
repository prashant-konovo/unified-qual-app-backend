package service

import (
	"context"

	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
)

// MediaService encapsulates interview media management logic.
type MediaService struct {
	irisRepo iris.SurveyRepository
}

// NewMediaService creates a new MediaService.
func NewMediaService(irisRepo iris.SurveyRepository) *MediaService {
	return &MediaService{irisRepo: irisRepo}
}

// Available returns true if the IRIS survey repository is configured.
func (s *MediaService) Available() bool { return s.irisRepo != nil }

func (s *MediaService) ListMediaForProject(ctx context.Context, projectID int64) ([]iris.ICInterviewMedia, error) {
	return s.irisRepo.ListMediaForProject(ctx, projectID)
}

func (s *MediaService) GetMediaByID(ctx context.Context, projectID, mediaID int64) (*iris.ICInterviewMedia, error) {
	return s.irisRepo.GetMediaByID(ctx, projectID, mediaID)
}

func (s *MediaService) DeleteMedia(ctx context.Context, projectID, mediaID int64) error {
	return s.irisRepo.DeleteMedia(ctx, projectID, mediaID)
}

func (s *MediaService) GetProjectIDByConferenceHash(ctx context.Context, hash string) (int64, error) {
	return s.irisRepo.GetProjectIDByConferenceHash(ctx, hash)
}
