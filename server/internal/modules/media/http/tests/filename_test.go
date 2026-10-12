package http_test

import (
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/media/application"
	"quizzivy/internal/modules/media/application/command"
	"quizzivy/internal/modules/media/application/query"
	"quizzivy/internal/modules/media/domain"
	mediahttp "quizzivy/internal/modules/media/http"
	"quizzivy/internal/shared/cqrs"
)

func nulFilenameUpload() *multipart.Reader {
	const part = "--test\r\nContent-Disposition: form-data; name=\"file\"; filename*=UTF-8''a%00b.png\r\n\r\nfile\r\n--test--\r\n"
	return multipart.NewReader(strings.NewReader(part), "test")
}

func requireFilenameRefusal(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	var body struct {
		Error struct {
			Code    string         `json:"code"`
			Message string         `json:"message"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != 400 || body.Error.Code != "VALIDATION_FAILED" || body.Error.Details["filename"] != body.Error.Message {
		t.Fatalf("answer = %d %+v, want 400 VALIDATION_FAILED naming the filename", recorder.Code, body.Error)
	}
}

func TestAnUploadWhoseFileNameHoldsANulAnswersValidationFailedNamingTheFileName(t *testing.T) {
	var handed string
	app := &application.Application{Commands: application.Commands{Upload: cqrs.HandlerFunc[command.Upload, domain.Asset](func(_ context.Context, in command.Upload) (domain.Asset, error) {
		handed = in.Filename
		return domain.Asset{}, domain.ErrInvalidFilename
	})}}

	response, err := mediahttp.NewMedia(app).UploadMedia(contextAs(t, principals()["a Teacher"]), openapi.UploadMediaRequestObject{Body: nulFilenameUpload()})
	if err != nil {
		t.Fatal(err)
	}
	if handed != "a\x00b.png" {
		t.Fatalf("the command was handed %q: an RFC 2231 file name carries the NUL to it", handed)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitUploadMediaResponse(recorder); err != nil {
		t.Fatal(err)
	}
	requireFilenameRefusal(t, recorder)
}

func TestAReplacementWhoseFileNameHoldsANulAnswersValidationFailedNamingTheFileName(t *testing.T) {
	app := &application.Application{
		Queries: application.Queries{ReplacementTarget: cqrs.HandlerFunc[query.ReplacementTarget, domain.ReplacementTarget](func(context.Context, query.ReplacementTarget) (domain.ReplacementTarget, error) {
			return domain.ReplacementTarget{}, nil
		})},
		Commands: application.Commands{Replace: cqrs.HandlerFunc[command.Replace, domain.ReplaceResult](func(context.Context, command.Replace) (domain.ReplaceResult, error) {
			return domain.ReplaceResult{}, domain.ErrInvalidFilename
		})},
	}

	response, err := mediahttp.NewMedia(app).ReplaceMedia(contextAs(t, principals()["a Teacher"]), openapi.ReplaceMediaRequestObject{Id: uuid.New(), Body: nulFilenameUpload()})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitReplaceMediaResponse(recorder); err != nil {
		t.Fatal(err)
	}
	requireFilenameRefusal(t, recorder)
}
