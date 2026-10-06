package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/attempts/application"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	attemptshttp "quizzivy/internal/modules/attempts/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
	"testing"
	"time"
)

func queueTransport(out domain.GradingQueue, err error) attemptshttp.Attempts {
	app := &application.Application{Queries: application.Queries{GradingQueue: cqrs.HandlerFunc[query.GradingQueue, domain.GradingQueue](func(context.Context, query.GradingQueue) (domain.GradingQueue, error) { return out, err })}}
	return attemptshttp.NewAttempts(app, nil, nil, nil)
}

func TestGradingQueueTransportKeepsFlatFrozenTeacherFieldsAndNulls(t *testing.T) {
	id := uuid.NewString()
	sample, transcript, comment := "Teacher sample", "Teacher transcript", "Teacher comment"
	prompt := json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"Frozen","marks":[]}]}]}`)
	queue := domain.GradingQueue{AnswersRemaining: 3, StudentsWaiting: 2, Groups: []domain.GradingQueueGroup{{Key: id, Kind: "student", Label: "Nguyễn An", Sub: "Live title", Remaining: 3}}, Items: []domain.GradingQueueItem{{AttemptID: id, AssignmentID: id, AssignmentTitle: "Live title", StudentID: id, StudentName: "Nguyễn An", QuestionNumber: 7, PublishedAt: time.Now(),
		Question: domain.ReviewQuestion{ID: id, Type: "fill_blank", Prompt: "Frozen", PromptContent: prompt, Points: 12345.67, SampleAnswer: &sample, Transcript: &transcript, Options: []domain.ReviewOption{{ID: id, Text: "Correct", IsCorrect: true}}, Blanks: []domain.ReviewBlank{{ID: id, Ordinal: 1, Accepted: []string{"secret"}, CaseSensitive: true}}}, Answer: domain.ReviewAnswer{Payload: []byte(`{"type":"text","value":"Saved answer"}`), GraderComment: &comment}}}}
	response, err := queueTransport(queue, nil).ListGradingQueue(context.Background(), openapi.ListGradingQueueRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	body, ok := response.(openapi.ListGradingQueue200JSONResponse)
	if !ok {
		t.Fatalf("response=%T", response)
	}
	if body.AnswersRemaining != 3 || body.StudentsWaiting != 2 || len(body.Groups) != 1 || body.Groups[0].Key != id || body.Groups[0].Remaining != 3 || len(body.Items) != 1 {
		t.Fatalf("complete flat queue identities/counts=%+v", body)
	}
	if body.Items[0].Options == nil || len(*body.Items[0].Options) != 1 || body.Items[0].Blanks == nil || len(*body.Items[0].Blanks) != 1 {
		t.Fatalf("missing frozen teacher option/blank keys=%+v", body.Items[0])
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	item := wire["items"].([]any)[0].(map[string]any)
	if item["score"] != nil || item["comment"] != comment || item["points"] != 12345.67 || item["questionNumber"] != float64(7) || item["studentName"] != "Nguyễn An" || item["sampleAnswer"] != sample || item["transcript"] != transcript {
		t.Fatalf("flat teacher projection/nulls: %s", raw)
	}
	if _, present := item["score"]; !present {
		t.Fatal("required null score omitted")
	}
	if _, present := item["promptContent"]; !present {
		t.Fatal("rich prompt dropped")
	}
	if item["options"].([]any)[0].(map[string]any)["isCorrect"] != true || item["blanks"].([]any)[0].(map[string]any)["acceptedAnswers"].([]any)[0] != "secret" {
		t.Fatalf("teacher keys dropped: %s", raw)
	}
	for _, private := range []string{"email", "role", "timeZone", "preferences", "phone", "acceptedAnswers", "question"} {
		if _, present := item[private]; present {
			t.Fatalf("unexpected flat/private field %s", private)
		}
	}
	queue.Items[0].Answer.GraderComment = nil
	response, err = queueTransport(queue, nil).ListGradingQueue(context.Background(), openapi.ListGradingQueueRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	raw, err = json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var emptyComment map[string]any
	if err := json.Unmarshal(raw, &emptyComment); err != nil {
		t.Fatal(err)
	}
	if _, present := emptyComment["items"].([]any)[0].(map[string]any)["comment"]; !present {
		t.Fatal("required null comment omitted")
	}
}

func TestGradingQueueTransportPreservesAllScopeAndBothFilters(t *testing.T) {
	for _, all := range []bool{false, true} {
		t.Run(map[bool]string{false: "own", true: "all"}[all], func(t *testing.T) {
			caller := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.TeachingGrading)}
			if all {
				caller.Permissions = access.NewSet(access.TeachingGrading, access.ScopeAll)
			}
			assignment, student := uuid.New(), uuid.New()
			mode := openapi.ListGradingQueueParamsMode("question")
			called := false
			app := &application.Application{Queries: application.Queries{GradingQueue: cqrs.HandlerFunc[query.GradingQueue, domain.GradingQueue](func(_ context.Context, q query.GradingQueue) (domain.GradingQueue, error) {
				called = true
				if q.Scope.All != all || q.Scope.UserID != caller.UserID || q.Mode != "question" || q.AssignmentID == nil || *q.AssignmentID != assignment.String() || q.StudentID == nil || *q.StudentID != student.String() {
					t.Fatalf("scope/filter=%+v", q)
				}
				return domain.GradingQueue{}, nil
			})}}
			_, err := attemptshttp.NewAttempts(app, nil, nil, nil).ListGradingQueue(reachContext(t, caller), openapi.ListGradingQueueRequestObject{Params: openapi.ListGradingQueueParams{Mode: &mode, AssignmentId: &assignment, StudentId: &student}})
			if err != nil || !called {
				t.Fatalf("called=%t err=%v", called, err)
			}
		})
	}
}

func TestGradingQueueTransportErrorsAreHonest(t *testing.T) {
	ctx := context.Background()
	for _, err := range []error{domain.ErrPaperNotFound, domain.ErrGroupContextUnavailable, errors.New("db unavailable")} {
		response, got := queueTransport(domain.GradingQueue{}, err).ListGradingQueue(ctx, openapi.ListGradingQueueRequestObject{})
		switch err {
		case domain.ErrPaperNotFound:
			if _, ok := response.(openapi.ListGradingQueue404JSONResponse); !ok || got != nil {
				t.Fatalf("missing=%T,%v", response, got)
			}
		case domain.ErrGroupContextUnavailable:
			if !errors.Is(got, httpx.ErrNotImplemented) {
				t.Fatalf("shared=%v", got)
			}
		default:
			if !errors.Is(got, err) {
				t.Fatalf("db=%v", got)
			}
		}
	}
	out := domain.GradingQueue{Items: []domain.GradingQueueItem{{Question: domain.ReviewQuestion{Media: &domain.Media{}}}}}
	if _, err := queueTransport(out, nil).ListGradingQueue(ctx, openapi.ListGradingQueueRequestObject{}); !errors.Is(err, httpx.ErrNotImplemented) {
		t.Fatalf("missing media port=%v", err)
	}
	out.Items[0].Question.Media = nil
	out.Items[0].Answer.Payload = []byte("broken")
	if _, err := queueTransport(out, nil).ListGradingQueue(ctx, openapi.ListGradingQueueRequestObject{}); err == nil {
		t.Fatal("malformed saved payload became success")
	}
	if _, err := (attemptshttp.Attempts{}).ListGradingQueue(ctx, openapi.ListGradingQueueRequestObject{}); !errors.Is(err, httpx.ErrNotImplemented) {
		t.Fatalf("nil application=%v", err)
	}
}
