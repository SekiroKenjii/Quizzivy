package storage

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"io"
)

// PutImmutable creates a checksum-verified object without replacing existing bytes; exact lost-response retries verify stored checksum, size and MIME type.
func (c *Client) PutImmutable(ctx context.Context, key, contentType string, body io.ReadSeeker, size int64, checksum []byte) error {
	if size <= 0 || len(checksum) != sha256.Size {
		return errors.New("storage: invalid immutable object")
	}
	digest := base64.StdEncoding.EncodeToString(checksum)
	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(c.bucket), Key: aws.String(key), Body: body, ContentType: aws.String(contentType), ContentLength: aws.Int64(size),
		IfNoneMatch: aws.String("*"), ChecksumSHA256: aws.String(digest),
	})
	if err == nil {
		return nil
	}
	var apiError smithy.APIError
	if !errors.As(err, &apiError) || apiError.ErrorCode() != "PreconditionFailed" {
		return fmt.Errorf("storage: create immutable object: %w", err)
	}
	head, err := c.s3.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(c.bucket), Key: aws.String(key), ChecksumMode: types.ChecksumModeEnabled})
	if err != nil {
		return fmt.Errorf("storage: verify immutable object: %w", err)
	}
	if aws.ToInt64(head.ContentLength) != size || aws.ToString(head.ChecksumSHA256) != digest || aws.ToString(head.ContentType) != contentType {
		return errors.New("storage: immutable object identity conflict")
	}
	return nil
}

// Open streams private object bytes to a trusted processor; the caller must close the body and verify its expected size and digest.
func (c *Client) Open(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	result, err := c.s3.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(c.bucket), Key: aws.String(key)})
	if err != nil {
		return nil, 0, fmt.Errorf("storage: read private object: %w", err)
	}
	return result.Body, aws.ToInt64(result.ContentLength), nil
}
