package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"quizzivy/internal/join"
)

// POST /join/preview is unauthenticated and takes a bearer secret. Two things
// are asserted on the WIRE rather than on the Go struct: what a success body
// contains, and what four different refusals give away.

const fakeClassName = "Lớp Tiếng Anh Giao Tiếp B2"

type fakeJoin struct {
	result join.PreviewResult
	err    error
	seen   []string
}

func (f *fakeJoin) Rotate(context.Context, join.RotateRequest) (join.Rotated, error) {
	return join.Rotated{}, nil
}
func (f *fakeJoin) Revoke(context.Context, join.RevokeRequest) error { return nil }
func (f *fakeJoin) Preview(_ context.Context, code string) (join.PreviewResult, error) {
	f.seen = append(f.seen, code)
	return f.result, f.err
}

func joinRouter(t *testing.T, fake *fakeJoin) http.Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := NewRouter(Deps{DB: fakeDB{}, Join: fake, Tokens: testIssuer(t)}, logger,
		[]string{"https://app.quizzivy.com"}, "")
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	return h
}

func previewFrom(t *testing.T, router http.Handler, ip, code string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/join/preview",
		strings.NewReader(fmt.Sprintf(`{"joinCode":%q}`, code)))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = ip + ":54321"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestTheSuccessBodyHasExactlyThreeKeys(t *testing.T) {
	// §6.5 names them: class name and teacher name, plus the classId the
	// confirm step needs as an idempotency key. Asserted over the decoded JSON
	// rather than the Go type, because the Go type is generated FROM the same
	// contract this is meant to be checking -- a field added to the schema
	// would appear in both and neither would notice.
	fake := &fakeJoin{result: join.PreviewResult{
		Outcome:     join.PreviewOK,
		ClassID:     "01935000-0000-7000-8000-0000000000c1",
		ClassName:   fakeClassName,
		TeacherName: "Thuong",
	}}
	rec := previewFrom(t, joinRouter(t, fake), "203.0.113.1", "K7M3-P9QR")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := map[string]bool{"classId": true, "className": true, "teacherName": true}
	for key := range body {
		if !want[key] {
			t.Errorf("response carries %q, which §6.5 does not permit", key)
		}
	}
	for key := range want {
		if _, ok := body[key]; !ok {
			t.Errorf("response is missing %q", key)
		}
	}
	if len(body) != 3 {
		t.Errorf("response has %d keys, want exactly 3: %s", len(body), rec.Body.String())
	}
}

func TestTheFourRefusalsCarryFourCodesAndNoClassName(t *testing.T) {
	for _, tc := range []struct {
		outcome join.PreviewOutcome
		code    string
	}{
		{join.PreviewInvalid, "JOIN_CODE_INVALID"},
		{join.PreviewRevoked, "JOIN_CODE_REVOKED"},
		{join.PreviewExpired, "JOIN_CODE_EXPIRED"},
		{join.PreviewExhausted, "JOIN_CODE_EXHAUSTED"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			fake := &fakeJoin{result: join.PreviewResult{Outcome: tc.outcome}}
			rec := previewFrom(t, joinRouter(t, fake), "203.0.113.2", "K7M3-P9QR")

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", rec.Code)
			}
			if got := errorCode(t, rec); got != tc.code {
				t.Errorf("error code = %q, want %q", got, tc.code)
			}
		})
	}
}

func TestNoRefusalEchoesAnythingIdentifying(t *testing.T) {
	// Even if the service were ever changed to populate the class on a
	// refusal, the wire must not carry it.
	for _, outcome := range []join.PreviewOutcome{
		join.PreviewInvalid, join.PreviewRevoked, join.PreviewExpired, join.PreviewExhausted,
	} {
		fake := &fakeJoin{result: join.PreviewResult{
			Outcome:     outcome,
			ClassID:     "01935000-0000-7000-8000-0000000000c1",
			ClassName:   fakeClassName,
			TeacherName: "Thuong",
		}}
		rec := previewFrom(t, joinRouter(t, fake), "203.0.113.3", "K7M3-P9QR")
		body := rec.Body.String()
		for _, leak := range []string{fakeClassName, "Thuong", "01935000"} {
			if strings.Contains(body, leak) {
				t.Errorf("outcome %v leaked %q: %s", outcome, leak, body)
			}
		}
	}
}

func TestTheEleventhPreviewInAMinuteFromOneAddressIs429(t *testing.T) {
	// §6.5. Without a limit, 40 bits of entropy is worth probing at scale.
	fake := &fakeJoin{result: join.PreviewResult{Outcome: join.PreviewInvalid}}
	router := joinRouter(t, fake)

	var last *httptest.ResponseRecorder
	for i := range 11 {
		// A different code each time, so only the per-IP bucket can fire.
		last = previewFrom(t, router, "198.51.100.7", fmt.Sprintf("AAAA-BB%02d", i))
	}
	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("11th request status = %d, want 429", last.Code)
	}
	if last.Header().Get("Retry-After") == "" {
		t.Error("429 without Retry-After: the client either gives up or retries immediately")
	}
	if got := errorCode(t, last); got != "RATE_LIMITED" {
		t.Errorf("error code = %q, want RATE_LIMITED", got)
	}
}

func TestTheThirtyFirstAttemptOnOneCodeIs429EvenAcrossAddresses(t *testing.T) {
	// The per-IP bucket does not stop a distributed probe of a single code,
	// which is what a forwarded code invites (R-02). Every request here comes
	// from a different address, so only the per-code bucket can fire.
	fake := &fakeJoin{result: join.PreviewResult{Outcome: join.PreviewInvalid}}
	router := joinRouter(t, fake)

	var last *httptest.ResponseRecorder
	for i := range 31 {
		last = previewFrom(t, router, fmt.Sprintf("198.51.100.%d", i+50), "K7M3-P9QR")
	}
	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("31st request status = %d, want 429", last.Code)
	}
}

func TestRespellingACodeDoesNotBuyAFreshAllowance(t *testing.T) {
	// §6.1 accepts a code with or without the dash and in any case, so
	// `K7M3-P9QR` and `k7m3p9qr` are ONE code. Keyed on the raw body value they
	// would be two buckets, and an attacker gets a new allowance for every
	// spelling of the same secret. The key has to be what the lookup
	// canonicalises to, not what was typed.
	fake := &fakeJoin{result: join.PreviewResult{Outcome: join.PreviewInvalid}}
	router := joinRouter(t, fake)

	spellings := []string{"K7M3-P9QR", "k7m3p9qr", "K7M3P9QR", "k7m3-p9qr", " K7M3 P9QR "}
	var last *httptest.ResponseRecorder
	for i := range 31 {
		// Every request from a different address, cycling through spellings.
		last = previewFrom(t, router, fmt.Sprintf("192.0.2.%d", i+1), spellings[i%len(spellings)])
	}
	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d after 31 attempts across five spellings, want 429 -- "+
			"respelling the code is buying a fresh bucket", last.Code)
	}
}
