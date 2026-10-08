package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/tests/application"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/application/query"
	"quizzivy/internal/modules/tests/domain"
	testshttp "quizzivy/internal/modules/tests/http"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
	"testing"
	"time"

	"github.com/google/uuid"
)

func publishPrincipal() access.Principal {
	return access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentTestsWrite)}
}

func TestPublishPassesTheChangeNoteToTheCommandAndAnswersTheVersionMetadata(t *testing.T) {
	var seen domain.PublishRequest
	updated := time.Date(2026, 10, 8, 3, 4, 5, 123456000, time.UTC)
	note := "Thêm phần Nghe"
	app := &application.Application{Commands: application.Commands{
		Publish: cqrs.HandlerFunc[command.Publish, domain.Version](func(_ context.Context, c command.Publish) (domain.Version, error) {
			seen = c.Request
			return domain.Version{
				ID: uuid.NewString(), Version: 3, TotalPoints: "10.50", AssignmentCount: 2,
				ChangeNote: &note, TestUpdatedAt: &updated, PublishedAt: updated, PublishedBy: "Giáo viên",
			}, nil
		}),
	}}
	transport := testshttp.NewTests(app, nil)
	ctx := contextAs(t, publishPrincipal())
	sent := "  Thêm phần Nghe  "

	for name, body := range map[string]*openapi.PublishTestJSONRequestBody{
		"a note":        {ChangeNote: &sent},
		"no note":       {},
		"an empty body": nil,
	} {
		t.Run(name, func(t *testing.T) {
			seen = domain.PublishRequest{}
			out, err := transport.PublishTest(ctx, openapi.PublishTestRequestObject{Id: uuid.New(), Body: body})
			if err != nil {
				t.Fatal(err)
			}
			if body != nil && body.ChangeNote != nil {
				if seen.ChangeNote == nil || *seen.ChangeNote != sent {
					t.Errorf("the command got note %v, want the body's text untouched", seen.ChangeNote)
				}
			} else if seen.ChangeNote != nil {
				t.Errorf("the command got note %q, want none", *seen.ChangeNote)
			}

			rec := httptest.NewRecorder()
			if err := out.VisitPublishTestResponse(rec); err != nil {
				t.Fatal(err)
			}
			if rec.Code != http.StatusCreated {
				t.Fatalf("status %d, want 201", rec.Code)
			}
			var version map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &version); err != nil {
				t.Fatal(err)
			}
			if version["assignmentCount"] != float64(2) || version["changeNote"] != note {
				t.Errorf("assignmentCount %v and changeNote %v, want 2 and %q", version["assignmentCount"], version["changeNote"], note)
			}
			if version["testUpdatedAt"] != "2026-10-08T03:04:05.123456Z" {
				t.Errorf("testUpdatedAt %v, want the publish transaction's time", version["testUpdatedAt"])
			}
		})
	}
}

func TestVersionHistoryAnswersANullNoteAndNoTestUpdateTime(t *testing.T) {
	used := "Bản đầu"
	app := &application.Application{Queries: application.Queries{
		ListVersions: cqrs.HandlerFunc[query.ListVersions, []domain.Version](func(context.Context, query.ListVersions) ([]domain.Version, error) {
			return []domain.Version{
				{ID: uuid.NewString(), Version: 2, TotalPoints: "5.00", PublishedAt: time.Now(), PublishedBy: "Giáo viên"},
				{ID: uuid.NewString(), Version: 1, TotalPoints: "5.00", PublishedAt: time.Now(), PublishedBy: "Giáo viên", ChangeNote: &used, AssignmentCount: 1},
			}, nil
		}),
	}}
	out, err := testshttp.NewTests(app, nil).ListTestVersions(contextAs(t, publishPrincipal()), openapi.ListTestVersionsRequestObject{Id: uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	if err := out.VisitListTestVersionsResponse(rec); err != nil {
		t.Fatal(err)
	}
	var body struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 2 {
		t.Fatalf("%d items, want 2", len(body.Items))
	}
	if note, present := body.Items[0]["changeNote"]; !present || note != nil {
		t.Errorf("a version without a note answers changeNote %v (present %v), want an explicit null", note, present)
	}
	if body.Items[1]["changeNote"] != used || body.Items[1]["assignmentCount"] != float64(1) {
		t.Errorf("version 1 answers %v and %v", body.Items[1]["changeNote"], body.Items[1]["assignmentCount"])
	}
	for i, item := range body.Items {
		if _, present := item["testUpdatedAt"]; present {
			t.Errorf("item %d carries testUpdatedAt; only publishTest answers it", i)
		}
		if _, present := item["assignmentCount"]; !present {
			t.Errorf("item %d has no assignmentCount", i)
		}
	}
}

func TestATestAnswersItsAssignmentCounts(t *testing.T) {
	app := &application.Application{Queries: application.Queries{
		Get: cqrs.HandlerFunc[query.Get, domain.Test](func(context.Context, query.Get) (domain.Test, error) {
			return domain.Test{
				ID: uuid.NewString(), Title: "Đề", Status: domain.Published, TotalPoints: "1.00",
				Assignments: domain.AssignmentCounts{Live: 2, Scheduled: 1, Closed: 4},
			}, nil
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
	var body struct {
		Assignments map[string]any `json:"assignments"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Assignments["live"] != float64(2) || body.Assignments["scheduled"] != float64(1) || body.Assignments["closed"] != float64(4) {
		t.Errorf("assignments = %v, want live 2, scheduled 1, closed 4", body.Assignments)
	}
}
