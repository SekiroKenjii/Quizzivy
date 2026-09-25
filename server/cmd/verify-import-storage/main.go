// Command verify-import-storage checks that a real R2 bucket accepts the Word
// import store's writes before production enables import. It uses the R2_*
// credentials `make verify-r2` reads, never prints them, and removes what it
// writes.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"quizzivy/internal/platform/storage"
)

func main() {
	if !run() {
		os.Exit(1)
	}
}

func run() bool {
	account, key, secret := os.Getenv("R2_ACCOUNT_ID"), os.Getenv("R2_ACCESS_KEY_ID"), os.Getenv("R2_SECRET_ACCESS_KEY")
	bucket := os.Getenv("R2_IMPORT_BUCKET")
	if bucket == "" {
		bucket = "quizzivy-imports"
	}
	if account == "" || key == "" || secret == "" {
		fmt.Println("  ✗ R2_ACCOUNT_ID, R2_ACCESS_KEY_ID and R2_SECRET_ACCESS_KEY must be set in .env (docs/setup/r2.md)")
		return false
	}
	endpoint := "https://" + account + ".r2.cloudflarestorage.com"
	fmt.Printf("Word import storage on R2 (bucket: %s)\n\n", bucket)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	client, err := storage.New(ctx, storage.Config{Endpoint: endpoint, Region: "auto", Bucket: bucket, AccessKeyID: key, SecretAccessKey: secret})
	if err != nil {
		fmt.Println("  ✗ storage client:", err)
		return false
	}
	c := check{ctx: ctx, client: client, endpoint: endpoint, bucket: bucket, ok: true}
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
	defer func() { _ = c.client.Delete(context.WithoutCancel(c.ctx), key) }()
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
	defer func() { _ = c.client.Delete(context.WithoutCancel(c.ctx), key) }()
	url, err := c.client.SignedDownloadURL(c.ctx, key, "de-thi.docx", time.Minute)
	if !c.report(err == nil, "a download URL can be signed", err) {
		return
	}
	got, status, err := get(c.ctx, url)
	c.report(err == nil && status == http.StatusOK && bytes.Equal(got, payload), "the signed download URL returns the source", err)
}

func (c *check) private(key string, payload []byte) {
	got, status, err := get(c.ctx, c.endpoint+"/"+c.bucket+"/"+key)
	c.report(err == nil && !bytes.Contains(got, payload), fmt.Sprintf("an unsigned request gets nothing (HTTP %d): the bucket is private", status), err)
}

func get(ctx context.Context, url string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %s", strings.SplitN(err.Error(), "?", 2)[0])
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
