package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/questions/application"
	questions "quizzivy/internal/modules/questions/domain"
	questionshttp "quizzivy/internal/modules/questions/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type altRepository struct {
	questions.Repository
	writes int
}

func (r *altRepository) Create(_ context.Context, in questions.WriteInput) (questions.Question, error) {
	r.writes++
	return questions.Question{
		ID: uuid.NewString(), Type: in.Input.Type, Prompt: in.Input.Prompt, Points: in.Input.Points,
		MediaAssetID: in.Input.MediaAssetID, MediaAssetKind: in.MediaAssetKind, MediaAlt: in.Input.MediaAlt,
	}, nil
}

func (r *altRepository) Update(ctx context.Context, in questions.WriteInput) (questions.Question, error) {
	return r.Create(ctx, in)
}

type fixedKind string

func (k fixedKind) Kind(context.Context, access.Scope, string) (string, error) { return string(k), nil }

func altBody[T any](t *testing.T, alt string) *T {
	t.Helper()
	raw := `{"type":"short_answer","prompt":"Câu hỏi","points":1,"tags":[],"mediaAssetId":"` + uuid.NewString() + `","mediaAlt":` + alt + `}`
	var body T
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatal(err)
	}
	return &body
}

func visitJSON(t *testing.T, visit func(http.ResponseWriter) error) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	if err := visit(rec); err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("not JSON: %s", rec.Body.String())
	}
	return rec.Code, out
}

func TestAltTextOnAnAudioQuestionAnswers400WithAFieldErrorOnMediaAlt(t *testing.T) {
	principal := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentQuestionsWrite)}
	ctx := contextAs(t, principal)
	repo := &altRepository{}
	handler := questionshttp.NewQuestions(application.New(repo, fixedKind("audio")), nil)

	create, err := handler.CreateQuestion(ctx, openapi.CreateQuestionRequestObject{Body: altBody[openapi.CreateQuestionJSONRequestBody](t, `"Một chú mèo"`)})
	if err != nil {
		t.Fatal(err)
	}
	update, err := handler.UpdateQuestion(ctx, openapi.UpdateQuestionRequestObject{Id: uuid.New(), Body: altBody[openapi.UpdateQuestionJSONRequestBody](t, `"Một chú mèo"`)})
	if err != nil {
		t.Fatal(err)
	}
	for name, visit := range map[string]func(http.ResponseWriter) error{
		"create": create.VisitCreateQuestionResponse,
		"update": update.VisitUpdateQuestionResponse,
	} {
		t.Run(name, func(t *testing.T) {
			status, body := visitJSON(t, visit)
			problem, _ := body["error"].(map[string]any)
			details, _ := problem["details"].(map[string]any)
			if status != http.StatusBadRequest || problem["code"] != "VALIDATION_FAILED" {
				t.Fatalf("alt text on an audio question answered %d %v, want 400 VALIDATION_FAILED", status, body)
			}
			if _, named := details["mediaAlt"]; !named {
				t.Fatalf("details = %v, want an entry for mediaAlt", details)
			}
		})
	}
	if repo.writes != 0 {
		t.Fatalf("a refused write reached the repository %d times", repo.writes)
	}
}

func TestAltTextOnAnImageQuestionIsStoredAndReturned(t *testing.T) {
	principal := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentQuestionsWrite)}
	repo := &altRepository{}
	handler := questionshttp.NewQuestions(application.New(repo, fixedKind("image")), nil)

	response, err := handler.CreateQuestion(contextAs(t, principal), openapi.CreateQuestionRequestObject{Body: altBody[openapi.CreateQuestionJSONRequestBody](t, `"Một chú mèo"`)})
	if err != nil {
		t.Fatal(err)
	}
	status, body := visitJSON(t, response.VisitCreateQuestionResponse)
	if status != http.StatusCreated || body["mediaAlt"] != "Một chú mèo" {
		t.Fatalf("answered %d %v, want 201 carrying mediaAlt", status, body)
	}
}

func TestStoredInputAndTheRequestShapeCarryAltText(t *testing.T) {
	alt := "Một chú mèo"
	wire, err := questionshttp.ToAPIInput(questions.Input{Type: questions.ShortAnswer, Prompt: "Hỏi", Points: "1", MediaAlt: &alt})
	if err != nil {
		t.Fatal(err)
	}
	if wire.MediaAlt == nil || *wire.MediaAlt != alt {
		t.Fatalf("the stored shape lost the alt text: %+v", wire.MediaAlt)
	}
	projected := questionshttp.ToQuestionInput(openapi.QuestionInput{Type: "short_answer", Prompt: "Hỏi", Points: 1, MediaAlt: &alt})
	if projected.MediaAlt == nil || *projected.MediaAlt != alt {
		t.Fatalf("the request lost the alt text: %+v", projected.MediaAlt)
	}
}

func TestTheWireAcceptsNullAndOneToAThousandCharactersOfAltText(t *testing.T) {
	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	validate, err := httpx.ValidateRequests(spec)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name  string
		alt   string
		valid bool
	}{
		{"null", `null`, true},
		{"one character", `"đ"`, true},
		{"a thousand characters", `"` + strings.Repeat("đ", questions.MaxMediaAltLength) + `"`, true},
		{"a thousand and one characters", `"` + strings.Repeat("đ", questions.MaxMediaAltLength+1) + `"`, false},
		{"empty", `""`, false},
		{"a number", `7`, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			raw := `{"type":"short_answer","prompt":"Hỏi","points":1,"mediaAssetId":"` + uuid.NewString() + `","mediaAlt":` + c.alt + `}`
			for _, method := range []string{http.MethodPost, http.MethodPatch} {
				path := "/teacher/questions"
				if method == http.MethodPatch {
					path += "/" + uuid.NewString()
				}
				reached := false
				handler := validate(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { reached = true; w.WriteHeader(http.StatusNoContent) }))
				req := httptest.NewRequest(method, path, strings.NewReader(raw))
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
				if reached != c.valid || (c.valid && rec.Code != http.StatusNoContent) || (!c.valid && rec.Code != http.StatusBadRequest) {
					t.Fatalf("%s: reached=%v status=%d body=%s", method, reached, rec.Code, rec.Body.String())
				}
			}
		})
	}
}
