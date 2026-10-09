package http_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/tests/application"
	"quizzivy/internal/modules/tests/application/query"
	"quizzivy/internal/modules/tests/domain"
	testshttp "quizzivy/internal/modules/tests/http"
	"quizzivy/internal/shared/cqrs"
	"testing"

	"github.com/google/uuid"
)

func TestPreviewCarriesTheFrozenAltTextBesideTheMedia(t *testing.T) {
	alt := "Một chú mèo ngồi trên ghế"
	other := "01935000-0000-7000-8000-000000000003"
	paper := domain.PreviewPaper{Version: 2,
		Sections: []domain.PreviewSection{{ID: previewAssetID, Title: "Reading"}},
		Questions: []domain.PreviewQuestion{
			{ID: previewQuestionID, SectionID: previewAssetID, Type: "short_answer", Prompt: "Con gì?", Points: "1.00", MediaAssetID: ptr(previewAssetID), MediaAlt: &alt},
			{ID: other, SectionID: previewAssetID, Type: "short_answer", Prompt: "Không ảnh", Points: "1.00"},
		},
	}
	app := &application.Application{Queries: application.Queries{Preview: cqrs.HandlerFunc[query.Preview, query.PreviewResult](func(context.Context, query.Preview) (query.PreviewResult, error) {
		return paper, nil
	})}}
	response, err := testshttp.NewTests(app, &previewMedia{}).PreviewTest(context.Background(), openapi.PreviewTestRequestObject{Id: uuid.MustParse(previewAssetID)})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitPreviewTestResponse(recorder); err != nil {
		t.Fatal(err)
	}
	var body struct {
		Questions []map[string]any `json:"questions"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Questions) != 2 || body.Questions[0]["mediaAlt"] != alt {
		t.Fatalf("questions = %v, want the first to carry mediaAlt %q", body.Questions, alt)
	}
	if _, present := body.Questions[1]["mediaAlt"]; present {
		t.Fatalf("a question with no alt text carries mediaAlt: %v", body.Questions[1])
	}
}

func ptr[T any](value T) *T { return &value }
