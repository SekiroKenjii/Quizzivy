package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/tests/application"
	"quizzivy/internal/modules/tests/application/query"
	"quizzivy/internal/modules/tests/domain"
	testshttp "quizzivy/internal/modules/tests/http"
	"quizzivy/internal/shared/cqrs"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
)

func diffApp(result query.DiffResult, err error, seen *query.Diff) *application.Application {
	return &application.Application{Queries: application.Queries{
		Diff: cqrs.HandlerFunc[query.Diff, query.DiffResult](func(_ context.Context, q query.Diff) (query.DiffResult, error) {
			if seen != nil {
				*seen = q
			}
			return result, err
		}),
	}}
}

func callDiff(t *testing.T, app *application.Application, version int, against string) *httptest.ResponseRecorder {
	t.Helper()
	out, err := testshttp.NewTests(app, nil).GetTestVersionDiff(contextAs(t, publishPrincipal()), openapi.GetTestVersionDiffRequestObject{
		Id: uuid.New(), Version: version, Params: openapi.GetTestVersionDiffParams{Against: against},
	})
	if err != nil {
		t.Fatalf("GetTestVersionDiff: %v", err)
	}
	rec := httptest.NewRecorder()
	if err := out.VisitGetTestVersionDiffResponse(rec); err != nil {
		t.Fatal(err)
	}
	return rec
}

