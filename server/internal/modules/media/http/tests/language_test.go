package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/media/application"
	"quizzivy/internal/modules/media/application/command"
	"quizzivy/internal/modules/media/application/model"
	"quizzivy/internal/modules/media/application/query"
	"quizzivy/internal/modules/media/domain"
	mediahttp "quizzivy/internal/modules/media/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/cqrs"
)

type refused struct {
	status  int
	code    openapi.ErrorCode
	message string
}

type refusing func(ctx context.Context, w http.ResponseWriter) error

func refusedIn(t *testing.T, acceptLanguage string, serve refusing) refused {
	t.Helper()
	handler := httpx.WithRequestMeta(func(*http.Request) string { return "203.0.113.9" })(
		httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
			return httpx.Principal{UserID: uuid.NewString()}, nil
		})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := serve(r.Context(), w); err != nil {
				t.Fatal(err)
			}
		})))
	request := httptest.NewRequest(http.MethodPost, "/teacher/media", nil)
	request.Header.Set("Authorization", "Bearer fixture")
	if acceptLanguage != "" {
		request.Header.Set("Accept-Language", acceptLanguage)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("%v: %s", err, response.Body.String())
	}
	return refused{status: response.Code, code: body.Error.Code, message: body.Error.Message}
}

func deleting(failure error) refusing {
	transport := mediahttp.NewMedia(&application.Application{Commands: application.Commands{
		Delete: cqrs.HandlerFunc[command.Delete, cqrs.Nothing](func(context.Context, command.Delete) (cqrs.Nothing, error) {
			return cqrs.Nothing{}, failure
		}),
	}})
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.DeleteMedia(ctx, openapi.DeleteMediaRequestObject{Id: uuid.New()})
		if err != nil {
			return err
		}
		return response.VisitDeleteMediaResponse(w)
	}
}

func minting(failure error) refusing {
	transport := mediahttp.NewMedia(&application.Application{Queries: application.Queries{
		MintForStudent: cqrs.HandlerFunc[query.MintForStudent, model.SignedURLResult](func(context.Context, query.MintForStudent) (model.SignedURLResult, error) {
			return model.SignedURLResult{}, failure
		}),
	}})
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.GetMediaUrl(ctx, openapi.GetMediaUrlRequestObject{AssetId: uuid.New()})
		if err != nil {
			return err
		}
		return response.VisitGetMediaUrlResponse(w)
	}
}

func uploading(filename string, failure error) refusing {
	transport := mediahttp.NewMedia(&application.Application{Commands: application.Commands{
		Upload: cqrs.HandlerFunc[command.Upload, domain.Asset](func(context.Context, command.Upload) (domain.Asset, error) {
			return domain.Asset{}, failure
		}),
	}})
	return func(ctx context.Context, w http.ResponseWriter) error {
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		if filename != "" {
			part, err := form.CreateFormFile("file", filename)
			if err != nil {
				return err
			}
			if _, err := part.Write([]byte("ID3")); err != nil {
				return err
			}
		}
		if err := form.Close(); err != nil {
			return err
		}
		response, err := transport.UploadMedia(ctx, openapi.UploadMediaRequestObject{Body: multipart.NewReader(&body, form.Boundary())})
		if err != nil {
			return err
		}
		return response.VisitUploadMediaResponse(w)
	}
}

func updating(body openapi.MediaUpdate, failure error) refusing {
	transport := mediahttp.NewMedia(&application.Application{Commands: application.Commands{
		Update: cqrs.HandlerFunc[command.Update, domain.Asset](func(context.Context, command.Update) (domain.Asset, error) {
			return domain.Asset{}, failure
		}),
	}})
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.UpdateMedia(ctx, openapi.UpdateMediaRequestObject{Id: uuid.New(), Body: &body})
		if err != nil {
			return err
		}
		return response.VisitUpdateMediaResponse(w)
	}
}

