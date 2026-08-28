package api

import (
	"net/http/httptest"
	"testing"
	"time"

	"quizzivy/gen/openapi"
	"quizzivy/internal/media"
)

// TestSignedURLCacheControlMatchesTheTTL pins §11.2's pairing: the cache
// directive and the signature lifetime are the same number.
//
// A cache entry that outlives its signature serves a student a URL that 403s,
// and the §11.2 remedy -- refetch on 403 -- would just return the cached copy
// again. Deriving the header from the TTL is what stops the two drifting; this
// asserts both the derivation and the literal §11.2 asks for.
func TestSignedURLCacheControlMatchesTheTTL(t *testing.T) {
	if got, want := cacheControlForSignedURL, "private, max-age=600"; got != want {
		t.Errorf("Cache-Control = %q, want %q (§11.2)", got, want)
	}
	if seconds := int(media.SignedURLTTL.Seconds()); seconds != 600 {
		t.Errorf("SignedURLTTL = %ds, want 600s (§11.2)", seconds)
	}
}

// TestGetMediaUrlResponseSendsCacheControl proves the header actually reaches
// the wire. The 200 path is unreachable end to end while ReachableByStudent
// denies everything, so without this the header would be untested until the
// version tables land -- and a header that is set but never written is exactly
// the kind of thing that stays broken quietly.
func TestGetMediaUrlResponseSendsCacheControl(t *testing.T) {
	resp := openapi.GetMediaUrl200JSONResponse{
		Body: struct {
			ExpiresAt openapi.Timestamp `json:"expiresAt"`
			Url       string            `json:"url"`
		}{
			ExpiresAt: time.Now().Add(media.SignedURLTTL),
			Url:       "https://example.test/audio/x.mp3?X-Amz-Signature=abc",
		},
		Headers: openapi.GetMediaUrl200ResponseHeaders{CacheControl: cacheControlForSignedURL},
	}

	rec := httptest.NewRecorder()
	if err := resp.VisitGetMediaUrlResponse(rec); err != nil {
		t.Fatalf("writing the response: %v", err)
	}
	if got, want := rec.Header().Get("Cache-Control"), "private, max-age=600"; got != want {
		t.Errorf("Cache-Control on the wire = %q, want %q", got, want)
	}
	if rec.Code != 200 {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}
