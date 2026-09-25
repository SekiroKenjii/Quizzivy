//go:build integration

package storage_test

import (
	"context"
	"crypto/sha256"
	"io"
	"strings"
	"testing"
)

func TestImmutableStorageRejectsBadDigestAndChangedReplay(t *testing.T) {
	c := newClient(t)
	ctx := context.Background()
	key := testKey(t)
	const payload = "private source evidence"
	hash := sha256.Sum256([]byte(payload))
	if err := c.PutImmutable(ctx, key, "application/json", strings.NewReader("wrong"), 5, hash[:]); err == nil {
		t.Fatal("checksum mismatch accepted")
	}
	for range 2 {
		if err := c.PutImmutable(ctx, key, "application/json", strings.NewReader(payload), int64(len(payload)), hash[:]); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { _ = c.Delete(context.Background(), key) })
	other := sha256.Sum256([]byte("different"))
	if err := c.PutImmutable(ctx, key, "application/json", strings.NewReader("different"), 9, other[:]); err == nil {
		t.Fatal("changed replay replaced bytes")
	}
	if err := c.PutImmutable(ctx, key, "image/png", strings.NewReader(payload), int64(len(payload)), hash[:]); err == nil {
		t.Fatal("changed MIME accepted")
	}
	body, size, err := c.Open(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	got, err := io.ReadAll(body)
	if err != nil || size != int64(len(payload)) || string(got) != payload {
		t.Fatalf("immutable read changed: %v", err)
	}
}
