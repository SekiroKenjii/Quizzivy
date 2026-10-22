package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/questions/application"
	questions "quizzivy/internal/modules/questions/domain"
	questionshttp "quizzivy/internal/modules/questions/http"
	"quizzivy/internal/shared/access"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

// typesRepository stores nothing: it answers a create with the question the
// input describes, ids minted, so the handler's projection can be read.
type typesRepository struct {
	questions.Repository
	written []questions.WriteInput
}

func (r *typesRepository) Create(_ context.Context, in questions.WriteInput) (questions.Question, error) {
	r.written = append(r.written, in)
	q := questions.Question{ID: uuid.NewString(), Type: in.Input.Type, Prompt: in.Input.Prompt, Points: in.Input.Points,
		SelectCount: in.Input.SelectCount, MaxWords: in.Input.MaxWords, MatchAllowReuse: in.Input.MatchAllowReuse}
	for i, o := range in.Input.Options {
		q.Options = append(q.Options, questions.Option{ID: uuid.NewString(), Ordinal: i, Text: o.Text, IsCorrect: o.IsCorrect})
	}
	for i, item := range in.Input.MatchItems {
		q.MatchItems = append(q.MatchItems, questions.MatchItem{ID: uuid.NewString(), Ordinal: i, Text: item.Text,
			CorrectOption: item.CorrectOption, CorrectOptionID: q.Options[item.CorrectOption].ID})
	}
	for i, criterion := range in.Input.Rubric {
		q.Rubric = append(q.Rubric, questions.RubricCriterion{ID: uuid.NewString(), Ordinal: i, Label: criterion.Label})
	}
	return q, nil
}

func createBody(t *testing.T, raw string) openapi.CreateQuestionRequestObject {
	t.Helper()
	var body openapi.CreateQuestionJSONRequestBody
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatal(err)
	}
	return openapi.CreateQuestionRequestObject{Body: &body}
}

// R6C-1: a field of another type is refused by name, never stored or dropped.
func TestAFieldOfAnotherTypeIsRefusedByName(t *testing.T) {
	ctx := contextAs(t, access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentQuestionsWrite)})
	choice := `"options":[{"text":"A","isCorrect":true},{"text":"B","isCorrect":false},{"text":"C","isCorrect":false}]`
	for field, raw := range map[string]string{
		"selectCount":     `{"type":"single_choice","prompt":"p","points":1,` + choice + `,"selectCount":2}`,
		"maxWords":        `{"type":"essay","prompt":"p","points":1,"maxWords":200}`,
		"matchAllowReuse": `{"type":"short_answer","prompt":"p","points":1,"matchAllowReuse":false}`,
		"matchItems":      `{"type":"single_choice","prompt":"p","points":1,` + choice + `,"matchItems":[{"text":"x","correctOption":0}]}`,
		"rubric":          `{"type":"short_answer","prompt":"p","points":1,"rubric":[{"label":"Grammar"}]}`,
	} {
		repo := &typesRepository{}
		handler := questionshttp.NewQuestions(application.New(repo, fixedKind("image")), nil)
		response, err := handler.CreateQuestion(ctx, createBody(t, raw))
		if err != nil {
			t.Fatal(err)
		}
		status, body := visitJSON(t, response.VisitCreateQuestionResponse)
		details, _ := body["error"].(map[string]any)["details"].(map[string]any)
		if status != http.StatusBadRequest || details[field] == nil {
			t.Errorf("%s on the wrong type answered %d %v, want 400 naming %s", field, status, body, field)
		}
		if len(repo.written) != 0 {
			t.Errorf("%s on the wrong type reached the store", field)
		}
	}
}

func TestAMatchingQuestionAnswersWithItsKeys(t *testing.T) {
	ctx := contextAs(t, access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentQuestionsWrite)})
	repo := &typesRepository{}
	handler := questionshttp.NewQuestions(application.New(repo, fixedKind("image")), nil)
	response, err := handler.CreateQuestion(ctx, createBody(t, `{"type":"matching","prompt":"Match","points":2,"matchAllowReuse":true,
		"options":[{"text":"Port","isCorrect":false},{"text":"Village","isCorrect":false}],
		"matchItems":[{"text":"Paragraph 1","correctOption":1},{"text":"Paragraph 2","correctOption":1}]}`))
	if err != nil {
		t.Fatal(err)
	}
	status, body := visitJSON(t, response.VisitCreateQuestionResponse)
	if status != http.StatusCreated || len(repo.written) != 1 {
		t.Fatalf("answered %d %v", status, body)
	}
	if in := repo.written[0].Input; len(in.MatchItems) != 2 || in.MatchItems[1].CorrectOption != 1 || in.MatchAllowReuse == nil || !*in.MatchAllowReuse {
		t.Errorf("the store was sent %+v reuse %v", in.MatchItems, in.MatchAllowReuse)
	}
	options := body["options"].([]any)
	items := body["matchItems"].([]any)
	if options[1].(map[string]any)["key"] != "ii" || items[1].(map[string]any)["key"] != "B" ||
		items[0].(map[string]any)["correctOptionId"] != options[1].(map[string]any)["id"] || body["matchAllowReuse"] != true {
		t.Errorf("answered options %v items %v", options, items)
	}
	if _, has := body["rubric"]; has {
		t.Errorf("a matching question answered a rubric: %v", body["rubric"])
	}
}

func TestStoredInputCarriesEveryNewField(t *testing.T) {
	id := uuid.NewString()
	for _, in := range []questions.Input{
		{Type: questions.Matching, Prompt: "p", Points: "1", MatchAllowReuse: new(false),
			Options:    []questions.OptionInput{{Text: "a"}, {Text: "b"}},
			MatchItems: []questions.MatchItemInput{{ID: &id, Text: "x", CorrectOption: 1}, {Text: "y", CorrectOption: 0}}},
		{Type: questions.Essay, Prompt: "p", Points: "9", Rubric: []questions.RubricCriterionInput{{ID: &id, Label: "Grammar"}}},
		{Type: questions.MultipleChoice, Prompt: "p", Points: "1", SelectCount: new(2)},
		{Type: questions.ShortAnswer, Prompt: "p", Points: "1", MaxWords: new(3)},
	} {
		wire, err := questionshttp.ToAPIInput(in)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(wire)
		if err != nil {
			t.Fatal(err)
		}
		var request openapi.QuestionInput
		if err := json.Unmarshal(raw, &request); err != nil {
			t.Fatal(err)
		}
		back := questionshttp.ToQuestionInput(request)
		if !reflect.DeepEqual(back.MatchItems, in.MatchItems) || !reflect.DeepEqual(back.Rubric, in.Rubric) ||
			!reflect.DeepEqual(back.SelectCount, in.SelectCount) || !reflect.DeepEqual(back.MaxWords, in.MaxWords) ||
			!reflect.DeepEqual(back.MatchAllowReuse, in.MatchAllowReuse) {
			t.Errorf("%s came back as %+v", in.Type, back)
		}
	}
}
