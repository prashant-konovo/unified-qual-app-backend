package service

import (
	"context"
	"io"

	"github.com/InCrowd/unified-qual-api/internal/integration"
	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
)

// MediaService encapsulates interview media management logic.
type MediaService struct {
	irisRepo        iris.SurveyRepository
	s3              *integration.S3Client
	recordingBucket string
}

// NewMediaService creates a new MediaService.
func NewMediaService(irisRepo iris.SurveyRepository, s3 *integration.S3Client, recordingBucket string) *MediaService {
	return &MediaService{irisRepo: irisRepo, s3: s3, recordingBucket: recordingBucket}
}

// Available returns true if the IRIS survey repository is configured.
func (s *MediaService) Available() bool { return s.irisRepo != nil }

func (s *MediaService) S3Configured() bool {
	return s.s3 != nil && s.s3.Configured()
}

func (s *MediaService) RecordingBucket() string {
	return s.recordingBucket
}

func (s *MediaService) GetS3Object(ctx context.Context, bucket, key string) (io.ReadCloser, int64, error) {
	return s.s3.GetObject(ctx, bucket, key)
}

func (s *MediaService) DeleteS3Object(ctx context.Context, bucket, key string) error {
	return s.s3.DeleteObject(ctx, bucket, key)
}

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

// UpsertInterviewMedia inserts or updates recording metadata in IRIS.
func (s *MediaService) UpsertInterviewMedia(ctx context.Context, meetingID string, projectID, subscriptionID int64, chimeMeetingID string, recordingURL, bucket, key string, duration int, size int64) error {
	return s.irisRepo.UpsertInterviewMedia(ctx, meetingID, projectID, subscriptionID, chimeMeetingID, recordingURL, bucket, key, duration, size)
}
