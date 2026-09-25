package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"quizzivy/gen/openapi"
	mediadomain "quizzivy/internal/modules/media/domain"
	"quizzivy/internal/modules/tests/application"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/domain"
	testshttp "quizzivy/internal/modules/tests/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/cqrs"
	"testing"
	"time"
)

type authoringMedia struct{ fail bool }

func (m authoringMedia) Get(context.Context, string) (mediadomain.Asset, error) {
	return mediadomain.Asset{ID: previewAssetID, Kind: mediadomain.KindAudio, MimeType: "audio/mpeg", Bytes: 100, OriginalFilename: "fixture.mp3", CreatedAt: time.Now()}, nil
}
func (m authoringMedia) SignedURL(context.Context, mediadomain.Asset) (string, error) {
	if m.fail {
		return "", errors.New("storage unavailable")
	}
	return "https://media.example/signed", nil
}

func TestGroupWriteRoundTripsPoliciesAndAcknowledgesSavedContentDuringMediaFailure(t *testing.T) {
	var body openapi.CreateQuestionGroupJSONRequestBody
	input := `{"bundle":{"group":{"id":"01935000-0000-7000-8000-000000000020","title":"Bài nghe","members":[],"stimuli":[{"id":"01935000-0000-7000-8000-000000000021","title":"Hội thoại","content":{"format":"semantic_v1","blocks":[{"type":"audio","assetId":"01935000-0000-7000-8000-000000000001","label":"Hội thoại"}]},"gaps":[]}],"recordings":[{"id":"01935000-0000-7000-8000-000000000022","assetId":"01935000-0000-7000-8000-000000000001","policy":{"maxPlays":2,"allowSeek":true,"showTranscriptAfterSubmit":true},"transcript":"Teacher transcript"}]},"questions":[]}}`
	if err := json.Unmarshal([]byte(input), &body); err != nil {
		t.Fatal(err)
	}
	for _, offline := range []bool{false, true} {
		t.Run(map[bool]string{false: "available", true: "offline"}[offline], func(t *testing.T) {
			app := &application.Application{Commands: application.Commands{CreateGroup: cqrs.HandlerFunc[command.CreateGroup, domain.StoredGroup](func(_ context.Context, in command.CreateGroup) (domain.StoredGroup, error) {
				if in.Actor.ID == "" {
					t.Fatal("write lost authenticated actor")
				}
				if err := in.Bundle.Validate(); err != nil {
					t.Fatalf("transport damaged graph: %v", err)
				}
				policy := in.Bundle.Group.Recordings[0].Policy
				if policy.MaxPlays == nil || *policy.MaxPlays != 2 || !policy.AllowSeek || !policy.ShowTranscriptAfterSubmit {
					t.Fatalf("policy was lost: %+v", policy)
				}
				return domain.StoredGroup{Bundle: in.Bundle, Revision: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
			})}}
			transport := testshttp.NewTests(app, authoringMedia{fail: offline})
			handler := httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
				return httpx.Principal{UserID: uuid.NewString(), Role: "admin"}, nil
			})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				response, err := transport.CreateQuestionGroup(r.Context(), openapi.CreateQuestionGroupRequestObject{Body: &body})
				if err != nil {
					t.Fatal(err)
				}
				if err := response.VisitCreateQuestionGroupResponse(w); err != nil {
					t.Fatal(err)
				}
			}))
			req := httptest.NewRequest(http.MethodPost, "/admin/question-groups", nil)
			req.Header.Set("Authorization", "Bearer synthetic")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			if response.Code != http.StatusCreated {
				t.Fatalf("save acknowledgement lost: %d %s", response.Code, response.Body.String())
			}
			var saved openapi.StoredQuestionGroup
			if err := json.Unmarshal(response.Body.Bytes(), &saved); err != nil {
				t.Fatal(err)
			}
			policy := saved.Bundle.Group.Recordings[0].Policy
			if policy.MaxPlays == nil || *policy.MaxPlays != 2 || !policy.AllowSeek || !policy.ShowTranscriptAfterSubmit || *saved.Bundle.Group.Recordings[0].Transcript != "Teacher transcript" {
				t.Fatal("serialized policy/transcript changed")
			}
			if offline {
				if len(saved.Assets) != 0 || len(saved.UnavailableAssetIds) != 1 {
					t.Fatalf("missing retryable material state: %+v", saved)
				}
			} else if len(saved.Assets) != 1 || len(saved.UnavailableAssetIds) != 0 {
				t.Fatalf("bound media not resolved: %+v", saved)
			}
		})
	}
}
