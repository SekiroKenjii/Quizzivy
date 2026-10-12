package http_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/media/application"
	"quizzivy/internal/modules/media/application/command"
	"quizzivy/internal/modules/media/application/query"
	"quizzivy/internal/modules/media/domain"
	mediahttp "quizzivy/internal/modules/media/http"
	"quizzivy/internal/shared/cqrs"
	"testing"
	"time"

	"github.com/google/uuid"
)

type retirementBody struct {
	ID       string           `json:"id"`
	Bytes    int64            `json:"bytes"`
	URL      string           `json:"url"`
	UsedIn   []map[string]any `json:"usedIn"`
	DeleteAt *time.Time       `json:"deleteAt"`
}

func rawRetirement(t *testing.T, raw json.RawMessage) retirementBody {
	t.Helper()
	var body retirementBody
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("%v: %s", err, raw)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"id", "bytes", "url", "usedIn", "deleteAt"} {
		if _, ok := fields[name]; !ok {
			t.Errorf("the retirement has no %q field: %s", name, raw)
		}
	}
	return body
}

func TestDeleteAnswersTheFileAndWhenItsObjectGoes(t *testing.T) {
	id, goes := uuid.New(), time.Date(2026, 10, 19, 3, 0, 0, 0, time.UTC)
	for name, retired := range map[string]domain.Retired{
		"nothing names it":    {Asset: domain.Asset{ID: id.String(), Bytes: 4096, URL: "https://files.test/old"}, DeleteAt: &goes},
		"a question names it": {Asset: domain.Asset{ID: id.String(), Bytes: 4096, URL: "https://files.test/old"}},
	} {
		t.Run(name, func(t *testing.T) {
			app := &application.Application{Commands: application.Commands{Delete: cqrs.HandlerFunc[command.Delete, domain.Retired](func(context.Context, command.Delete) (domain.Retired, error) {
				return retired, nil
			})}}
			response, err := mediahttp.NewMedia(app).DeleteMedia(contextAs(t, principals()["an Admin"]), openapi.DeleteMediaRequestObject{Id: id})
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := response.(openapi.DeleteMedia200JSONResponse); !ok {
				t.Fatalf("the delete answered %T, want 200", response)
			}
			recorder := httptest.NewRecorder()
			if err := response.VisitDeleteMediaResponse(recorder); err != nil {
				t.Fatal(err)
			}
			if recorder.Code != 200 {
				t.Fatalf("status %d", recorder.Code)
			}
			body := rawRetirement(t, recorder.Body.Bytes())
			if body.ID != id.String() || body.Bytes != 4096 || body.URL != "https://files.test/old" || body.UsedIn == nil || len(body.UsedIn) != 0 {
				t.Errorf("the body is %+v, want the file, its URL and no version", body)
			}
			if (retired.DeleteAt == nil) != (body.DeleteAt == nil) || (body.DeleteAt != nil && !body.DeleteAt.Equal(goes)) {
				t.Errorf("deleteAt is %v, want %v", body.DeleteAt, retired.DeleteAt)
			}
		})
	}
}

func TestReplaceAnswersTheOldFileWithTheVersionsThatHoldIt(t *testing.T) {
	old, tested := uuid.New(), uuid.New()
	held := domain.Retired{
		Asset:  domain.Asset{ID: old.String(), Bytes: 2048, URL: "https://files.test/old"},
		UsedIn: []domain.TestRef{{ID: tested.String(), Title: "Đề nghe", Version: 3}},
	}
	app := &application.Application{Queries: application.Queries{ReplacementTarget: cqrs.HandlerFunc[query.ReplacementTarget, domain.ReplacementTarget](func(context.Context, query.ReplacementTarget) (domain.ReplacementTarget, error) {
		return domain.ReplacementTarget{}, nil
	})}, Commands: application.Commands{Replace: cqrs.HandlerFunc[command.Replace, domain.ReplaceResult](func(context.Context, command.Replace) (domain.ReplaceResult, error) {
		return domain.ReplaceResult{Asset: domain.Asset{ID: uuid.NewString(), Kind: domain.KindImage}, Previous: held}, nil
	})}}
	response, err := mediahttp.NewMedia(app).ReplaceMedia(contextAs(t, principals()["an Admin"]), replacementRequest())
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitReplaceMediaResponse(recorder); err != nil {
		t.Fatal(err)
	}
	var body struct {
		Previous json.RawMessage `json:"previous"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil || body.Previous == nil {
		t.Fatalf("the replacement carries no previous file: %v %s", err, recorder.Body.String())
	}
	previous := rawRetirement(t, body.Previous)
	if previous.ID != old.String() || previous.Bytes != 2048 || previous.DeleteAt != nil || len(previous.UsedIn) != 1 || previous.UsedIn[0]["id"] != tested.String() || previous.UsedIn[0]["version"] != float64(3) {
		t.Errorf("the old file is %+v, want it held by version 3 of the test with no removal time", previous)
	}
}
