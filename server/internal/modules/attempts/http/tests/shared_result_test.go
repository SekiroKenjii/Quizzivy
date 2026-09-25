package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/attempts/application"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	attemptshttp "quizzivy/internal/modules/attempts/http"
	testsdomain "quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/cqrs"

	"github.com/google/uuid"
)

func TestSharedResultAuthorizesAssetsAndCarriesOnlyReleasedTranscripts(t *testing.T) {
	student, attempt, group, section, asset, recording := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	app := &application.Application{Queries: application.Queries{
		Result: cqrs.HandlerFunc[query.Result, domain.Result](func(_ context.Context, q query.Result) (domain.Result, error) {
			if q.StudentID != student.String() || q.AttemptID != attempt.String() {
				t.Fatal("result lost request identity")
			}
			return domain.Result{Attempt: domain.Attempt{ID: attempt.String()}, SharedContext: &domain.SharedReviewContext{
				Groups:      []testsdomain.PreviewGroup{{ID: group.String(), SectionID: section.String(), Title: "Frozen material", AssetIDs: []string{asset.String()}}},
				Transcripts: map[string]string{recording.String(): "Released transcript"}, AudioPlays: map[string]int{recording.String(): 3},
			}}, nil
		}),
	}}
	for _, denied := range []bool{false, true} {
		media := &groupMedia{student: student.String(), asset: asset.String(), denied: denied}
		transport := attemptshttp.NewAttempts(app, media, nil, nil)
		var failure error
		handler := httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
			return httpx.Principal{UserID: student.String(), Role: "student"}, nil
		})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response, err := transport.GetAttemptResult(r.Context(), openapi.GetAttemptResultRequestObject{Id: attempt})
			if err != nil {
				failure = err
				return
			}
			failure = response.VisitGetAttemptResultResponse(w)
		}))
		request := httptest.NewRequest(http.MethodGet, "/app/attempts/"+attempt.String()+"/result", nil)
		request.Header.Set("Authorization", "Bearer fixture")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if len(media.minted) != 1 || media.minted[0] != student.String()+":"+asset.String() {
			t.Fatal("shared result used teacher signing or skipped ownership")
		}
		if denied {
			if failure == nil || len(media.read) != 0 || response.Body.Len() != 0 {
				t.Fatal("denied media produced a partial shared result")
			}
			continue
		}
		if failure != nil {
			t.Fatal(failure)
		}
		body := response.Body.String()
		for _, expected := range []string{"sharedContext", "Frozen material", "Released transcript", "audioPlays", "shared-authorized"} {
			if !strings.Contains(body, expected) {
				t.Fatalf("missing %s in result", expected)
			}
		}
		for _, secret := range []string{"isCorrect", "sampleAnswer", "acceptedAnswers", "teacherNote"} {
			if strings.Contains(body, secret) {
				t.Fatalf("result leaked %s", secret)
			}
		}
	}
}
