package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/attempts/application"
	"quizzivy/internal/modules/attempts/application/command"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	attemptshttp "quizzivy/internal/modules/attempts/http"
	mediamodel "quizzivy/internal/modules/media/application/model"
	mediadomain "quizzivy/internal/modules/media/domain"
	testsdomain "quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/cqrs"

	"github.com/google/uuid"
)

type groupMedia struct {
	student string
	asset   string
	denied  bool
	minted  []string
	read    []string
}

func (m *groupMedia) MintForStudent(_ context.Context, student, asset string) (mediamodel.SignedURLResult, error) {
	m.minted = append(m.minted, student+":"+asset)
	if m.denied || student != m.student || asset != m.asset {
		return mediamodel.SignedURLResult{}, mediadomain.ErrForbidden
	}
	return mediamodel.SignedURLResult{URL: "https://assets.example/shared-authorized"}, nil
}
func (m *groupMedia) Get(_ context.Context, id string) (mediadomain.Asset, error) {
	m.read = append(m.read, id)
	return mediadomain.Asset{ID: id, Kind: mediadomain.KindImage, MimeType: "image/png", Bytes: 100, OriginalFilename: "diagram.png"}, nil
}
func (*groupMedia) SignedURL(context.Context, mediadomain.Asset) (string, error) {
	return "", errors.New("teacher signing must not be used")
}
func (*groupMedia) SignedURLTTL() time.Duration { return time.Minute }

func TestAttemptGroupTransportAuthorizesEveryAssetAndOmitsKeys(t *testing.T) {
	for _, start := range []bool{false, true} {
		t.Run(map[bool]string{false: "get", true: "start"}[start], func(t *testing.T) {
			student, attempt, section, question, group, material, asset := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
			session := domain.Session{Attempt: domain.Attempt{ID: attempt.String()}, SessionID: uuid.NewString(),
				Questions: []domain.Question{{ID: question.String(), SectionID: section.String(), Type: "single_choice", Prompt: "Choose", GroupID: group.String()}},
				Groups: []testsdomain.PreviewGroup{{ID: group.String(), SectionID: section.String(), Title: "Shared", QuestionIDs: []string{question.String()}, AssetIDs: []string{asset.String()},
					Stimuli: []testsdomain.GroupStimulus{{ID: material.String(), Title: "Passage", Content: json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"gap","id":"pick","label":"1"}]}]}`), Gaps: []testsdomain.GroupGapBinding{{GapID: "pick", Kind: "question", QuestionID: question.String()}}}},
				}},
			}
			app := &application.Application{
				Queries: application.Queries{Get: cqrs.HandlerFunc[query.Get, domain.Session](func(_ context.Context, q query.Get) (domain.Session, error) {
					if q.StudentID != student.String() || q.AttemptID != attempt.String() {
						t.Fatal("request identity lost")
					}
					return session, nil
				})},
				Commands: application.Commands{StartOrResume: cqrs.HandlerFunc[command.StartOrResume, domain.Session](func(_ context.Context, q command.StartOrResume) (domain.Session, error) {
					if q.StudentID != student.String() {
						t.Fatal("request identity lost")
					}
					return session, nil
				})},
			}
			media := &groupMedia{student: student.String(), asset: asset.String()}
			transport := attemptshttp.NewAttempts(app, media, nil, nil)
			serve := func() (*httptest.ResponseRecorder, error) {
				var failure error
				handler := httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
					return httpx.Principal{UserID: student.String(), Role: "student"}, nil
				})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if start {
						response, err := transport.StartOrResumeAttempt(r.Context(), openapi.StartOrResumeAttemptRequestObject{Id: uuid.New()})
						if err != nil {
							failure = err
							return
						}
						failure = response.VisitStartOrResumeAttemptResponse(w)
					} else {
						response, err := transport.GetAttempt(r.Context(), openapi.GetAttemptRequestObject{Id: attempt})
						if err != nil {
							failure = err
							return
						}
						failure = response.VisitGetAttemptResponse(w)
					}
				}))
				request := httptest.NewRequest(http.MethodGet, "/app/attempts/"+attempt.String(), nil)
				request.Header.Set("Authorization", "Bearer fixture")
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, request)
				return response, failure
			}
			response, err := serve()
			if err != nil {
				t.Fatal(err)
			}
			body := response.Body.String()
			for _, key := range []string{"isCorrect", "acceptedAnswers", "sampleAnswer", "transcript", "transcripts"} {
				if strings.Contains(body, `"`+key+`"`) {
					t.Fatalf("payload leaked %s", key)
				}
			}
			for _, required := range []string{`"groups":[`, `"kind":"question"`, `"questionId":"` + question.String() + `"`, "https://assets.example/shared-authorized"} {
				if !strings.Contains(body, required) {
					t.Fatalf("payload lost %s: %s", required, body)
				}
			}
			if !reflect.DeepEqual(media.minted, []string{student.String() + ":" + asset.String()}) || !reflect.DeepEqual(media.read, []string{asset.String()}) {
				t.Fatalf("incorrect asset authorization: %+v", media)
			}
			media.denied = true
			media.read = nil
			response, err = serve()
			if !errors.Is(err, mediadomain.ErrForbidden) || len(media.read) != 0 || response.Body.Len() != 0 {
				t.Fatalf("authorization failure exposed content: %v, %s", err, response.Body.String())
			}
			app.Queries.Get = cqrs.HandlerFunc[query.Get, domain.Session](func(context.Context, query.Get) (domain.Session, error) {
				return domain.Session{}, domain.ErrGroupContextUnavailable
			})
			app.Commands.StartOrResume = cqrs.HandlerFunc[command.StartOrResume, domain.Session](func(context.Context, command.StartOrResume) (domain.Session, error) {
				return domain.Session{}, domain.ErrGroupContextUnavailable
			})
			response, err = serve()
			if !errors.Is(err, httpx.ErrNotImplemented) || response.Body.Len() != 0 {
				t.Fatalf("missing context reader did not fail explicitly: %v", err)
			}
		})
	}
}
