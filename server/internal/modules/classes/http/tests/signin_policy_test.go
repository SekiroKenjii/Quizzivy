package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/classes/application"
	"quizzivy/internal/modules/classes/domain"
	classeshttp "quizzivy/internal/modules/classes/http"
	organizationmodel "quizzivy/internal/modules/organization/application/model"
	organizationquery "quizzivy/internal/modules/organization/application/query"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/cqrs"
)

const (
	liveCode      = "ABCD2345"
	liveClassID   = "01935000-0000-7000-8000-0000000000c1"
	unknownCode   = "ZZZZ-ZZZZ"
	invalidInVi   = "Mã lớp không đúng. Vui lòng kiểm tra lại."
	invalidInEn   = "That class code is not right. Please check it."
	liveTeacher   = "Cô Lan"
	liveClassName = "Lớp 10A"
)

type liveCodeRepository struct {
	domain.Repository
}

func (r liveCodeRepository) isLive(code domain.JoinCodeLookup) bool {
	legacy := domain.JoinCodes.Hash(liveCode)
	return slices.ContainsFunc(code.Hashes(), func(hash []byte) bool { return bytes.Equal(hash, legacy) })
}

func (r liveCodeRepository) LookupByCode(_ context.Context, code domain.JoinCodeLookup) (*domain.CodeRow, error) {
	if !r.isLive(code) {
		return nil, nil
	}
	teacher := liveTeacher
	return &domain.CodeRow{
		CodeState:   domain.CodeState{SelfJoinEnabled: true, ExpiresAt: time.Now().Add(time.Hour)},
		ClassID:     liveClassID,
		ClassName:   liveClassName,
		Lookup:      domain.StoredLookup{Scheme: domain.LookupLegacy, Hash: domain.JoinCodes.Hash(liveCode)},
		TeacherName: &teacher,
	}, nil
}

func (r liveCodeRepository) Enrol(_ context.Context, in domain.EnrolInput) (domain.EnrolResult, error) {
	if !r.isLive(in.Code) {
		return domain.EnrolResult{Outcome: domain.PreviewInvalid}, nil
	}
	return domain.EnrolResult{
		Outcome:     domain.PreviewOK,
		UserID:      uuid.NewString(),
		Class:       domain.EnrolledClass{ID: liveClassID, Name: liveClassName},
		StudentName: "Trần Thị B",
		TeacherID:   uuid.NewString(),
	}, nil
}

func joiningHandler(joinCodes bool) classeshttp.Classes {
	keys, err := domain.NewJoinCodeKeys([]byte("0123456789abcdef0123456789abcdef"), nil)
	if err != nil {
		panic(err)
	}
	policy := cqrs.HandlerFunc[organizationquery.SignInPolicy, organizationmodel.SignInPolicy](
		func(context.Context, organizationquery.SignInPolicy) (organizationmodel.SignInPolicy, error) {
			return organizationmodel.SignInPolicy{Google: true, Password: true, ForcePasswordChange: true, JoinCodes: joinCodes}, nil
		})
	return classeshttp.NewClasses(application.New(liveCodeRepository{}, nil, keys).WithSignInPolicy(policy))
}

type servedAnswer struct {
	status int
	body   string
}

var requestIDField = regexp.MustCompile(`"requestId":"[^"]*"`)

func serve(t *testing.T, visit func(http.ResponseWriter) error) servedAnswer {
	t.Helper()
	recorder := httptest.NewRecorder()
	if err := visit(recorder); err != nil {
		t.Fatal(err)
	}
	return servedAnswer{status: recorder.Code, body: string(requestIDField.ReplaceAll(recorder.Body.Bytes(), []byte(`"requestId":""`)))}
}

func previewServed(t *testing.T, h classeshttp.Classes, accept, code string) servedAnswer {
	t.Helper()
	response, err := h.PreviewJoinCode(contextIn(t, accept), openapi.PreviewJoinCodeRequestObject{Body: &openapi.PreviewJoinCodeJSONRequestBody{JoinCode: code}})
	if err != nil {
		t.Fatalf("PreviewJoinCode(%q): %v", code, err)
	}
	return serve(t, response.VisitPreviewJoinCodeResponse)
}

func signedInStudentContext(t *testing.T, acceptLanguage string) context.Context {
	t.Helper()
	var ctx context.Context
	student := httpx.Principal{UserID: uuid.NewString()}
	handler := httpx.WithRequestMeta(func(*http.Request) string { return "203.0.113.9" })(
		httpx.RequireAuth(nil, func(string) (httpx.Principal, error) { return student, nil })(
			http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { ctx = r.Context() })))
	request := httptest.NewRequest(http.MethodPost, "/app/classes/join", nil)
	request.Header.Set("Authorization", "Bearer fixture")
	if acceptLanguage != "" {
		request.Header.Set("Accept-Language", acceptLanguage)
	}
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if ctx == nil {
		t.Fatal("the request did not reach the handler")
	}
	return ctx
}

func joinServed(t *testing.T, h classeshttp.Classes, accept, code string) servedAnswer {
	t.Helper()
	response, err := h.JoinClass(signedInStudentContext(t, accept), openapi.JoinClassRequestObject{Body: &openapi.JoinClassJSONRequestBody{JoinCode: code}})
	if err != nil {
		t.Fatalf("JoinClass(%q): %v", code, err)
	}
	return serve(t, response.VisitJoinClassResponse)
}

func errorOf(t *testing.T, answer servedAnswer) (openapi.ErrorCode, string) {
	t.Helper()
	var envelope struct {
		Error struct {
			Code    openapi.ErrorCode `json:"code"`
			Message string            `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(answer.body), &envelope); err != nil {
		t.Fatalf("the answer is not the error envelope: %v: %s", err, answer.body)
	}
	return envelope.Error.Code, envelope.Error.Message
}

func TestJoiningOffAnswersThePreviewAndTheJoinExactlyAsAnUnknownCodeIsAnswered(t *testing.T) {
	on, off := joiningHandler(true), joiningHandler(false)
	for _, language := range []struct {
		name, accept, message string
	}{{"vi", "", invalidInVi}, {"en", "en", invalidInEn}} {
		for _, route := range []struct {
			name string
			call func(*testing.T, classeshttp.Classes, string, string) servedAnswer
		}{{"POST /join/preview", previewServed}, {"POST /app/classes/join", joinServed}} {
			t.Run(language.name+"/"+route.name, func(t *testing.T) {
				unknown := route.call(t, on, language.accept, unknownCode)
				if code, message := errorOf(t, unknown); unknown.status != http.StatusNotFound || code != openapi.JOINCODEINVALID || message != language.message {
					t.Fatalf("an unknown code answered %d %s %q, want 404 JOIN_CODE_INVALID %q", unknown.status, code, message, language.message)
				}
				if live := route.call(t, on, language.accept, liveCode); live.status != http.StatusOK {
					t.Fatalf("with joining on, the live code answered %d: %s", live.status, live.body)
				}

				for _, typed := range []string{liveCode, "ABCD-2345", "abcd-2345", unknownCode} {
					if refused := route.call(t, off, language.accept, typed); refused != unknown {
						t.Errorf("with joining off, %q answered %d %s, want what an unknown code answers, %d %s", typed, refused.status, refused.body, unknown.status, unknown.body)
					}
				}
			})
		}
	}
}
