package http_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/attempts/application"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	attemptshttp "quizzivy/internal/modules/attempts/http"
	identityquery "quizzivy/internal/modules/identity/application/query"
	identitydomain "quizzivy/internal/modules/identity/domain"
	mediamodel "quizzivy/internal/modules/media/application/model"
	mediadomain "quizzivy/internal/modules/media/domain"
	"quizzivy/internal/shared/cqrs"
)

const (
	chartID   = "01935000-0000-7000-8000-00000000000a"
	missingID = "01935000-0000-7000-8000-00000000000b"
	answerID  = "01935000-0000-7000-8000-00000000000c"
)

type staffMedia struct{ read []string }

func (m *staffMedia) Get(_ context.Context, id string) (mediadomain.Asset, error) {
	m.read = append(m.read, id)
	switch id {
	case chartID:
		return mediadomain.Asset{ID: id, Kind: mediadomain.KindImage, MimeType: "image/png", Bytes: 100, OriginalFilename: "chart.png"}, nil
	case answerID:
		return mediadomain.Asset{ID: id, Kind: mediadomain.KindAudio, MimeType: "audio/mpeg", Bytes: 100, OriginalFilename: "answer.mp3"}, nil
	}
	return mediadomain.Asset{}, mediadomain.ErrNotFound
}
func (*staffMedia) SignedURL(_ context.Context, asset mediadomain.Asset) (string, error) {
	return "https://assets.example/staff/" + asset.ID, nil
}
func (*staffMedia) MintForStudent(context.Context, string, string) (mediamodel.SignedURLResult, error) {
	return mediamodel.SignedURLResult{}, mediadomain.ErrForbidden
}
func (*staffMedia) SignedURLTTL() time.Duration { return time.Minute }

func essayWithMedia() domain.ReviewQuestion {
	return domain.ReviewQuestion{
		ID: uuid.NewString(), Type: "essay", Points: 9,
		Prompt:             "Describe the chart.\n\nA bar chart\n\nGone",
		PromptContent:      json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"Describe the chart.","marks":[]}]},{"type":"image","assetId":"` + chartID + `","alt":"A bar chart"},{"type":"image","assetId":"` + missingID + `","alt":"Gone"},{"type":"image","assetId":"` + chartID + `","alt":""}]}`),
		ExplanationContent: json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"Hear a model answer.","marks":[]}]},{"type":"audio","assetId":"` + answerID + `"}]}`),
	}
}

func assertSignedForTheGrader(t *testing.T, got *[]openapi.MediaAsset) {
	t.Helper()
	if got == nil {
		t.Fatal("contentAssets absent, want the chart and the model answer signed")
	}
	want := []string{chartID, answerID}
	if len(*got) != len(want) {
		t.Fatalf("contentAssets %+v, want %v once each, the missing file left out", *got, want)
	}
	for i, asset := range *got {
		if asset.Id.String() != want[i] || asset.Url != "https://assets.example/staff/"+want[i] {
			t.Errorf("asset %d = %s %s, want %s staff-signed", i, asset.Id, asset.Url, want[i])
		}
	}
}

