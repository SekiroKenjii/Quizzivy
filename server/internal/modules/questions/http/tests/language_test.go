package http_test

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/questions/application"
	"quizzivy/internal/modules/questions/application/command"
	"quizzivy/internal/modules/questions/application/query"
	"quizzivy/internal/modules/questions/domain"
	questionshttp "quizzivy/internal/modules/questions/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/cqrs"
)

type refusing func(ctx context.Context, w http.ResponseWriter) error

func refusedIn(t *testing.T, acceptLanguage string, serve refusing) *httptest.ResponseRecorder {
	t.Helper()
	caller := uuid.NewString()
	handler := httpx.WithRequestMeta(func(*http.Request) string { return "203.0.113.9" })(
		httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
			return httpx.Principal{UserID: caller}, nil
		})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := serve(r.Context(), w); err != nil {
				t.Fatal(err)
			}
		})))
	request := httptest.NewRequest(http.MethodPost, "/teacher/questions", nil)
	request.Header.Set("Authorization", "Bearer fixture")
	if acceptLanguage != "" {
		request.Header.Set("Accept-Language", acceptLanguage)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func getting(outcome error) refusing {
	transport := questionshttp.NewQuestions(&application.Application{Queries: application.Queries{
		Get: cqrs.HandlerFunc[query.Get, domain.Question](func(context.Context, query.Get) (domain.Question, error) {
			return domain.Question{}, outcome
		}),
	}}, nil)
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.GetQuestion(ctx, openapi.GetQuestionRequestObject{Id: uuid.New()})
		if err != nil {
			return err
		}
		return response.VisitGetQuestionResponse(w)
	}
}

func deleting(outcome error) refusing {
	transport := questionshttp.NewQuestions(&application.Application{Commands: application.Commands{
		Delete: cqrs.HandlerFunc[command.Delete, cqrs.Nothing](func(context.Context, command.Delete) (cqrs.Nothing, error) {
			return cqrs.Nothing{}, outcome
		}),
	}}, nil)
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.DeleteQuestion(ctx, openapi.DeleteQuestionRequestObject{Id: uuid.New()})
		if err != nil {
			return err
		}
		return response.VisitDeleteQuestionResponse(w)
	}
}

func creating(t *testing.T, outcome error) refusing {
	t.Helper()
	transport := questionshttp.NewQuestions(&application.Application{Commands: application.Commands{
		Create: cqrs.HandlerFunc[command.Create, domain.Question](func(context.Context, command.Create) (domain.Question, error) {
			return domain.Question{}, outcome
		}),
	}}, nil)
	body := questionBody[openapi.CreateQuestionJSONRequestBody](t)
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.CreateQuestion(ctx, openapi.CreateQuestionRequestObject{Body: body})
		if err != nil {
			return err
		}
		return response.VisitCreateQuestionResponse(w)
	}
}

func updating(t *testing.T, outcome error) refusing {
	t.Helper()
	transport := questionshttp.NewQuestions(&application.Application{Commands: application.Commands{
		Update: cqrs.HandlerFunc[command.Update, domain.Question](func(context.Context, command.Update) (domain.Question, error) {
			return domain.Question{}, outcome
		}),
	}}, nil)
	body := questionBody[openapi.UpdateQuestionJSONRequestBody](t)
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.UpdateQuestion(ctx, openapi.UpdateQuestionRequestObject{Id: uuid.New(), Body: body})
		if err != nil {
			return err
		}
		return response.VisitUpdateQuestionResponse(w)
	}
}

func TestQuestionRefusalsSpeakTheCallersLanguage(t *testing.T) {
	const blankPrompt = "Nội dung câu hỏi không được để trống."
	for _, c := range []struct {
		name      string
		serve     refusing
		status    int
		code      openapi.ErrorCode
		vi        string
		en        string
		detailsVI map[string]interface{}
		detailsEN map[string]interface{}
	}{
		{
			name:   "reading a question that is not there",
			serve:  getting(domain.ErrNotFound),
			status: http.StatusNotFound,
			code:   openapi.NOTFOUND,
			vi:     "Không tìm thấy câu hỏi.",
			en:     "The question was not found.",
		},
		{
			name:   "deleting a question a draft uses",
			serve:  deleting(domain.ErrReferenced),
			status: http.StatusConflict,
			code:   openapi.QUESTIONREFERENCED,
			vi:     "Câu hỏi đang được dùng trong một đề nháp nên không thể xoá.",
			en:     "The question is used in a draft test, so it cannot be deleted.",
		},
		{
			name:      "creating a question whose file is gone",
			serve:     creating(t, domain.ErrMediaNotFound),
			status:    http.StatusBadRequest,
			code:      openapi.VALIDATIONFAILED,
			vi:        "Không tìm thấy tệp đính kèm.",
			en:        "The attachment was not found.",
			detailsVI: map[string]interface{}{"mediaAssetId": "Tệp không tồn tại hoặc đã bị xoá."},
			detailsEN: map[string]interface{}{"mediaAssetId": "The file does not exist or has been deleted."},
		},
		{
			name:      "updating a question the domain refuses",
			serve:     updating(t, &domain.ValidationError{Fields: []domain.FieldError{{Field: "prompt", Message: blankPrompt}}}),
			status:    http.StatusBadRequest,
			code:      openapi.VALIDATIONFAILED,
			vi:        "Dữ liệu câu hỏi không hợp lệ.",
			en:        "The question data is not valid.",
			detailsVI: map[string]interface{}{"prompt": blankPrompt},
			detailsEN: map[string]interface{}{"prompt": blankPrompt},
		},
	} {
		for _, language := range []struct {
			accept  string
			message string
			details map[string]interface{}
		}{{"", c.vi, c.detailsVI}, {"en", c.en, c.detailsEN}} {
			response := refusedIn(t, language.accept, c.serve)
			var body openapi.ErrorResponse
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("%s with Accept-Language %q: %v in %s", c.name, language.accept, err, response.Body.String())
			}
			if response.Code != c.status || body.Error.Code != c.code {
				t.Errorf("%s with Accept-Language %q answered %d %s, want %d %s", c.name, language.accept, response.Code, body.Error.Code, c.status, c.code)
			}
			if body.Error.Message != language.message {
				t.Errorf("%s with Accept-Language %q answered %q, want %q", c.name, language.accept, body.Error.Message, language.message)
			}
			var details map[string]interface{}
			if body.Error.Details != nil {
				details = *body.Error.Details
			}
			if !maps.Equal(details, language.details) {
				t.Errorf("%s with Accept-Language %q carries details %v, want %v", c.name, language.accept, details, language.details)
			}
		}
	}
}
