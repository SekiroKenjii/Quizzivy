package http_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"quizzivy/gen/openapi"
	mediadomain "quizzivy/internal/modules/media/domain"
	questions "quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/tests/application"
	"quizzivy/internal/modules/tests/application/query"
	"quizzivy/internal/modules/tests/domain"
	testshttp "quizzivy/internal/modules/tests/http"
	"quizzivy/internal/shared/cqrs"
	"strings"
	"testing"

	"github.com/google/uuid"
)

const previewAssetID = "01935000-0000-7000-8000-000000000001"
const previewQuestionID = "01935000-0000-7000-8000-000000000002"

type previewMedia struct{ requested []string }

func (m *previewMedia) Get(_ context.Context, id string) (mediadomain.Asset, error) {
	m.requested = append(m.requested, id)
	return mediadomain.Asset{ID: id, Kind: mediadomain.KindAudio, MimeType: "audio/mpeg", Bytes: 12, OriginalFilename: "audio.mp3"}, nil
}

func (*previewMedia) SignedURL(_ context.Context, _ mediadomain.Asset) (string, error) {
	return "https://assets.example/authorized", nil
}

func TestPreviewSerializesSafeGroupBindingsAndOnlyBoundAssets(t *testing.T) {
	blank := "response"
	paper := domain.PreviewPaper{Version: 3,
		Sections:  []domain.PreviewSection{{ID: previewAssetID, Title: "Reading"}},
		Questions: []domain.PreviewQuestion{{ID: previewQuestionID, SectionID: previewAssetID, Type: "fill_blank", Prompt: "[1]", Points: "1.00"}},
		Groups: []domain.PreviewGroup{{ID: previewAssetID, SectionID: previewAssetID, Title: "Shared", QuestionIDs: []string{previewQuestionID}, AssetIDs: []string{previewAssetID},
			Stimuli:    []domain.GroupStimulus{{ID: previewAssetID, Title: "Passage", Content: json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"gap","id":"material","label":"1"}]}]}`), Gaps: []domain.GroupGapBinding{{Kind: "blank", GapID: "material", QuestionID: previewQuestionID, BlankGapID: &blank}}}},
			Recordings: []domain.PreviewRecording{{ID: previewQuestionID, AssetID: previewAssetID, Policy: questions.AudioPolicy{AllowSeek: true}}},
		}},
	}
	app := &application.Application{Queries: application.Queries{Preview: cqrs.HandlerFunc[query.Preview, query.PreviewResult](func(_ context.Context, in query.Preview) (query.PreviewResult, error) {
		if in.Version != 3 {
			t.Fatal("requested version lost")
		}
		return paper, nil
	})}}
	media := &previewMedia{}
	handler := testshttp.NewTests(app, media)
	version := 3
	response, err := handler.PreviewTest(context.Background(), openapi.PreviewTestRequestObject{Id: uuid.MustParse(previewAssetID), Params: openapi.PreviewTestParams{Version: &version}})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitPreviewTestResponse(recorder); err != nil {
		t.Fatal(err)
	}
	body := recorder.Body.String()
	for _, key := range []string{"isCorrect", "sampleAnswer", "acceptedAnswers", "transcript"} {
		if strings.Contains(body, `"`+key+`"`) {
			t.Fatalf("preview exposes %s", key)
		}
	}
	for _, required := range []string{`"kind":"blank"`, `"blankGapId":"response"`, `"questionId":"` + previewQuestionID + `"`, `https://assets.example/authorized`, `"questionIds":["` + previewQuestionID + `"]`} {
		if !strings.Contains(body, required) {
			t.Fatalf("preview lost %s: %s", required, body)
		}
	}
	if len(media.requested) != 1 || media.requested[0] != previewAssetID {
		t.Fatalf("unexpected asset resolution: %v", media.requested)
	}
}