func TestMediaRefusalsSpeakTheCallersLanguage(t *testing.T) {
	renamed := "Tên mới"
	rename := openapi.MediaUpdate{DisplayName: &renamed}
	testID := uuid.NewString()
	inATest := &domain.ReferencedError{Tests: []domain.TestRef{{ID: testID, Title: "Published", Version: 2}}}
	inAGroup := &domain.ReferencedError{
		Tests:  []domain.TestRef{{ID: testID, Title: "Published", Version: 2}},
		Groups: []domain.GroupRef{{ID: uuid.NewString(), Title: "Shared context", TestID: &testID}},
	}
	for _, c := range []struct {
		name   string
		serve  refusing
		status int
		code   openapi.ErrorCode
		vi     string
		en     string
	}{
		{
			name:   "deleting a file that is not there",
			serve:  deleting(domain.ErrNotFound),
			status: http.StatusNotFound,
			code:   openapi.NOTFOUND,
			vi:     "Không tìm thấy tệp.",
			en:     "The file was not found.",
		},
		{
			name:   "deleting a file a published test uses",
			serve:  deleting(inATest),
			status: http.StatusConflict,
			code:   openapi.MEDIAREFERENCED,
			vi:     "Tệp đang được dùng trong một đề đã xuất bản nên không thể xoá.",
			en:     "The file is used in a published test, so it cannot be deleted.",
		},
		{
			name:   "deleting a file a question group uses",
			serve:  deleting(inAGroup),
			status: http.StatusConflict,
			code:   openapi.MEDIAREFERENCED,
			vi:     "Tệp đang được dùng trong nhóm câu hỏi hoặc đề đã xuất bản nên không thể xoá.",
			en:     "The file is used in a question group or a published test, so it cannot be deleted.",
		},
		{
			name:   "a student asking for a file they cannot reach",
			serve:  minting(domain.ErrForbidden),
			status: http.StatusForbidden,
			code:   openapi.FORBIDDEN,
			vi:     "Bạn không có quyền truy cập tệp này.",
			en:     "You do not have permission to access this file.",
		},
		{
			name:   "an upload with no file",
			serve:  uploading("", nil),
			status: http.StatusUnsupportedMediaType,
			code:   openapi.VALIDATIONFAILED,
			vi:     "Không tìm thấy tệp trong yêu cầu tải lên.",
			en:     "The upload request holds no file.",
		},
		{
			name:   "an upload over the size limit",
			serve:  uploading("bai-nghe.mp3", domain.ErrTooLarge),
			status: http.StatusRequestEntityTooLarge,
			code:   openapi.MEDIATOOLARGE,
			vi:     "Tệp vượt quá 50 MB. Vui lòng nén hoặc cắt ngắn tệp.",
			en:     "The file is larger than 50 MB. Please compress or shorten it.",
		},
		{
			name:   "an upload cut off by the transport's own cap",
			serve:  uploading("bai-nghe.mp3", fmt.Errorf("media: reading upload: %w", &http.MaxBytesError{Limit: 52559872})),
			status: http.StatusRequestEntityTooLarge,
			code:   openapi.MEDIATOOLARGE,
			vi:     "Tệp vượt quá 50 MB. Vui lòng nén hoặc cắt ngắn tệp.",
			en:     "The file is larger than 50 MB. Please compress or shorten it.",
		},
		{
			name:   "an image over the image limit",
			serve:  uploading("ban-do.png", domain.ErrImageTooLarge),
			status: http.StatusRequestEntityTooLarge,
			code:   openapi.MEDIATOOLARGE,
			vi:     "Ảnh vượt quá 10 MB. Vui lòng dùng ảnh nhỏ hơn.",
			en:     "The image is larger than 10 MB. Please use a smaller one.",
		},
		{
			name:   "an upload into a full library",
			serve:  uploading("bai-nghe.mp3", domain.ErrQuotaExceeded),
			status: http.StatusConflict,
			code:   openapi.MEDIAQUOTAEXCEEDED,
			vi:     "Thư viện đã hết dung lượng. Hãy xoá bớt tệp rồi tải lại.",
			en:     "Your media library is full. Delete some files, then upload again.",
		},
		{
			name:   "an image uploaded with a play limit",
			serve:  uploading("ban-do.png", domain.ErrPlayLimitOnImage),
			status: http.StatusBadRequest,
			code:   openapi.VALIDATIONFAILED,
			vi:     "Chỉ tệp âm thanh mới có giới hạn số lần nghe.",
			en:     "Only an audio file takes a play limit.",
		},
		{
			name:   "an upload with a play limit out of range",
			serve:  uploading("bai-nghe.mp3", domain.ErrInvalidPlayLimit),
			status: http.StatusBadRequest,
			code:   openapi.VALIDATIONFAILED,
			vi:     "Giới hạn số lần nghe phải từ 0 đến 3.",
			en:     "The play limit must be between 0 and 3.",
		},
		{
			name:   "renaming a file that is not there",
			serve:  updating(rename, domain.ErrNotFound),
			status: http.StatusNotFound,
			code:   openapi.NOTFOUND,
			vi:     "Không tìm thấy tệp.",
			en:     "The file was not found.",
		},
		{
			name:   "renaming a file to a blank name",
			serve:  updating(rename, domain.ErrInvalidName),
			status: http.StatusBadRequest,
			code:   openapi.VALIDATIONFAILED,
			vi:     "Tên tệp phải có từ 1 đến 200 ký tự.",
			en:     "The file name must be 1 to 200 characters long.",
		},
		{
			name:   "a play limit on an image",
			serve:  updating(rename, domain.ErrPlayLimitOnImage),
			status: http.StatusBadRequest,
			code:   openapi.VALIDATIONFAILED,
			vi:     "Chỉ tệp âm thanh mới có giới hạn số lần nghe.",
			en:     "Only an audio file takes a play limit.",
		},
		{
			name:   "a play limit out of range",
			serve:  updating(rename, domain.ErrInvalidPlayLimit),
			status: http.StatusBadRequest,
			code:   openapi.VALIDATIONFAILED,
			vi:     "Giới hạn số lần nghe phải từ 0 đến 3.",
			en:     "The play limit must be between 0 and 3.",
		},
		{
			name:   "a play limit that is not a number",
			serve:  updating(openapi.MediaUpdate{DefaultMaxPlays: json.RawMessage(`"hai"`)}, nil),
			status: http.StatusBadRequest,
			code:   openapi.VALIDATIONFAILED,
			vi:     "Giới hạn số lần nghe phải từ 0 đến 3.",
			en:     "The play limit must be between 0 and 3.",
		},
		{
			name:   "an update that changes nothing",
			serve:  updating(rename, domain.ErrNothingToUpdate),
			status: http.StatusBadRequest,
			code:   openapi.VALIDATIONFAILED,
			vi:     "Cần ít nhất một thay đổi.",
			en:     "At least one change is needed.",
		},
		{
			name:   "an upload over the length limit",
			serve:  uploading("bai-nghe.mp3", domain.ErrTooLong),
			status: http.StatusUnsupportedMediaType,
			code:   openapi.MEDIATOOLONG,
			vi:     "Tệp âm thanh dài hơn 5 phút. Vui lòng cắt ngắn.",
			en:     "The audio file is longer than 5 minutes. Please shorten it.",
		},
		{
			name:   "an upload whose length cannot be read",
			serve:  uploading("bai-nghe.mp3", domain.ErrUnmeasurable),
			status: http.StatusUnsupportedMediaType,
			code:   openapi.MEDIAUNREADABLE,
			vi:     "Không đọc được tệp âm thanh này. Tệp có thể bị lỗi hoặc chưa tải lên hết.",
			en:     "This audio file cannot be read. It may be damaged or not fully uploaded.",
		},
		{
			name:   "an upload of a type that is not accepted",
			serve:  uploading("bai-nghe.wav", domain.ErrUnsupportedType),
			status: http.StatusUnsupportedMediaType,
			code:   openapi.MEDIATYPEUNSUPPORTED,
			vi:     "Chỉ hỗ trợ mp3, m4a và ảnh png/jpg/webp.",
			en:     "Only mp3, m4a and png, jpg or webp images are supported.",
		},
	} {
		for _, language := range []struct {
			accept string
			want   string
		}{{"", c.vi}, {"en", c.en}} {
			got := refusedIn(t, language.accept, c.serve)
			if got.status != c.status || got.code != c.code {
				t.Errorf("%s with Accept-Language %q answered %d %s, want %d %s", c.name, language.accept, got.status, got.code, c.status, c.code)
			}
			if got.message != language.want {
				t.Errorf("%s with Accept-Language %q answered %q, want %q", c.name, language.accept, got.message, language.want)
			}
		}
	}
}

