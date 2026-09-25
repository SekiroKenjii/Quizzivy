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
func (f *failure) VisitProcessWordImportResponse(w http.ResponseWriter) error    { return f.write(w) }
func (f *failure) VisitCancelWordImportResponse(w http.ResponseWriter) error     { return f.write(w) }
func (f *failure) VisitGetWordImportReviewResponse(w http.ResponseWriter) error  { return f.write(w) }
func (f *failure) VisitSaveWordImportReviewResponse(w http.ResponseWriter) error { return f.write(w) }
func (f *failure) VisitGetWordImportSourceResponse(w http.ResponseWriter) error  { return f.write(w) }
func (f *failure) VisitCommitWordImportResponse(w http.ResponseWriter) error     { return f.write(w) }
func (f *failure) VisitAdoptWordImportReprocessedResponse(w http.ResponseWriter) error {
	return f.write(w)
}

func importFailure(ctx context.Context, err error) (*failure, error) {
	type translation struct {
		err    error
		status int
		code   openapi.ErrorCode
		vi, en string
	}
	messages := []translation{
		{domain.ErrNotFound, 404, openapi.NOTFOUND, "Không tìm thấy lượt nhập hoặc tệp đã hoàn tất.", "The import or completed source was not found."},
		{domain.ErrNoDraft, 404, openapi.IMPORTNOTPROCESSED, "Tài liệu chưa được xử lý xong nên chưa có bản rà soát.", "The document has not finished processing, so there is nothing to review yet."},
		{domain.ErrStale, 409, openapi.STALEWRITE, "Bản nhập đã thay đổi ở nơi khác. Hãy tải lại để xem bản mới nhất.", "The import was changed elsewhere. Reload to see the latest version."},
		{domain.ErrNotReady, 422, openapi.IMPORTNOTREADY, "Vẫn còn mục cần xử lý hoặc cần xác nhận trước khi tạo đề.", "Some items still need fixing or a decision before the test can be created."},
		{domain.ErrBadDraft, 422, openapi.VALIDATIONFAILED, "Bản rà soát gửi lên không hợp lệ. Hãy tải lại trang và thử lại.", "The submitted review is malformed. Reload the page and try again."},
		{domain.ErrConflict, 409, openapi.IMPORTCONFLICT, "Lượt nhập đã thay đổi hoặc mã tải lên đã được dùng. Hãy tải lại dữ liệu trước khi tiếp tục.", "The import changed or the upload identity was already used. Reload before continuing."},
		{domain.ErrQuota, 429, openapi.IMPORTQUOTAEXCEEDED, "Đã đạt giới hạn lưu trữ hoặc số lượt nhập. Vui lòng liên hệ quản trị viên.", "The storage or import quota has been reached. Contact your administrator."},
		{domain.ErrBusy, 429, openapi.IMPORTBUSY, "Hệ thống đang xử lý tệp khác. Vui lòng thử lại sau.", "The system is busy processing another file. Please retry later."},
		{domain.ErrTooLarge, 413, openapi.IMPORTSOURCETOOLARGE, "Tệp vượt giới hạn 25 MiB hoặc có cấu trúc quá lớn. Hãy chia nhỏ tài liệu.", "The file exceeds 25 MiB or its expanded structure is too large. Split the document."},
		{domain.ErrUnsupported, 415, openapi.IMPORTSOURCEUNSUPPORTED, "Hãy dùng tệp Word (.docx, hoặc .doc khi máy chủ hỗ trợ) không có mật khẩu, macro hoặc đối tượng thực thi.", "Use an unprotected Word file (.docx, or .doc where the server supports it) without macros or active objects."},
		{domain.ErrInvalid, 415, openapi.IMPORTSOURCEINVALID, "Không đọc được cấu trúc Word. Hãy mở tệp trong Word và lưu lại dưới dạng .docx.", "The Word package is invalid. Open it in Word and save a new .docx copy."},
		{domain.ErrProcessingOff, 503, openapi.IMPORTPROCESSINGUNAVAILABLE, "Máy chủ này chưa bật xử lý tài liệu Word nên chưa thể xử lý lượt nhập. Các lượt nhập đã xử lý xong vẫn rà soát và tạo đề được.", "Word processing is not enabled on this server, so the import cannot be processed. Imports that already finished processing can still be reviewed and turned into tests."},
		{domain.ErrFilesRemoved, 410, openapi.IMPORTFILESREMOVED, "Tệp gốc và bản rà soát của lượt nhập này đã được xoá theo chính sách lưu trữ.", "This import's original files and review were removed under the retention policy."},
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
