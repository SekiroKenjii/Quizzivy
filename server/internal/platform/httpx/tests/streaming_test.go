package httpx_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	gen "quizzivy/gen/openapi"
	"quizzivy/internal/platform/httpx"
)

// uploadPattern is the one streaming route in the contract today. Named here so
// a change to the upload path fails these tests rather than silently skipping
// nothing.
const uploadPattern = "POST /admin/media"

func TestStreamingBodyRoutesFindsTheUpload(t *testing.T) {
	spec, err := gen.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	streaming := httpx.StreamingBodyRoutes(spec)
	if _, ok := streaming[uploadPattern]; !ok {
		t.Fatalf("%q not detected as a streaming route; got %v", uploadPattern, streaming)
	}
	if _, ok := streaming["POST /auth/login"]; ok {
		t.Error("a JSON route was classified as streaming")
	}
}

func TestStreamingParametersRemainValidated(t *testing.T) {
	for _, query := range []string{
		"role=learner&uploadId=01900000-0000-7000-8000-000000000001&expectedRevision=1",
		"role=exam&uploadId=01900000-0000-7000-8000-000000000001&expectedRevision=0",
		"role=exam&expectedRevision=1",
	} {
		t.Run(query, func(t *testing.T) {
			reached := false
			h := validatorUnderTest(t)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { reached = true }))
			req := httptest.NewRequest(http.MethodPost, "/admin/imports/01900000-0000-7000-8000-000000000001/sources?"+query, nil)
			req.Pattern = "POST /admin/imports/{id}/sources"
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if reached || rec.Code != http.StatusBadRequest {
				t.Fatalf("invalid upload parameters reached handler: %d", rec.Code)
			}
		})
	}
}

// newUpload builds a multipart body the way a real client does: Go's own
// CreateFormFile labels the part `application/octet-stream`, which the removed
// `encoding.file.contentType` used to reject outright.
func newUpload(t *testing.T, size int) (*http.Request, int) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", "bài nghe.mp3")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(bytes.Repeat([]byte("A"), size)); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	raw := body.Bytes()
	req := httptest.NewRequest(http.MethodPost, "/admin/media", bytes.NewReader(raw))
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Pattern = uploadPattern
	return req, len(raw)
}

func validatorUnderTest(t *testing.T) func(http.Handler) http.Handler {
	t.Helper()
	spec, err := gen.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	v, err := httpx.ValidateRequests(spec)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// TestUploadReachesHandlerWithOctetStreamPart is the defect this fixes: a valid
// upload was answered 400 VALIDATION_FAILED because of the part's Content-Type,
// so the sniffer that actually decides the type never ran.
func TestUploadReachesHandlerWithOctetStreamPart(t *testing.T) {
	var reached bool
	h := validatorUnderTest(t)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusCreated)
	}))

	req, _ := newUpload(t, 1024)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !reached {
		t.Fatalf("validator rejected the upload with %d; the handler never saw it", rec.Code)
	}
}

// TestUploadBodyIsNotBufferedByValidator pins the memory fix. The validator used
// to io.ReadAll the body and decode the file part -- 44.8 MB for a 10 MB upload,
// which is what the temp-file path in media.Service exists to avoid.
func TestUploadBodyIsNotBufferedByValidator(t *testing.T) {
	const bodySize = 10 << 20 // the §11.1 cap
	h := validatorUnderTest(t)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req, wire := newUpload(t, bodySize)

	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	h.ServeHTTP(httptest.NewRecorder(), req)
	runtime.ReadMemStats(&after)

	allocated := after.TotalAlloc - before.TotalAlloc
	if limit := uint64(wire / 4); allocated > limit {
		t.Errorf("validator allocated %.1f MB for a %.1f MB upload (limit %.1f MB); "+
			"it is buffering the body again",
			float64(allocated)/(1<<20), float64(wire)/(1<<20), float64(limit)/(1<<20))
	} else {
		t.Logf("validator allocated %.1f KB for a %.1f MB upload",
			float64(allocated)/(1<<10), float64(wire)/(1<<20))
	}
}
