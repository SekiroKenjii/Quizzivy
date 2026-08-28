package storage_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/storage"
)

// Runs against the real MinIO from docker-compose -- the same client that
// talks to R2 in production (00-overview.md §4.7). A fake would prove the Go
// compiles; this proves the wire format is one an S3 server accepts.

func newClient(t *testing.T) *storage.Client {
	t.Helper()
	endpoint := os.Getenv("S3_ENDPOINT")
	bucket := os.Getenv("S3_BUCKET")
	if endpoint == "" || bucket == "" {
		t.Skip("S3_ENDPOINT / S3_BUCKET are not set; skipping object-store integration test")
	}

	client, err := storage.New(context.Background(), storage.Config{
		Endpoint:        endpoint,
		Region:          os.Getenv("S3_REGION"),
		Bucket:          bucket,
		AccessKeyID:     os.Getenv("S3_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("S3_SECRET_ACCESS_KEY"),
		// MinIO serves buckets as a path; R2 serves them as a subdomain.
		ForcePathStyle: os.Getenv("S3_FORCE_PATH_STYLE") != "false",
	})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	return client
}

func testKey(t *testing.T) string {
	t.Helper()
	nonce := make([]byte, 8)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	return "test/" + hex.EncodeToString(nonce) + ".bin"
}

func TestAnObjectRoundTripsThroughASignedURL(t *testing.T) {
	// The whole media path in one: put a private object, mint a URL for it,
	// and fetch the bytes back.
	//
	// This is also what catches the checksum trap. Recent aws-sdk-go-v2
	// versions send x-amz-sdk-checksum-algorithm on PutObject by default and R2
	// reports it unimplemented -- an upload that works against MinIO and fails
	// in production. The client sets RequestChecksumCalculation to
	// WhenRequired; if that is ever removed, this still passes here and breaks
	// there, which is why docs/setup/r2.md exists and why T-2.3's acceptance
	// asks for one upload against real R2.
	client := newClient(t)
	ctx := context.Background()
	key := testKey(t)
	payload := []byte("nội dung thử nghiệm cho Quizzivy")

	if err := client.Put(ctx, key, "application/octet-stream", bytes.NewReader(payload), int64(len(payload))); err != nil {
		t.Fatalf("Put: %v", err)
	}
	t.Cleanup(func() { _ = client.Delete(context.Background(), key) })

	url, err := client.SignedURL(ctx, key, 10*time.Minute)
	if err != nil {
		t.Fatalf("SignedURL: %v", err)
	}

	resp, err := http.Get(url) //nolint:gosec // the URL is one this test just minted
	if err != nil {
		t.Fatalf("GET signed url: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("signed URL returned %d", resp.StatusCode)
	}
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("round trip changed the bytes")
	}
}

func TestTheBucketIsPrivate(t *testing.T) {
	// §11.2 mints a signed URL per request precisely because the bucket is not
	// readable. If it were public, every signature would be theatre.
	client := newClient(t)
	ctx := context.Background()
	key := testKey(t)

	// len() of the BYTES, not the characters: "bí mật" is six runes and nine
	// bytes, and ContentLength is a byte count.
	const secret = "bí mật"
	if err := client.Put(ctx, key, "text/plain", strings.NewReader(secret), int64(len(secret))); err != nil {
		t.Fatalf("Put: %v", err)
	}
	t.Cleanup(func() { _ = client.Delete(context.Background(), key) })

	signed, err := client.SignedURL(ctx, key, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	// The same URL with the signature stripped.
	unsigned, _, _ := strings.Cut(signed, "?")

	resp, err := http.Get(unsigned) //nolint:gosec // deliberately unsigned
	if err != nil {
		t.Fatalf("GET unsigned: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Fatal("the object is readable without a signature; the bucket is public")
	}
}

func TestASignatureExpires(t *testing.T) {
	// §11.2's ten minutes only means something if the deadline is enforced by
	// the server rather than by the client choosing not to reuse the URL.
	client := newClient(t)
	ctx := context.Background()
	key := testKey(t)

	const expiring = "hết hạn"
	if err := client.Put(ctx, key, "text/plain", strings.NewReader(expiring), int64(len(expiring))); err != nil {
		t.Fatalf("Put: %v", err)
	}
	t.Cleanup(func() { _ = client.Delete(context.Background(), key) })

	// One second, then wait it out.
	url, err := client.SignedURL(ctx, key, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Second)

	resp, err := http.Get(url) //nolint:gosec // the point is that this fails
	if err != nil {
		t.Fatalf("GET expired: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Error("an expired signature still worked")
	}
}

func TestDeletingSomethingThatIsNotThereSucceeds(t *testing.T) {
	// Delete undoes a Put whose row failed. If it errored on a missing key, the
	// cleanup path would report a failure for having nothing to clean.
	client := newClient(t)
	if err := client.Delete(context.Background(), testKey(t)); err != nil {
		t.Errorf("Delete on a missing key: %v", err)
	}
}
