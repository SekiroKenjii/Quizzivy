package http_test

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"mime/multipart"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/media/application"
	"quizzivy/internal/modules/media/application/command"
	"quizzivy/internal/modules/media/application/query"
	"quizzivy/internal/modules/media/domain"
	mediahttp "quizzivy/internal/modules/media/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
	"strings"
	"testing"
)

func replacementRequest() openapi.ReplaceMediaRequestObject {
	return openapi.ReplaceMediaRequestObject{Id: uuid.New(), Body: multipart.NewReader(strings.NewReader("--test\r\nContent-Disposition: form-data; name=\"file\"; filename=\"ảnh.png\"\r\n\r\nfile\r\n--test--\r\n"), "test")}
}
func TestReplacementTransportStrongErrorsPrecedeWrappedDomainRefusals(t *testing.T) {
	internal := errors.New("private cause")
	for _, tc := range []struct {
		name     string
		cause    error
		internal bool
	}{
		{"known quota", &domain.ReplacementError{Outcome: domain.ReplacementNotCommitted, Cause: domain.ErrQuotaExceeded}, false},
		{"unknown quota", &domain.ReplacementError{Outcome: domain.ReplacementUnknown, Cause: domain.ErrQuotaExceeded}, true},
		{"committed quota", &domain.ReplacementError{Outcome: domain.ReplacementCommitted, Cause: domain.ErrQuotaExceeded}, true},
		{"rollback quota", &domain.ReplacementError{Outcome: domain.ReplacementNotCommitted, Cause: domain.ErrQuotaExceeded, RollbackError: internal}, true},
		{"cleanup quota", &domain.ReplacementCleanupError{Cause: domain.ErrQuotaExceeded, Cleanup: internal}, true},
		{"unknown missing", &domain.ReplacementError{Outcome: domain.ReplacementUnknown, Cause: domain.ErrNotFound}, true},
		{"cleanup kind", &domain.ReplacementCleanupError{Cause: domain.ErrKindMismatch, Cleanup: internal}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := &application.Application{Queries: application.Queries{ReplacementTarget: cqrs.HandlerFunc[query.ReplacementTarget, domain.ReplacementTarget](func(context.Context, query.ReplacementTarget) (domain.ReplacementTarget, error) {
				return domain.ReplacementTarget{}, nil
			})}, Commands: application.Commands{Replace: cqrs.HandlerFunc[command.Replace, domain.ReplaceResult](func(context.Context, command.Replace) (domain.ReplaceResult, error) {
				return domain.ReplaceResult{}, tc.cause
			})}}
			response, err := mediahttp.NewMedia(app).ReplaceMedia(contextAs(t, principals()["an Admin"]), replacementRequest())
			if tc.internal {
				if response != nil || !errors.Is(err, tc.cause) {
					t.Fatalf("strong failure mapped as clean refusal response=%T err=%v", response, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := response.(openapi.ReplaceMedia409JSONResponse); !ok {
				t.Fatalf("known refusal response=%T", response)
			}
		})
	}
}
func TestReplacementTransportPreservesAdminFullScopeAndCallerIdentity(t *testing.T) {
	principal := principals()["an Admin"]
	var preflight access.Scope
	var commandInput command.Replace
	app := &application.Application{Queries: application.Queries{ReplacementTarget: cqrs.HandlerFunc[query.ReplacementTarget, domain.ReplacementTarget](func(_ context.Context, q query.ReplacementTarget) (domain.ReplacementTarget, error) {
		preflight = q.Scope
		return domain.ReplacementTarget{}, nil
	})}, Commands: application.Commands{Replace: cqrs.HandlerFunc[command.Replace, domain.ReplaceResult](func(_ context.Context, cmd command.Replace) (domain.ReplaceResult, error) {
		commandInput = cmd
		return domain.ReplaceResult{Asset: domain.Asset{ID: uuid.NewString(), Kind: domain.KindImage}}, nil
	})}}
	response, err := mediahttp.NewMedia(app).ReplaceMedia(contextAs(t, principal), replacementRequest())
	if err != nil {
		t.Fatal(err)
	}
	want := access.Scope{UserID: principal.UserID, All: true}
	if preflight != want || commandInput.Scope != want || commandInput.UploaderID != principal.UserID {
		t.Fatalf("scope=%+v/%+v actor=%s", preflight, commandInput.Scope, commandInput.UploaderID)
	}
	if _, ok := response.(openapi.ReplaceMedia201JSONResponse); !ok {
		t.Fatalf("response=%T", response)
	}
}
func TestReplacementTransportMissingApplicationAnswersUnavailable(t *testing.T) {
	response, err := mediahttp.NewMedia(nil).ReplaceMedia(context.Background(), replacementRequest())
	if response != nil || !errors.Is(err, httpx.ErrNotImplemented) {
		t.Fatalf("response=%T error=%v", response, err)
	}
}
