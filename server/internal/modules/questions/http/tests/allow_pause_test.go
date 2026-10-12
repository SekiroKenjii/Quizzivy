package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/questions/application"
	questions "quizzivy/internal/modules/questions/domain"
	questionshttp "quizzivy/internal/modules/questions/http"
	"quizzivy/internal/shared/access"
	"testing"

	"github.com/google/uuid"
)

type pauseRepository struct {
	questions.Repository
	written []questions.WriteInput
	stored  *bool
}

func (r *pauseRepository) Create(_ context.Context, in questions.WriteInput) (questions.Question, error) {
	r.written = append(r.written, in)
	policy := *in.Input.Audio
	policy.AllowPause = r.stored
	return questions.Question{
		ID: uuid.NewString(), Type: in.Input.Type, Prompt: in.Input.Prompt, Points: in.Input.Points,
		MediaAssetID: in.Input.MediaAssetID, MediaAssetKind: in.MediaAssetKind, Audio: &policy,
	}, nil
}

func (r *pauseRepository) Update(ctx context.Context, in questions.WriteInput) (questions.Question, error) {
	return r.Create(ctx, in)
}

func pauseBody[T any](t *testing.T, audio string) *T {
	t.Helper()
	raw := `{"type":"short_answer","prompt":"Nghe","points":1,"tags":[],"mediaAssetId":"` + uuid.NewString() + `","audio":` + audio + `}`
	var body T
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatal(err)
	}
	return &body
}

func policyOf(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	policy, ok := body["audio"].(map[string]any)
	if !ok {
		t.Fatalf("response carries no audio policy: %v", body)
	}
	return policy
}

func TestARequestsAllowPauseReachesTheStoreAsSent(t *testing.T) {
	principal := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentQuestionsWrite)}
	ctx := contextAs(t, principal)
	repo := &pauseRepository{}
	handler := questionshttp.NewQuestions(application.New(repo, fixedKind("audio")), nil)
	const policy = `{"maxPlays":2,"allowSeek":false,"showTranscriptAfterSubmit":true`

	for name, tc := range map[string]struct {
		audio string
		want  *bool
	}{
		"absent": {policy + `}`, nil},
		"off":    {policy + `,"allowPause":false}`, new(false)},
		"on":     {policy + `,"allowPause":true}`, new(true)},
	} {
		t.Run(name, func(t *testing.T) {
			repo.written = nil
			if _, err := handler.CreateQuestion(ctx, openapi.CreateQuestionRequestObject{Body: pauseBody[openapi.CreateQuestionJSONRequestBody](t, tc.audio)}); err != nil {
				t.Fatal(err)
			}
			if _, err := handler.UpdateQuestion(ctx, openapi.UpdateQuestionRequestObject{Id: uuid.New(), Body: pauseBody[openapi.UpdateQuestionJSONRequestBody](t, tc.audio)}); err != nil {
				t.Fatal(err)
			}
			if len(repo.written) != 2 {
				t.Fatalf("%d writes reached the store, want 2", len(repo.written))
			}
			for _, in := range repo.written {
				got := in.Input.Audio.AllowPause
				if (got == nil) != (tc.want == nil) || (got != nil && *got != *tc.want) {
					t.Fatalf("the store was handed allowPause %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestTheTeachersAnswerAlwaysCarriesAllowPause(t *testing.T) {
	principal := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentQuestionsWrite)}
	ctx := contextAs(t, principal)
	for name, tc := range map[string]struct {
		stored *bool
		want   bool
	}{
		"stored off":   {new(false), false},
		"stored on":    {new(true), true},
		"stored unset": {nil, true},
	} {
		t.Run(name, func(t *testing.T) {
			handler := questionshttp.NewQuestions(application.New(&pauseRepository{stored: tc.stored}, fixedKind("audio")), nil)
			response, err := handler.CreateQuestion(ctx, openapi.CreateQuestionRequestObject{Body: pauseBody[openapi.CreateQuestionJSONRequestBody](t, `{"maxPlays":null,"allowSeek":true,"showTranscriptAfterSubmit":false}`)})
			if err != nil {
				t.Fatal(err)
			}
			status, body := visitJSON(t, response.VisitCreateQuestionResponse)
			if status != http.StatusCreated {
				t.Fatalf("answered %d %v", status, body)
			}
			if got := policyOf(t, body)["allowPause"]; got != tc.want {
				t.Fatalf("allowPause = %v, want %v in %v", got, tc.want, body["audio"])
			}
		})
	}
}

func TestStoredInputCarriesAllowPause(t *testing.T) {
	for name, tc := range map[string]struct {
		pause *bool
		want  bool
	}{
		"off": {new(false), false}, "on": {new(true), true}, "unset": {nil, true},
	} {
		t.Run(name, func(t *testing.T) {
			wire, err := questionshttp.ToAPIInput(questions.Input{
				Type: questions.ShortAnswer, Prompt: "Nghe", Points: "1",
				Audio: &questions.AudioPolicy{AllowPause: tc.pause},
			})
			if err != nil {
				t.Fatal(err)
			}
			if wire.Audio == nil || wire.Audio.AllowPause == nil || *wire.Audio.AllowPause != tc.want {
				t.Fatalf("the stored shape of a group member carries %+v, want allowPause %v", wire.Audio, tc.want)
			}
		})
	}
}
