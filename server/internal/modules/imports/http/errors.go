package http

import (
	"context"
	"encoding/json"
	"errors"
	"golang.org/x/text/language"
	"net/http"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
)

var errMultipart = errors.New("imports: malformed multipart")
var importLanguages = language.NewMatcher([]language.Tag{language.Vietnamese, language.English})

type failure struct {
	status int
	body   openapi.ErrorResponse
}

func (f *failure) write(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if f.status == http.StatusTooManyRequests {
		w.Header().Set("Retry-After", "10")
	}
	w.WriteHeader(f.status)
	return json.NewEncoder(w).Encode(f.body)
}
func (f *failure) VisitCreateWordImportResponse(w http.ResponseWriter) error     { return f.write(w) }
func (f *failure) VisitGetWordImportResponse(w http.ResponseWriter) error        { return f.write(w) }
func (f *failure) VisitUploadImportSourceResponse(w http.ResponseWriter) error   { return f.write(w) }
func (f *failure) VisitDownloadImportSourceResponse(w http.ResponseWriter) error { return f.write(w) }

func importFailure(ctx context.Context, err error) (*failure, error) {
	type translation struct {
		err    error
		status int
		code   openapi.ErrorCode
		vi, en string
	}
	messages := []translation{
		{domain.ErrNotFound, 404, openapi.NOTFOUND, "Không tìm thấy lượt nhập hoặc tệp đã hoàn tất.", "The import or completed source was not found."},
		{domain.ErrConflict, 409, openapi.IMPORTCONFLICT, "Lượt nhập đã thay đổi hoặc mã tải lên đã được dùng. Hãy tải lại dữ liệu trước khi tiếp tục.", "The import changed or the upload identity was already used. Reload before continuing."},
		{domain.ErrQuota, 429, openapi.IMPORTQUOTAEXCEEDED, "Đã đạt giới hạn lưu trữ hoặc số lượt nhập. Vui lòng liên hệ quản trị viên.", "The storage or import quota has been reached. Contact your administrator."},
		{domain.ErrBusy, 429, openapi.IMPORTBUSY, "Hệ thống đang xử lý tệp khác. Vui lòng thử lại sau.", "The system is busy processing another file. Please retry later."},
		{domain.ErrTooLarge, 413, openapi.IMPORTSOURCETOOLARGE, "Tệp vượt giới hạn 25 MiB hoặc có cấu trúc quá lớn. Hãy chia nhỏ tài liệu.", "The file exceeds 25 MiB or its expanded structure is too large. Split the document."},
		{domain.ErrUnsupported, 415, openapi.IMPORTSOURCEUNSUPPORTED, "Hãy dùng tệp .docx không có mật khẩu, macro hoặc đối tượng thực thi. Tệp .doc chưa được bật ở bản này.", "Use an unprotected .docx without macros or active objects. Legacy .doc intake is not enabled in this build."},
		{domain.ErrInvalid, 415, openapi.IMPORTSOURCEINVALID, "Không đọc được cấu trúc Word. Hãy mở tệp trong Word và lưu lại dưới dạng .docx.", "The Word package is invalid. Open it in Word and save a new .docx copy."},
		{errMultipart, 400, openapi.VALIDATIONFAILED, "Yêu cầu phải chứa đúng một tệp trong trường file.", "The request must contain exactly one file part named file."},
	}
	for _, m := range messages {
		if errors.Is(err, m.err) {
			message := m.vi
			if _, index := language.MatchStrings(importLanguages, httpx.RequestMetaFromContext(ctx).Language); index == 1 {
				message = m.en
			}
			return &failure{status: m.status, body: httpapi.Error(ctx, m.code, message)}, nil
		}
	}
	return nil, err
}