func TestTheReviewSignsWhatTheFrozenPromptAndExplanationShow(t *testing.T) {
	media := &staffMedia{}
	paper := domain.Review{Attempt: domain.Attempt{ID: uuid.NewString(), StudentID: uuid.NewString(), Status: domain.Submitted},
		PublishedAt: time.Now(), Questions: []domain.ReviewQuestion{essayWithMedia()}}
	app := &application.Application{Queries: application.Queries{
		Review:   cqrs.HandlerFunc[query.Review, domain.Review](func(context.Context, query.Review) (domain.Review, error) { return paper, nil }),
		Timeline: cqrs.HandlerFunc[query.Timeline, domain.Timeline](func(context.Context, query.Timeline) (domain.Timeline, error) { return domain.Timeline{}, nil }),
	}}
	students := cqrs.HandlerFunc[identityquery.StudentAccount, identitydomain.Account](func(_ context.Context, q identityquery.StudentAccount) (identitydomain.Account, error) {
		return identitydomain.Account{ID: q.ID}, nil
	})
	response, err := attemptshttp.NewAttempts(app, media, students, nil).GetAttemptForReview(context.Background(), openapi.GetAttemptForReviewRequestObject{Id: uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	body, ok := response.(openapi.GetAttemptForReview200JSONResponse)
	if !ok {
		t.Fatalf("response=%T", response)
	}
	assertSignedForTheGrader(t, body.Questions[0].ContentAssets)
}

func TestTheByQuestionReadSignsWhatTheFrozenPromptAndExplanationShow(t *testing.T) {
	app := &application.Application{Queries: application.Queries{AnswersForQuestion: cqrs.HandlerFunc[query.AnswersForQuestion, domain.ByQuestion](
		func(context.Context, query.AnswersForQuestion) (domain.ByQuestion, error) {
			return domain.ByQuestion{Question: essayWithMedia(), PublishedAt: time.Now(), Number: 1, Count: 1}, nil
		})}}
	response, err := attemptshttp.NewAttempts(app, &staffMedia{}, nil, nil).ListAnswersForQuestion(context.Background(), openapi.ListAnswersForQuestionRequestObject{
		Id: uuid.New(), Params: openapi.ListAnswersForQuestionParams{QuestionId: uuid.New()}})
	if err != nil {
		t.Fatal(err)
	}
	assertSignedForTheGrader(t, response.(openapi.ListAnswersForQuestion200JSONResponse).Question.ContentAssets)
}

func TestAQueuedAnswerCarriesWhatTheFrozenPromptAndExplanationShow(t *testing.T) {
	id := uuid.NewString()
	queue := domain.GradingQueue{AnswersRemaining: 1, Items: []domain.GradingQueueItem{{AttemptID: id, AssignmentID: id, StudentID: id, QuestionNumber: 1, PublishedAt: time.Now(),
		Question: essayWithMedia(), Answer: domain.ReviewAnswer{Payload: []byte(`{"type":"text","value":"An essay."}`)}}}}
	app := &application.Application{Queries: application.Queries{GradingQueue: cqrs.HandlerFunc[query.GradingQueue, domain.GradingQueue](
		func(context.Context, query.GradingQueue) (domain.GradingQueue, error) { return queue, nil })}}
	response, err := attemptshttp.NewAttempts(app, &staffMedia{}, nil, nil).ListGradingQueue(context.Background(), openapi.ListGradingQueueRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	assertSignedForTheGrader(t, response.(openapi.ListGradingQueue200JSONResponse).Items[0].ContentAssets)
}

func TestAQuestionWithoutMediaCarriesNoContentAssets(t *testing.T) {
	media := &staffMedia{}
	app := &application.Application{Queries: application.Queries{AnswersForQuestion: cqrs.HandlerFunc[query.AnswersForQuestion, domain.ByQuestion](
		func(context.Context, query.AnswersForQuestion) (domain.ByQuestion, error) {
			return domain.ByQuestion{Question: domain.ReviewQuestion{ID: uuid.NewString(), Type: "essay", Prompt: "Write", Points: 1}, PublishedAt: time.Now(), Number: 1, Count: 1}, nil
		})}}
	response, err := attemptshttp.NewAttempts(app, media, nil, nil).ListAnswersForQuestion(context.Background(), openapi.ListAnswersForQuestionRequestObject{
		Id: uuid.New(), Params: openapi.ListAnswersForQuestionParams{QuestionId: uuid.New()}})
	if err != nil {
		t.Fatal(err)
	}
	if got := response.(openapi.ListAnswersForQuestion200JSONResponse).Question.ContentAssets; got != nil || len(media.read) != 0 {
		t.Errorf("contentAssets %v, reads %v; want none", got, media.read)
	}
}

func TestAReaderOutsideThePapersScopeIsAnsweredAsBeforeAndSignsNothing(t *testing.T) {
	media := &staffMedia{}
	app := &application.Application{Queries: application.Queries{
		Review: cqrs.HandlerFunc[query.Review, domain.Review](func(context.Context, query.Review) (domain.Review, error) {
			return domain.Review{}, domain.ErrPaperNotFound
		}),
		AnswersForQuestion: cqrs.HandlerFunc[query.AnswersForQuestion, domain.ByQuestion](func(context.Context, query.AnswersForQuestion) (domain.ByQuestion, error) {
			return domain.ByQuestion{}, domain.ErrPaperNotFound
		}),
	}}
	students := cqrs.HandlerFunc[identityquery.StudentAccount, identitydomain.Account](func(_ context.Context, q identityquery.StudentAccount) (identitydomain.Account, error) {
		return identitydomain.Account{ID: q.ID}, nil
	})
	h := attemptshttp.NewAttempts(app, media, students, nil)
	review, err := h.GetAttemptForReview(context.Background(), openapi.GetAttemptForReviewRequestObject{Id: uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := review.(openapi.GetAttemptForReview404JSONResponse); !ok {
		t.Errorf("review=%T, want 404", review)
	}
	byQuestion, err := h.ListAnswersForQuestion(context.Background(), openapi.ListAnswersForQuestionRequestObject{
		Id: uuid.New(), Params: openapi.ListAnswersForQuestionParams{QuestionId: uuid.New()}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := byQuestion.(openapi.ListAnswersForQuestion404JSONResponse); !ok {
		t.Errorf("byQuestion=%T, want 404", byQuestion)
	}
	if len(media.read) != 0 {
		t.Errorf("read %v for a caller outside the scope", media.read)
	}
}