func TestARefusedFieldIsNamedInTheDetails(t *testing.T) {
	renamed := "Tên mới"
	for name, c := range map[string]struct {
		serve refusing
		field string
	}{
		"an image uploaded with a play limit": {uploading("ban-do.png", domain.ErrPlayLimitOnImage), "defaultMaxPlays"},
		"a play limit on an image":            {updating(openapi.MediaUpdate{DisplayName: &renamed}, domain.ErrPlayLimitOnImage), "defaultMaxPlays"},
		"a blank name":                        {updating(openapi.MediaUpdate{DisplayName: &renamed}, domain.ErrInvalidName), "displayName"},
	} {
		handler := httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
			return httpx.Principal{UserID: uuid.NewString()}, nil
		})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := c.serve(r.Context(), w); err != nil {
				t.Fatal(err)
			}
		}))
		request := httptest.NewRequest(http.MethodPost, "/teacher/media", nil)
		request.Header.Set("Authorization", "Bearer fixture")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		var body struct {
			Error struct {
				Message string            `json:"message"`
				Details map[string]string `json:"details"`
			} `json:"error"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Error.Details) != 1 || body.Error.Details[c.field] == "" || body.Error.Details[c.field] != body.Error.Message {
			t.Errorf("%s names %v, want only %q with the message", name, body.Error.Details, c.field)
		}
	}
}
