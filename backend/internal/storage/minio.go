package storage

import (
	"context"
	"fmt"
	"io"
	"strings"

	"idp-platform/backend/internal/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIO struct {
	client    *minio.Client
	bucket    string
	publicURL string
}

func NewMinIO(cfg config.Config) (*MinIO, error) {
	client, err := minio.New(cfg.MinIOEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinIOAccessKey, cfg.MinIOSecretKey, ""),
		Secure: cfg.MinIOUseSSL,
	})
	if err != nil {
		return nil, err
	}

	return &MinIO{
		client:    client,
		bucket:    cfg.MinIOBucket,
		publicURL: strings.TrimRight(cfg.MinIOPublicURL, "/"),
	}, nil
}

func (s *MinIO) PutAvatar(ctx context.Context, userID, contentType, extension string, reader io.Reader, size int64) (string, error) {
	if err := s.ensureBucket(ctx); err != nil {
		return "", err
	}

	objectName := fmt.Sprintf("avatars/%s%s", userID, extension)
	if _, err := s.client.PutObject(ctx, s.bucket, objectName, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	}); err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/%s/%s", s.publicURL, s.bucket, objectName), nil
}

func (s *MinIO) ensureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if !exists {
		if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
			return err
		}
	}

	policy := fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [{
			"Effect": "Allow",
			"Principal": {"AWS": ["*"]},
			"Action": ["s3:GetObject"],
			"Resource": ["arn:aws:s3:::%s/*"]
		}]
	}`, s.bucket)

	return s.client.SetBucketPolicy(ctx, s.bucket, policy)
}
