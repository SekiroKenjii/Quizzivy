package httpx_test

import (
	"encoding/json"
	"errors"
	"github.com/getkin/kin-openapi/openapi3"
	"io"
	"net/http"
	"net/http/httptest"
	"quizzivy/internal/platform/httpx"
	"strings"
	"testing"
)

func statusSpec(t *testing.T) *openapi3.T {
	t.Helper()
	raw := `{"openapi":"3.0.3","info":{"title":"cap","version":"1"},"paths":{"/budget":{"patch":{"requestBody":{"content":{"application/json":{"schema":{"type":"object"}}}},"responses":{"400":{"description":"bad"},"413":{"description":"large"}}}}}}`
	doc, err := openapi3.NewLoader().LoadFromData([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}
func TestBodyLimitStatusMetadataIsStrictAndOldFallbackRemains(t *testing.T) {
	for _, raw := range []any{400, 413} {
		spec := statusSpec(t)
		op := spec.Paths.Value("/budget").Patch
		op.Extensions = map[string]any{"x-max-body-bytes": 8192, "x-body-limit-status": raw}
		limits, err := httpx.RequestBodyLimits(spec)
		if err != nil || limits["PATCH /budget"].ExceededStatus != raw || limits["PATCH /budget"].Bytes != 8192 {
			t.Fatalf("limits=%v error=%v", limits, err)
		}
	}
	for _, raw := range []any{399, 414, "400", true, nil, 400.5} {
		spec := statusSpec(t)
		op := spec.Paths.Value("/budget").Patch
		op.Extensions = map[string]any{"x-max-body-bytes": 8192, "x-body-limit-status": raw}
		if _, err := httpx.RequestBodyLimits(spec); err == nil {
			t.Fatalf("bad status%v accepted", raw)
		}
	}
	for _, raw := range []any{nil, 0, -1, "8192", 1.5} {
		spec := statusSpec(t)
		spec.Paths.Value("/budget").Patch.Extensions = map[string]any{"x-max-body-bytes": raw, "x-body-limit-status": 400}
		if _, err := httpx.RequestBodyLimits(spec); err == nil {
			t.Fatalf("bad budget%v accepted", raw)
		}
	}
	for _, kind := range []string{"no body", "no response", "streaming"} {
		spec := statusSpec(t)
		op := spec.Paths.Value("/budget").Patch
		op.Extensions = map[string]any{"x-max-body-bytes": 8192, "x-body-limit-status": 400}
		switch kind {
		case "no body":
			op.RequestBody = nil
		case "no response":
			op.Responses.Delete("400")
		case "streaming":
			op.RequestBody.Value.Content = openapi3.Content{"multipart/form-data": op.RequestBody.Value.Content["application/json"]}
		}
		if _, err := httpx.RequestBodyLimits(spec); err == nil {
			t.Fatalf("%s accepted", kind)
		}
	}
	for _, raw := range []any{nil, "invalid", -1} {
		spec := statusSpec(t)
		spec.Paths.Value("/budget").Patch.Extensions = map[string]any{"x-max-body-bytes": raw}
		limits, err := httpx.RequestBodyLimits(spec)
		if err != nil || len(limits) != 0 {
			t.Fatalf("old fallback=%v %v", limits, err)
		}
	}
	spec := statusSpec(t)
	spec.Paths.Value("/budget").Patch.Extensions = map[string]any{"x-max-body-bytes": 123}
	limits, err := httpx.RequestBodyLimits(spec)
	if err != nil || limits["PATCH /budget"].ExceededStatus != 413 {
		t.Fatalf("default status=%v %v", limits, err)
	}
}

type capReader struct {
	io.Reader
	read   int
	closed bool
}

func (r *capReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.read += n
	return n, err
}
func (r *capReader) Close() error { r.closed = true; return nil }

type interruptedBody struct{}

func (interruptedBody) Read([]byte) (int, error) { return 0, errors.New("interrupted") }
func (interruptedBody) Close() error             { return nil }

func TestSelectedRawBodyCapReadsAtMostLimitPlusOneAndReplaysExactBytes(t *testing.T) {
	limits := map[string]httpx.RequestBodyLimit{"PATCH /budget": {Bytes: 8192, ExceededStatus: 400}}
	for _, length := range []int{8192, 8193, 20000} {
		for _, unknown := range []bool{false, true} {
			t.Run(strings.Join([]string{http.StatusText(length), map[bool]string{true: "unknown", false: "known"}[unknown]}, "/"), func(t *testing.T) {
				body := strings.Repeat(" ", length-2) + "{}"
				reader := &capReader{Reader: strings.NewReader(body)}
				calls := 0
				h := httpx.LimitRequestBody(nil, 100, limits)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					raw, err := io.ReadAll(r.Body)
					if err != nil || string(raw) != body {
						t.Fatalf("replay=%q %v", raw, err)
					}
					w.WriteHeader(204)
				}))
				req := httptest.NewRequest("PATCH", "/budget", reader)
				req.Pattern = "PATCH /budget"
				req.ContentLength = int64(length)
				if unknown {
					req.ContentLength = -1
				}
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)
				if length == 8192 {
					if rec.Code != 204 || calls != 1 || !reader.closed {
						t.Fatalf("exact: %d calls%d closed%v", rec.Code, calls, reader.closed)
					}
				} else {
					if rec.Code != 400 || calls != 0 {
						t.Fatalf("oversize: %d calls%d", rec.Code, calls)
					}
					var envelope struct{ Error struct{ Code string } }
					if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil || envelope.Error.Code != "VALIDATION_FAILED" {
						t.Fatalf("envelope=%s", rec.Body.String())
					}
					if !unknown && reader.read != 0 {
						t.Fatal("declared oversize read body")
					}
				}
				if reader.read > 8193 {
					t.Fatalf("unbounded read=%d", reader.read)
				}
			})
		}
	}
}

func TestBodyStatusPreservesIncompleteDefaultAndStreamingBehavior(t *testing.T) {
	h := httpx.LimitRequestBody(nil, 10, nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("rejected request reached handler") }))
	req := httptest.NewRequest("POST", "/other", strings.NewReader(strings.Repeat("x", 11)))
	req.Pattern = "POST /other"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 413 {
		t.Fatalf("default=%d", rec.Code)
	}
	req = httptest.NewRequest("POST", "/other", interruptedBody{})
	req.Pattern = "POST /other"
	req.ContentLength = -1
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 408 {
		t.Fatalf("incomplete=%d", rec.Code)
	}
	reader := &capReader{Reader: strings.NewReader(strings.Repeat("x", 20))}
	req = httptest.NewRequest("POST", "/stream", reader)
	req.Pattern = "POST /stream"
	rec = httptest.NewRecorder()
	stream := httpx.LimitRequestBody(map[string]struct{}{"POST /stream": {}}, 10, map[string]httpx.RequestBodyLimit{"POST /stream": {Bytes: 12, ExceededStatus: 413}})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if reader.read != 0 {
			t.Fatal("stream eagerly read")
		}
		raw, err := io.ReadAll(r.Body)
		if len(raw) != 12 || err == nil {
			t.Fatalf("stream budget=%d %v", len(raw), err)
		}
		w.WriteHeader(204)
	}))
	stream.ServeHTTP(rec, req)
	if rec.Code != 204 {
		t.Fatal(rec.Code)
	}
}
