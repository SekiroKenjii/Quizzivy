package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/classes/application"
	"quizzivy/internal/modules/classes/application/query"
	"quizzivy/internal/modules/classes/domain"
	classeshttp "quizzivy/internal/modules/classes/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/cqrs"
)

func contextIn(t *testing.T, acceptLanguage string) context.Context {
	t.Helper()
	var ctx context.Context
	handler := httpx.WithRequestMeta(func(*http.Request) string { return "203.0.113.9" })(
		http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { ctx = r.Context() }))
	request := httptest.NewRequest(http.MethodPost, "/join/preview", nil)
	if acceptLanguage != "" {
		request.Header.Set("Accept-Language", acceptLanguage)
	}
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if ctx == nil {
		t.Fatal("the request did not reach the handler")
	}
	return ctx
}

func TestJoinCodeRefusalsSpeakTheCallersLanguage(t *testing.T) {
	for _, c := range []struct {
		name    string
		outcome domain.PreviewOutcome
		code    openapi.ErrorCode
		vi      string
		en      string
	}{
		{
			name:    "a revoked code",
			outcome: domain.PreviewRevoked,
			code:    openapi.JOINCODEREVOKED,
			vi:      "Mã lớp này đã bị thu hồi. Vui lòng xin giáo viên mã mới.",
			en:      "This class code has been revoked. Please ask your teacher for a new one.",
		},
		{
			name:    "an expired code",
			outcome: domain.PreviewExpired,
			code:    openapi.JOINCODEEXPIRED,
			vi:      "Mã lớp này đã hết hạn. Vui lòng xin giáo viên mã mới.",
			en:      "This class code has expired. Please ask your teacher for a new one.",
		},
		{
			name:    "a code with no uses left",
			outcome: domain.PreviewExhausted,
			code:    openapi.JOINCODEEXHAUSTED,
			vi:      "Mã lớp này đã hết lượt sử dụng. Vui lòng xin giáo viên mã mới.",
			en:      "This class code has no uses left. Please ask your teacher for a new one.",
		},
		{
			name:    "a code no class uses",
			outcome: domain.PreviewInvalid,
			code:    openapi.JOINCODEINVALID,
			vi:      "Mã lớp không đúng. Vui lòng kiểm tra lại.",
			en:      "That class code is not right. Please check it.",
		},
	} {
		for _, language := range []struct {
			accept string
			want   string
		}{{"", c.vi}, {"en", c.en}} {
			refusal := classeshttp.JoinCodeError(contextIn(t, language.accept), c.outcome)
			if refusal.Error.Code != c.code {
				t.Errorf("%s with Accept-Language %q answered %s, want %s", c.name, language.accept, refusal.Error.Code, c.code)
			}
			if refusal.Error.Message != language.want {
				t.Errorf("%s with Accept-Language %q answered %q, want %q", c.name, language.accept, refusal.Error.Message, language.want)
			}
		}
	}

	missing := classeshttp.NewClasses(&application.Application{Queries: application.Queries{
		Get: cqrs.HandlerFunc[query.Get, domain.Class](func(context.Context, query.Get) (domain.Class, error) {
			return domain.Class{}, domain.ErrNotFound
		}),
	}})
	for _, language := range []struct {
		accept string
		want   string
	}{{"", "Không tìm thấy lớp học."}, {"en", "The class was not found."}} {
		response, err := missing.GetClass(contextIn(t, language.accept), openapi.GetClassRequestObject{Id: uuid.New()})
		if err != nil {
			t.Fatalf("a missing class with Accept-Language %q: %v", language.accept, err)
		}
		refused, ok := response.(openapi.GetClass404JSONResponse)
		if !ok {
			t.Fatalf("a missing class with Accept-Language %q answered %T, want 404", language.accept, response)
		}
		if refused.Error.Code != openapi.NOTFOUND {
			t.Errorf("a missing class with Accept-Language %q answered %s, want NOT_FOUND", language.accept, refused.Error.Code)
		}
		if refused.Error.Message != language.want {
			t.Errorf("a missing class with Accept-Language %q answered %q, want %q", language.accept, refused.Error.Message, language.want)
		}
	}
}
