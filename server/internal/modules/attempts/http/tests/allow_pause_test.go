package http_test

import (
	"context"
	"net/http"
	"testing"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/attempts/application"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	attemptshttp "quizzivy/internal/modules/attempts/http"
	"quizzivy/internal/shared/cqrs"

	"github.com/google/uuid"
)

func TestTheStudentsAudioPolicyAlwaysCarriesAllowPause(t *testing.T) {
	student, attempt, section, asset := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	listening := func(id uuid.UUID, pause bool) domain.Question {
		return domain.Question{
			ID: id.String(), SectionID: section.String(), Type: "short_answer", Prompt: "Nghe",
			Media: &domain.Media{ID: asset.String(), Kind: "audio", MimeType: "audio/mpeg", Filename: "nghe.mp3", Bytes: 100},
			Audio: &domain.AudioPolicy{AllowSeek: false, ShowTranscriptAfterSubmit: true, AllowPause: pause},
		}
	}
	session := domain.Session{
		Attempt: domain.Attempt{ID: attempt.String()}, SessionID: uuid.NewString(),
		Questions: []domain.Question{listening(uuid.New(), true), listening(uuid.New(), false)},
	}
	app := &application.Application{Queries: application.Queries{Get: cqrs.HandlerFunc[query.Get, domain.Session](func(context.Context, query.Get) (domain.Session, error) {
		return session, nil
	})}}
	transport := attemptshttp.NewAttempts(app, &groupMedia{student: student.String(), asset: asset.String()}, nil, nil)

	questions := serveAs(t, student, func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.GetAttempt(ctx, openapi.GetAttemptRequestObject{Id: attempt})
		if err != nil {
			return err
		}
		return response.VisitGetAttemptResponse(w)
	})
	if len(questions) != 2 {
		t.Fatalf("questions = %v, want two", questions)
	}
	for i, want := range []bool{true, false} {
		policy, ok := questions[i]["audio"].(map[string]any)
		if !ok {
			t.Fatalf("question %d carries no audio policy: %v", i, questions[i])
		}
		if got, present := policy["allowPause"]; !present || got != want {
			t.Fatalf("question %d audio policy = %v, want allowPause %v", i, policy, want)
		}
		for _, key := range []string{"maxPlays", "allowSeek", "showTranscriptAfterSubmit"} {
			if _, present := policy[key]; !present {
				t.Fatalf("question %d audio policy lacks %s: %v", i, key, policy)
			}
		}
	}
}
