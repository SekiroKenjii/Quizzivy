package storage

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"io"
)

const checksumMetadata = "sha256"

// PutImmutable creates a checksum-verified object without replacing existing
// bytes. The body must match size and its SHA-256 checksum before anything is
// sent. The digest travels as object metadata, with Content-MD5 for transport
// integrity, because R2 rejects full-object SHA-256 checksums. A lost-response
// retry of the same object succeeds only if the stored size, digest and MIME
// type match.
func (c *Client) PutImmutable(ctx context.Context, key, contentType string, body io.ReadSeeker, size int64, checksum []byte) error {
	if size <= 0 || len(checksum) != sha256.Size {
		return errors.New("storage: invalid immutable object")
	}
	transfer, err := verifyBody(body, size, checksum)
	if err != nil {
		return err
	}
	digest := hex.EncodeToString(checksum)
	_, err = c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(c.bucket), Key: aws.String(key), Body: body, ContentType: aws.String(contentType), ContentLength: aws.Int64(size),
		IfNoneMatch: aws.String("*"), ContentMD5: aws.String(transfer), Metadata: map[string]string{checksumMetadata: digest},
	})
	if err == nil {
		return nil
	}
	var apiError smithy.APIError
	if !errors.As(err, &apiError) || apiError.ErrorCode() != "PreconditionFailed" {
		return fmt.Errorf("storage: create immutable object: %w", err)
	}
	head, err := c.s3.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(c.bucket), Key: aws.String(key)})
	if err != nil {
		return fmt.Errorf("storage: verify immutable object: %w", err)
	}
	if aws.ToInt64(head.ContentLength) != size || head.Metadata[checksumMetadata] != digest || aws.ToString(head.ContentType) != contentType {
		return errors.New("storage: immutable object identity conflict")
	}
	return nil
}

func verifyBody(body io.ReadSeeker, size int64, checksum []byte) (string, error) {
	sum, transfer := sha256.New(), md5.New()
	n, err := io.Copy(io.MultiWriter(sum, transfer), body)
	if err != nil {
		return "", fmt.Errorf("storage: read immutable object: %w", err)
	}
	if n != size || !bytes.Equal(sum.Sum(nil), checksum) {
		return "", errors.New("storage: immutable object does not match its size and checksum")
	}
	if _, err := body.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("storage: rewind immutable object: %w", err)
	}
	return base64.StdEncoding.EncodeToString(transfer.Sum(nil)), nil
}

// Open streams private object bytes to a trusted processor; the caller must close the body and verify its expected size and digest.
func (c *Client) Open(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	result, err := c.s3.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(c.bucket), Key: aws.String(key)})
	if err != nil {
		return nil, 0, fmt.Errorf("storage: read private object: %w", err)
	}
	return result.Body, aws.ToInt64(result.ContentLength), nil
}
