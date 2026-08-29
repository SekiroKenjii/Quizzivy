package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type Config struct {
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	ForcePathStyle  bool
}

type Client struct {
	s3      *s3.Client
	presign *s3.PresignClient
	bucket  string
}

// New builds the client.
//
// The two checksum settings are the load-bearing part, and getting them wrong
// produces a bug that appears ONLY in production. Recent aws-sdk-go-v2 versions
// compute and send `x-amz-sdk-checksum-algorithm` on PutObject by default. R2
// implements a limited set and reports most of those headers as unimplemented,
// so the default gives an upload that works perfectly against MinIO and fails
// against R2. See docs/setup/r2.md.
func New(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.Bucket == "" {
		return nil, errors.New("storage: bucket is required")
	}

	loaded, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID, cfg.SecretAccessKey, "")),
		// Only when the operation requires it -- never opportunistically.
		awsconfig.WithRequestChecksumCalculation(aws.RequestChecksumCalculationWhenRequired),
		awsconfig.WithResponseChecksumValidation(aws.ResponseChecksumValidationWhenRequired),
	)
	if err != nil {
		return nil, fmt.Errorf("storage: load aws config: %w", err)
	}

	client := s3.NewFromConfig(loaded, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.ForcePathStyle
	})

	return &Client{s3: client, presign: s3.NewPresignClient(client), bucket: cfg.Bucket}, nil
}

// Put writes an object. The bucket is private; nothing here makes it readable.
func (c *Client) Put(ctx context.Context, key, contentType string, body io.Reader, size int64) error {
	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	})
	if err != nil {
		return fmt.Errorf("storage: put %s: %w", key, err)
	}
	return nil
}

// Delete removes an object. Used to undo a Put whose database row failed, so
// that a half-finished upload leaves nothing behind.
func (c *Client) Delete(ctx context.Context, key string) error {
	_, err := c.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var notFound *types.NoSuchKey
		if errors.As(err, &notFound) {
			return nil
		}
		return fmt.Errorf("storage: delete %s: %w", key, err)
	}
	return nil
}

// SignedURL mints a time-limited GET URL (§11.2).
//
// Per request, never cached and never stored: the URL IS the capability, so one
// that outlives its purpose is a leak that no later revocation can undo.
func (c *Client) SignedURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	req, err := c.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("storage: sign %s: %w", key, err)
	}
	return req.URL, nil
}
