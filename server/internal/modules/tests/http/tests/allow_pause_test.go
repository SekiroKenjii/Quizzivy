package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"quizzivy/gen/openapi"
	questions "quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/tests/application"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/application/query"
	"quizzivy/internal/modules/tests/domain"
	testshttp "quizzivy/internal/modules/tests/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/cqrs"

	"github.com/google/uuid"
)

func recordingBody(t *testing.T, policy string) openapi.CreateQuestionGroupJSONRequestBody {
	t.Helper()
	raw := `{"bundle":{"group":{"id":"01935000-0000-7000-8000-000000000020","title":"Bài nghe","members":[],"stimuli":[{"id":"01935000-0000-7000-8000-000000000021","title":"Hội thoại","content":{"format":"semantic_v1","blocks":[{"type":"audio","assetId":"01935000-0000-7000-8000-000000000001","label":"Hội thoại"}]},"gaps":[]}],"recordings":[{"id":"01935000-0000-7000-8000-000000000022","assetId":"01935000-0000-7000-8000-000000000001","policy":` + policy + `}]},"questions":[]}}`
	var body openapi.CreateQuestionGroupJSONRequestBody
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestARecordingsAllowPauseReachesTheCommandAsSentAndComesBack(t *testing.T) {
	const policy = `{"maxPlays":2,"allowSeek":false,"showTranscriptAfterSubmit":true`
	off, on := false, true
	for name, tc := range map[string]struct {
		policy string
		want   *bool
	}{
		"absent": {policy + `}`, nil},
		"off":    {policy + `,"allowPause":false}`, &off},
		"on":     {policy + `,"allowPause":true}`, &on},
	} {
		t.Run(name, func(t *testing.T) {
			body := recordingBody(t, tc.policy)
			var received *bool
			app := &application.Application{Commands: application.Commands{CreateGroup: cqrs.HandlerFunc[command.CreateGroup, domain.StoredGroup](func(_ context.Context, in command.CreateGroup) (domain.StoredGroup, error) {
				received = in.Bundle.Group.Recordings[0].Policy.AllowPause
				return domain.StoredGroup{Bundle: in.Bundle, Revision: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
			})}}
			transport := testshttp.NewTests(app, authoringMedia{})
			handler := httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
				return httpx.Principal{UserID: uuid.NewString()}, nil
			})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				response, err := transport.CreateQuestionGroup(r.Context(), openapi.CreateQuestionGroupRequestObject{Body: &body})
				if err != nil {
					t.Fatal(err)
				}
				if err := response.VisitCreateQuestionGroupResponse(w); err != nil {
					t.Fatal(err)
				}
			}))
			request := httptest.NewRequest(http.MethodPost, "/teacher/question-groups", nil)
			request.Header.Set("Authorization", "Bearer synthetic")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusCreated {
				t.Fatalf("answered %d %s", response.Code, response.Body.String())
			}
			if (received == nil) != (tc.want == nil) || (received != nil && *received != *tc.want) {
				t.Fatalf("the command was handed allowPause %v, want %v", received, tc.want)
			}
			var saved openapi.StoredQuestionGroup
			if err := json.Unmarshal(response.Body.Bytes(), &saved); err != nil {
				t.Fatal(err)
			}
			echoed := saved.Bundle.Group.Recordings[0].Policy.AllowPause
			if (echoed == nil) != (tc.want == nil) || (echoed != nil && *echoed != *tc.want) {
				t.Fatalf("the answer carries allowPause %v, want %v", echoed, tc.want)
			}
		})
	}
}

func TestThePreviewCarriesTheFrozenPauseRuleOfEachQuestionAndRecording(t *testing.T) {
	off, on := false, true
	seek, script := false, true
	question := func(id string, pause bool) domain.PreviewQuestion {
		return domain.PreviewQuestion{ID: id, SectionID: previewAssetID, Type: "short_answer", Prompt: "Nghe", Points: "1.00",
			AllowSeek: &seek, ShowScript: &script, AllowPause: pause}
	}
	paper := domain.PreviewPaper{Version: 1,
		Sections:  []domain.PreviewSection{{ID: previewAssetID, Title: "Listening"}},
		Questions: []domain.PreviewQuestion{question(previewQuestionID, false), question(previewAssetID, true)},
		Groups: []domain.PreviewGroup{{ID: previewAssetID, SectionID: previewAssetID, Title: "Shared", QuestionIDs: []string{previewQuestionID}, AssetIDs: []string{previewAssetID},
			Stimuli: []domain.GroupStimulus{},
			Recordings: []domain.PreviewRecording{
				{ID: previewQuestionID, AssetID: previewAssetID, Policy: questions.AudioPolicy{AllowPause: &off}},
				{ID: previewAssetID, AssetID: previewAssetID, Policy: questions.AudioPolicy{AllowPause: &on}},
				{ID: uuid.NewString(), AssetID: previewAssetID, Policy: questions.AudioPolicy{}},
			},
		}},
	}
	app := &application.Application{Queries: application.Queries{Preview: cqrs.HandlerFunc[query.Preview, query.PreviewResult](func(context.Context, query.Preview) (query.PreviewResult, error) {
		return paper, nil
	})}}
	handler := testshttp.NewTests(app, &previewMedia{})
	response, err := handler.PreviewTest(context.Background(), openapi.PreviewTestRequestObject{Id: uuid.MustParse(previewAssetID)})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitPreviewTestResponse(recorder); err != nil {
		t.Fatal(err)
	}
	var body struct {
		Questions []struct {
			Audio map[string]any `json:"audio"`
		} `json:"questions"`
		Groups []struct {
			Recordings []struct {
				Policy map[string]any `json:"policy"`
			} `json:"recordings"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("not JSON: %s", recorder.Body.String())
	}
	var got []any
	for _, q := range body.Questions {
		got = append(got, q.Audio["allowPause"])
	}
	for _, r := range body.Groups[0].Recordings {
		got = append(got, r.Policy["allowPause"])
	}
	want := []any{false, true, false, true, true}
	if len(got) != len(want) {
		t.Fatalf("allowPause values = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("allowPause values = %v, want %v", got, want)
		}
	}
	for _, key := range []string{"isCorrect", "sampleAnswer", "acceptedAnswers", "transcript"} {
		if strings.Contains(recorder.Body.String(), `"`+key+`"`) {
			t.Fatalf("preview exposes %s", key)
		}
	}
}
