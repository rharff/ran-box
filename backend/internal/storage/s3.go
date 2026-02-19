package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Client wraps the AWS S3 client for QNAP-compatible operations.
type S3Client struct {
	client *s3.Client
	bucket string
}

// NewS3Client creates a new S3 client configured for QNAP (or any S3-compatible store).
func NewS3Client(endpoint, accessKey, secretKey, region, bucket string, forcePathStyle bool) (*S3Client, error) {
	creds := credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")

	cfg := aws.Config{
		Region:      region,
		Credentials: creds,
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = forcePathStyle // required for QNAP / MinIO
	})

	return &S3Client{
		client: client,
		bucket: bucket,
	}, nil
}

// PutObject uploads data to S3 with key as filename. The key is the SHA-256 hash.
func (s *S3Client) PutObject(ctx context.Context, key string, body io.Reader, sizeBytes int64) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(sizeBytes),
	})
	if err != nil {
		return fmt.Errorf("S3Client.PutObject key=%s: %w", key, err)
	}
	return nil
}

// GetObject fetches an object from S3 and returns a ReadCloser.
// Caller is responsible for closing the returned body.
func (s *S3Client) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("S3Client.GetObject key=%s: %w", key, err)
	}
	return out.Body, nil
}

// DeleteObject removes an object from S3 (used during block garbage collection).
func (s *S3Client) DeleteObject(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("S3Client.DeleteObject key=%s: %w", key, err)
	}
	return nil
}

// ObjectExists checks whether a key already exists in the bucket.
func (s *S3Client) ObjectExists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		// If we get a 404-like error, object does not exist
		return false, nil
	}
	return true, nil
}
