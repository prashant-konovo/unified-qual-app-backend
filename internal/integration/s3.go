package integration

import (
	"context"
	"fmt"
	"io"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/InCrowd/unified-qual-api/internal/config"
)

// ──────────────────────────────────────────────
// S3 Client (AWS SDK v2)
// ──────────────────────────────────────────────

// S3Client wraps AWS SDK v2 S3 operations.
type S3Client struct {
	region        string
	inquiryBucket string
	exportBucket  string
}

func newS3Client(cfg config.S3Config) *S3Client {
	return &S3Client{region: cfg.Region, inquiryBucket: cfg.InquiryBucket, exportBucket: cfg.ExportBucket}
}

func (sc *S3Client) Configured() bool { return sc.inquiryBucket != "" }

func (sc *S3Client) InquiryBucket() string { return sc.inquiryBucket }

func (sc *S3Client) ExportBucket() string { return sc.exportBucket }

func (sc *S3Client) UploadFile(ctx context.Context, bucket, key string, body io.Reader, contentType string) (string, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(sc.region))
	if err != nil {
		return "", fmt.Errorf("load AWS config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg)
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &bucket,
		Key:         &key,
		Body:        body,
		ContentType: &contentType,
	})
	if err != nil {
		return "", fmt.Errorf("s3 upload: %w", err)
	}
	publicURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", bucket, sc.region, key)
	return publicURL, nil
}

// GetPresignedURL returns a presigned GET URL for an S3 object.
func (sc *S3Client) GetPresignedURL(ctx context.Context, bucket, key string, expires time.Duration) (string, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(sc.region))
	if err != nil {
		return "", fmt.Errorf("load AWS config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg)
	presignClient := s3.NewPresignClient(client)
	req, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expires
	})
	if err != nil {
		return "", fmt.Errorf("presign: %w", err)
	}
	return req.URL, nil
}

// GetObject downloads an object from S3 and returns the body reader and content length.
func (sc *S3Client) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, int64, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(sc.region))
	if err != nil {
		return nil, 0, fmt.Errorf("load AWS config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg)
	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("s3 get object: %w", err)
	}
	var length int64
	if out.ContentLength != nil {
		length = *out.ContentLength
	}
	return out.Body, length, nil
}

// DeleteObject deletes an object from S3.
func (sc *S3Client) DeleteObject(ctx context.Context, bucket, key string) error {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(sc.region))
	if err != nil {
		return fmt.Errorf("load AWS config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg)
	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return fmt.Errorf("s3 delete object: %w", err)
	}
	return nil
}
