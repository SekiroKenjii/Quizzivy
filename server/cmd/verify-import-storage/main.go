// Command verify-import-storage checks that a real R2 bucket accepts the Word
// import store's writes before production enables import. It uses the R2_*
// credentials `make verify-r2` reads and never prints them, and it removes what
// it writes and reports whether that succeeded. Whether the bucket is public
// through r2.dev or a custom domain is outside the S3 API; check that with
// wrangler as docs/setup/word-import-worker.md describes.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"quizzivy/internal/platform/storage"
)

func main() {
	if !run() {
		os.Exit(1)
	}
}

func run() bool {
	bucket := flag.String("bucket", "", "the private import bucket; must equal IMPORT_S3_BUCKET in production")
	flag.Parse()
	if *bucket == "" {
		fmt.Println("  ✗ usage: verify-import-storage -bucket <import bucket>")
		return false
	}
	account, key, secret := os.Getenv("R2_ACCOUNT_ID"), os.Getenv("R2_ACCESS_KEY_ID"), os.Getenv("R2_SECRET_ACCESS_KEY")
	if account == "" || key == "" || secret == "" {
		fmt.Println("  ✗ R2_ACCOUNT_ID, R2_ACCESS_KEY_ID and R2_SECRET_ACCESS_KEY must be set in .env (docs/setup/r2.md)")
		return false
	}
	endpoint := "https://" + account + ".r2.cloudflarestorage.com"
	fmt.Printf("Word import storage on R2 (bucket: %s)\n\n", *bucket)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	client, err := storage.New(ctx, storage.Config{Endpoint: endpoint, Region: "auto", Bucket: *bucket, AccessKeyID: key, SecretAccessKey: secret})
	if err != nil {
		fmt.Println("  ✗ storage client:", err)
		return false
	}
	c := check{ctx: ctx, client: client, endpoint: endpoint, bucket: *bucket, ok: true}
	c.immutable()
	c.source()
	return c.ok
}

type check struct {
	ctx              context.Context
	client           *storage.Client
	endpoint, bucket string
	ok               bool
}

func (c *check) report(pass bool, what string, err error) bool {
	if pass {
		fmt.Println("  ✓", what)
		return true
	}
	c.ok = false
	if err != nil {
		fmt.Printf("  ✗ %s: %v\n", what, err)
	} else {
		fmt.Println("  ✗", what)
	}
	return false
}

func (c *check) immutable() {
	key := "_quizzivy-verify/" + nonce() + ".json"
	payload := []byte(`{"verification":"import artifact"}`)
	digest := sha256.Sum256(payload)
	put := func(body []byte, sum []byte, contentType string) error {
		return c.client.PutImmutable(c.ctx, key, contentType, bytes.NewReader(body), int64(len(body)), sum)
	}
	err := put(payload, digest[:], "application/json")
	if !c.report(err == nil, "a create-only write with its SHA-256 is accepted", err) {
		return
	}
	defer c.remove(key)
	err = put(payload, digest[:], "application/json")
	c.report(err == nil, "an identical retry is recognised as already stored", err)
	other := []byte(`{"verification":"changed"}`)
	otherDigest := sha256.Sum256(other)
	c.report(put(other, otherDigest[:], "application/json") != nil, "a changed retry cannot replace the stored bytes", nil)
	body, size, err := c.client.Open(c.ctx, key)
	if c.report(err == nil, "the stored object can be read back", err) {
		got, readErr := io.ReadAll(body)
		_ = body.Close()
		c.report(readErr == nil && size == int64(len(payload)) && bytes.Equal(got, payload), "the bytes read back are the bytes written", readErr)
	}
	c.private(key, payload)
}

func (c *check) source() {
	key := "_quizzivy-verify/" + nonce() + ".docx"
	payload := []byte("verification source")
	err := c.client.Put(c.ctx, key, "application/octet-stream", bytes.NewReader(payload), int64(len(payload)))
	if !c.report(err == nil, "a source upload is accepted", err) {
		return
	}
	defer c.remove(key)
	signed, err := c.client.SignedDownloadURL(c.ctx, key, "de-thi.docx", time.Minute)
	if !c.report(err == nil, "a download URL can be signed", err) {
		return
	}
	got, status, err := get(c.ctx, signed)
	c.report(err == nil && status == http.StatusOK && bytes.Equal(got, payload), "the signed download URL returns the source", err)
}

func (c *check) remove(key string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.ctx), 10*time.Second)
	defer cancel()
	err := c.client.Delete(ctx, key)
	c.report(err == nil, "the verification object is removed", err)
}

func (c *check) private(key string, payload []byte) {
	got, status, err := get(c.ctx, c.endpoint+"/"+c.bucket+"/"+key)
	refused := status == http.StatusBadRequest || status == http.StatusUnauthorized || status == http.StatusForbidden
	c.report(err == nil && refused && !bytes.Contains(got, payload), fmt.Sprintf("the S3 endpoint refuses an unsigned request (HTTP %d)", status), err)
}

func get(ctx context.Context, target string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	var failed *url.Error
	if errors.As(err, &failed) {
		return nil, 0, fmt.Errorf("request failed: %s: %w", failed.Op, failed.Err)
	}
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return body, resp.StatusCode, err
}

func nonce() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
