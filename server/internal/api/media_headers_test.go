package api

import (
	"net/http/httptest"
	"testing"
	"time"

	"quizzivy/gen/openapi"
	"quizzivy/internal/media"
)

// §11.2 pairs the cache directive with the signature lifetime. A cache entry
// that outlives its signature serves a student a URL that 403s, and §11.2's
// remedy -- refetch on 403 -- would just return the cached copy again.

func TestSignedURLCacheControlIsDerivedFromTheTTL(t *testing.T) {
	if got, want := cacheControlForSignedURL(10*time.Minute), "private, max-age=600"; got != want {
		t.Errorf("Cache-Control = %q, want %q (§11.2)", got, want)
	}
	if seconds := int(media.DefaultSignedURLTTL.Seconds()); seconds != 600 {
		t.Errorf("DefaultSignedURLTTL = %ds, want 600s (§11.2)", seconds)
	}
	// The point of deriving it: a configured TTL moves the header with it,
	// rather than leaving two independent copies of "ten minutes".
	if got, want := cacheControlForSignedURL(90*time.Second), "private, max-age=90"; got != want {
		t.Errorf("a configured TTL gave %q, want %q", got, want)
	}
}

func TestGetMediaUrlResponseSendsCacheControl(t *testing.T) {
	// The 200 path is unreachable end to end while ReachableByStudent denies
	// everything, so without this the header would go untested until the
	// version tables land -- and a header that is set but never written is
	// exactly the kind of thing that stays broken quietly.
	resp := openapi.GetMediaUrl200JSONResponse{
		Body: struct {
			ExpiresAt openapi.Timestamp `json:"expiresAt"`
			Url       string            `json:"url"`
		}{
			ExpiresAt: time.Now().Add(media.DefaultSignedURLTTL),
			Url:       "https://example.test/audio/x.mp3?X-Amz-Signature=abc",
		},
		Headers: openapi.GetMediaUrl200ResponseHeaders{
			CacheControl: cacheControlForSignedURL(media.DefaultSignedURLTTL),
		},
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

// TestListMediaResponseSendsCacheControl covers the endpoint that hands out
// MANY of the same capability. It carries a signed URL per item with the same
// ten-minute life, and used to name no cache policy at all -- so a 200 to a GET
// was heuristically cacheable, and a library page a teacher left open kept
// showing URLs that had stopped working.
func TestListMediaResponseSendsCacheControl(t *testing.T) {
	var resp openapi.ListMedia200JSONResponse
	resp.Headers.CacheControl = cacheControlForSignedURLList

	rec := httptest.NewRecorder()
	if err := resp.VisitListMediaResponse(rec); err != nil {
		t.Fatalf("writing the response: %v", err)
	}
	// no-store rather than max-age: unlike a single asset URL there is nothing
	// worth re-serving, since the library changes on every upload and delete.
	if got, want := rec.Header().Get("Cache-Control"), "private, no-store"; got != want {
		t.Errorf("Cache-Control on the wire = %q, want %q", got, want)
	}
}

// TestBothSignedURLEndpointsStateAPolicy is the rule rather than the two
// instances of it: an endpoint returning a signed URL must say something about
// caching. The list opted out silently once already.
func TestBothSignedURLEndpointsStateAPolicy(t *testing.T) {
	for _, tc := range []struct {
		name  string
		write func(*httptest.ResponseRecorder) error
	}{
		{"GET /app/media/{assetId}/url", func(rec *httptest.ResponseRecorder) error {
			return openapi.GetMediaUrl200JSONResponse{
				Headers: openapi.GetMediaUrl200ResponseHeaders{
					CacheControl: cacheControlForSignedURL(media.DefaultSignedURLTTL),
				},
			}.VisitGetMediaUrlResponse(rec)
		}},
		{"GET /admin/media", func(rec *httptest.ResponseRecorder) error {
			var r openapi.ListMedia200JSONResponse
			r.Headers.CacheControl = cacheControlForSignedURLList
			return r.VisitListMediaResponse(rec)
		}},
	} {
		rec := httptest.NewRecorder()
		if err := tc.write(rec); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if rec.Header().Get("Cache-Control") == "" {
			t.Errorf("%s returns a signed URL and states no cache policy", tc.name)
		}
	}
}
