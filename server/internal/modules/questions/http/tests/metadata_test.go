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
	"quizzivy/internal/shared/paging"
	"reflect"
	"strings"
	"testing"
)

func TestQuestionAuthoringWireCapAndClosedMetadata(t *testing.T) {
	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	validate, err := httpx.ValidateRequests(spec)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{8, 9} {
		options := make([]map[string]any, n)
		for i := range options {
			options[i] = map[string]any{"text": "Choice", "isCorrect": i == 0}
		}
		for _, metadata := range []map[string]any{{"level": "pre_a1", "skill": "speaking"}, {"level": nil, "skill": nil}, {}, {"level": "A1"}, {"skill": "unknown"}} {
			body := map[string]any{"type": "single_choice", "prompt": "Pick", "points": 1, "options": options}
			for k, v := range metadata {
				body[k] = v
			}
			raw, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			reached := false
			handler := validate(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { reached = true; w.WriteHeader(204) }))
			for _, method := range []string{http.MethodPost, http.MethodPatch} {
				path := "/teacher/questions"
				if method == http.MethodPatch {
					path += "/019535d9-3df7-79fb-b466-fa907fa17f9e"
				}
				req := httptest.NewRequest(method, path, strings.NewReader(string(raw)))
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()
				reached = false
				handler.ServeHTTP(rec, req)
				valid := n == 8 && metadata["level"] != "A1" && metadata["skill"] != "unknown"
				if reached != valid || (valid && rec.Code != 204) || (!valid && rec.Code != 400) {
					t.Fatalf("%s/%d/%v reached=%v status=%d body=%s", method, n, metadata, reached, rec.Code, rec.Body.String())
				}
			}
		}
	}
}

func TestStoredInputPreservesMetadataAndExplicitNulls(t *testing.T) {
	level, skill := questions.Level("c2"), questions.Skill("reading")
	in := questions.Input{Type: questions.ShortAnswer, Prompt: "Write", Points: "1", Level: &level, Skill: &skill}
	wire, err := questionshttp.ToAPIInput(in)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got["level"] != "c2" || got["skill"] != "reading" {
		t.Fatalf("metadata lost: %s", raw)
	}
	request := openapi.QuestionInput{Type: openapi.QuestionType("short_answer"), Prompt: "Write", Points: 1, Level: wire.Level, Skill: wire.Skill}
	projected := questionshttp.ToQuestionInput(request)
	if projected.Level == nil || *projected.Level != level || projected.Skill == nil || *projected.Skill != skill {
		t.Fatalf("request lost metadata: %+v", projected)
	}
	in.Level, in.Skill = nil, nil
	wire, err = questionshttp.ToAPIInput(in)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"level":null`) || !strings.Contains(string(raw), `"skill":null`) {
		t.Fatalf("unset fields omitted: %s", raw)
	}
}

func TestQuestionListDimensionsReachEveryQueryAndFixedFacetsKeepZeros(t *testing.T) {
	repo := &dimensionRepository{seen: map[string]questions.ListInput{}}
	principal := access.Principal{UserID: "019535d9-3df7-79fb-b466-fa907fa17f9e", Permissions: access.NewSet(access.ContentQuestionsWrite, access.ScopeAll)}
	levels := []openapi.QuestionLevel{"a1", "c2"}
	skills := []openapi.QuestionSkill{"reading", "speaking"}
	types := []openapi.QuestionType{"single_choice"}
	tags := []string{"one", "two"}
	mode := openapi.ListQuestionsParamsTagMatchAll
	response, err := questionshttp.NewQuestions(application.New(repo, nil), nil).ListQuestions(contextAs(t, principal), openapi.ListQuestionsRequestObject{Params: openapi.ListQuestionsParams{Type: &types, Tag: &tags, Level: &levels, Skill: &skills, TagMatch: &mode}})
	if err != nil {
		t.Fatal(err)
	}
	expected := questions.ListInput{Scope: access.Scope{UserID: principal.UserID}, Types: []questions.Type{questions.SingleChoice}, Tags: tags, Levels: []questions.Level{"a1", "c2"}, Skills: []questions.Skill{"reading", "speaking"}, TagMatch: "all"}
	if len(repo.seen) != 4 {
		t.Fatalf("missing query inputs: %+v", repo.seen)
	}
	for name, input := range repo.seen {
		if !reflect.DeepEqual(input, expected) {
			t.Fatalf("%s dimensions/scope got=%+v want=%+v", name, input, expected)
		}
	}
	rec := httptest.NewRecorder()
	if err := response.VisitListQuestionsResponse(rec); err != nil {
		t.Fatal(err)
	}
	var out struct {
		Facets struct {
			Levels map[string]int
			Skills map[string]int
		}
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Facets.Levels) != 7 || len(out.Facets.Skills) != 6 || out.Facets.Levels["a1"] != 2 || out.Facets.Skills["speaking"] != 2 || out.Facets.Levels["c2"] != 0 || out.Facets.Skills["reading"] != 0 {
		t.Fatalf("fixed facets lost values/zeros: %s", rec.Body.String())
	}
}

type dimensionRepository struct {
	questions.Repository
	seen map[string]questions.ListInput
}

func (r *dimensionRepository) List(_ context.Context, in questions.ListInput) ([]questions.Question, paging.Page, error) {
	r.seen["list"] = in
	return []questions.Question{}, paging.Page{Number: 1, Size: 20}, nil
}

func (r *dimensionRepository) Facets(_ context.Context, in questions.ListInput) (questions.TypeFacets, error) {
	r.seen["facets"] = in
	return questions.TypeFacets{All: 2, ByType: map[questions.Type]int{questions.SingleChoice: 2}, ByLevel: map[questions.Level]int{"a1": 2}, BySkill: map[questions.Skill]int{"speaking": 2}}, nil
}

func (r *dimensionRepository) Tags(_ context.Context, in questions.ListInput) ([]string, error) {
	r.seen["tags"] = in
	return []string{}, nil
}

func (r *dimensionRepository) Counts(_ context.Context, in questions.ListInput) (int, int, error) {
	r.seen["counts"] = in
	return 3, 2, nil
}
