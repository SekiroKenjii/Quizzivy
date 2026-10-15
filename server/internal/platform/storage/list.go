package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Object is a stored object's key and the time it was last written.
type Object struct {
	Key          string
	LastModified time.Time
}

const listPage = 1000

// List returns at most limit objects whose keys begin with prefix, in key
// order. Objects beyond limit are left out, not an error.
func (c *Client) List(ctx context.Context, prefix string, limit int) ([]Object, error) {
	if limit <= 0 {
		return nil, errors.New("storage: list needs a positive limit")
	}
	var out []Object
	var token *string
	for len(out) < limit {
		page, err := c.s3.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket: aws.String(c.bucket), Prefix: aws.String(prefix),
			MaxKeys: aws.Int32(int32(min(limit-len(out), listPage))), ContinuationToken: token,
		})
		if err != nil {
			return nil, fmt.Errorf("storage: list %s: %w", prefix, err)
		}
		for _, o := range page.Contents {
			out = append(out, Object{Key: aws.ToString(o.Key), LastModified: aws.ToTime(o.LastModified)})
		}
		if !aws.ToBool(page.IsTruncated) {
			break
		}
		token = page.NextContinuationToken
	}
	return out, nil
}