func TestTheDiffAnswersEveryKindWithItsOwnParams(t *testing.T) {
	published := time.Date(2026, 10, 8, 3, 4, 5, 0, time.UTC)
	added, removed, changed, answered := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	var seen query.Diff
	result := query.DiffResult{
		From: &domain.DiffSide{Version: 1, PublishedAt: published},
		To:   domain.DiffSide{Draft: true},
		Changes: []domain.Change{
			{Kind: domain.ChangeAdded, Number: 5, QuestionID: added, Prompt: "Câu mới"},
			{Kind: domain.ChangeRemoved, Number: 2, QuestionID: removed, Prompt: "Câu cũ"},
			{Kind: domain.ChangeChanged, Number: 4, QuestionID: changed, Fields: []domain.ChangedField{domain.FieldPrompt, domain.FieldContext}},
			{Kind: domain.ChangeAnswer, Number: 1, QuestionID: answered, AnswerFrom: []string{"B"}, AnswerTo: []string{"A", "C"}},
			{Kind: domain.ChangePoints, PointsFrom: "10.00", PointsTo: "10.50"},
		},
	}
	rec := callDiff(t, diffApp(result, nil, &seen), 1, "draft")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if seen.Version != 1 || seen.Against.Kind != domain.AgainstDraft {
		t.Errorf("the query got %+v, want version 1 against the draft", seen)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(body["to"], map[string]any{"kind": "draft"}) {
		t.Errorf("to = %v, want the draft with no version", body["to"])
	}
	if !reflect.DeepEqual(body["from"], map[string]any{"kind": "version", "version": float64(1), "publishedAt": "2026-10-08T03:04:05Z"}) {
		t.Errorf("from = %v", body["from"])
	}
	want := []any{
		map[string]any{"kind": "added", "questionNumber": float64(5), "questionId": added, "params": map[string]any{"prompt": "Câu mới"}},
		map[string]any{"kind": "removed", "questionNumber": float64(2), "questionId": removed, "params": map[string]any{"prompt": "Câu cũ"}},
		map[string]any{"kind": "changed", "questionNumber": float64(4), "questionId": changed, "params": map[string]any{"fields": []any{"prompt", "context"}}},
		map[string]any{"kind": "answer", "questionNumber": float64(1), "questionId": answered, "params": map[string]any{"answerFrom": []any{"B"}, "answerTo": []any{"A", "C"}}},
		map[string]any{"kind": "points", "params": map[string]any{"pointsFrom": float64(10), "pointsTo": 10.5}},
	}
	if !reflect.DeepEqual(body["changes"], want) {
		t.Errorf("changes = %v\nwant     %v", body["changes"], want)
	}
}

func TestTheFirstVersionHasNoFromSideAndAnEmptyListIsNotNull(t *testing.T) {
	result := query.DiffResult{To: domain.DiffSide{Version: 1, PublishedAt: time.Now()}, Changes: []domain.Change{}}
	rec := callDiff(t, diffApp(result, nil, nil), 1, "previous")
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if from, present := body["from"]; !present || from != nil {
		t.Errorf("from = %v (present %v), want an explicit null", from, present)
	}
	if changes, ok := body["changes"].([]any); !ok || len(changes) != 0 {
		t.Errorf("changes = %v, want an empty array", body["changes"])
	}
}

func TestTheDiffRefusesTheVersionItselfAndAnythingElseAsATarget(t *testing.T) {
	for name, c := range map[string]struct {
		version int
		against string
	}{
		"the version itself": {3, "3"},
		"zero":               {3, "0"},
		"a word":             {3, "latest"},
		"a negative":         {3, "-1"},
		"empty":              {3, ""},
	} {
		t.Run(name, func(t *testing.T) {
			var seen query.Diff
			rec := callDiff(t, diffApp(query.DiffResult{}, nil, &seen), c.version, c.against)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status %d, want 400: %s", rec.Code, rec.Body)
			}
			var body struct {
				Error struct {
					Code    string         `json:"code"`
					Details map[string]any `json:"details"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Error.Code != "VALIDATION_FAILED" || body.Error.Details["against"] == nil {
				t.Errorf("error = %+v, want VALIDATION_FAILED naming against", body.Error)
			}
			if seen.TestID != "" {
				t.Errorf("the query ran for a refused target: %+v", seen)
			}
		})
	}
}

func TestTheDiffAnswersNotFoundAndAnUnreadableDraft(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code int
		body string
	}{
		{"missing", domain.ErrNotFound, http.StatusNotFound, "NOT_FOUND"},
		{"unreadable draft", domain.ErrDraftUnreadable, http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := callDiff(t, diffApp(query.DiffResult{}, c.err, nil), 1, "draft")
			if rec.Code != c.code {
				t.Fatalf("status %d, want %d: %s", rec.Code, c.code, rec.Body)
			}
			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Error.Code != c.body {
				t.Errorf("code %q, want %q", body.Error.Code, c.body)
			}
		})
	}
}

func TestAnyOtherFailureOfTheDiffIsNotMasked(t *testing.T) {
	_, err := testshttp.NewTests(diffApp(query.DiffResult{}, context.DeadlineExceeded, nil), nil).GetTestVersionDiff(
		contextAs(t, publishPrincipal()), openapi.GetTestVersionDiffRequestObject{Id: uuid.New(), Version: 1, Params: openapi.GetTestVersionDiffParams{Against: "draft"}})
	if err == nil {
		t.Fatal("a database failure was answered as a result")
	}
}

func TestATestAnswersItsUnpublishedChangesOrAnExplicitNull(t *testing.T) {
	three := 3
	for name, c := range map[string]struct {
		count *int
		want  any
	}{"counted": {&three, float64(3)}, "not computed": {nil, nil}} {
		t.Run(name, func(t *testing.T) {
			app := &application.Application{Queries: application.Queries{
				Get: cqrs.HandlerFunc[query.Get, domain.Test](func(context.Context, query.Get) (domain.Test, error) {
					return domain.Test{ID: uuid.NewString(), Title: "Đề", Status: domain.Published, TotalPoints: "1.00", UnpublishedChanges: c.count}, nil
				}),
			}}
			out, err := testshttp.NewTests(app, nil).GetTest(contextAs(t, publishPrincipal()), openapi.GetTestRequestObject{Id: uuid.New()})
			if err != nil {
				t.Fatal(err)
			}
			rec := httptest.NewRecorder()
			if err := out.VisitGetTestResponse(rec); err != nil {
				t.Fatal(err)
			}
			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if got, present := body["unpublishedChanges"]; !present || got != c.want {
				t.Errorf("unpublishedChanges = %v (present %v), want %v", got, present, c.want)
			}
		})
	}
}
