package http

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/platform/tabular"
)

const (
	maxImportText = 64
	maxImportFile = 5<<20 + 1

	formFile        = "file"
	formDefaultRole = "defaultRole"
	formDelivery    = "delivery"
	formRequestID   = "requestId"
)

type importForm struct {
	file        []byte
	filename    string
	haveFile    bool
	defaultRole domain.ImportDefaultRole
	delivery    domain.ImportDelivery
	requestID   string
}

var errImportForm = errors.New("user import: the upload is not the expected form")

func readImportForm(reader *multipart.Reader) (importForm, error) {
	var form importForm
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			return form, nil
		}
		if err != nil {
			return importForm{}, err
		}
		err = form.take(part)
		_ = part.Close()
		if err != nil {
			return importForm{}, err
		}
	}
}

func (f *importForm) take(part *multipart.Part) error {
	switch part.FormName() {
	case formFile:
		if f.haveFile {
			return errImportForm
		}
		file, err := io.ReadAll(io.LimitReader(part, maxImportFile))
		f.file, f.filename, f.haveFile = file, part.FileName(), err == nil
		return err
	case formDefaultRole, formDelivery, formRequestID:
		value, err := io.ReadAll(io.LimitReader(part, maxImportText))
		if err != nil {
			return err
		}
		f.setText(part.FormName(), strings.TrimSpace(string(value)))
	}
	return nil
}

func (f *importForm) setText(name, value string) {
	switch name {
	case formDefaultRole:
		f.defaultRole = domain.ImportDefaultRole(value)
	case formDelivery:
		f.delivery = domain.ImportDelivery(value)
	case formRequestID:
		f.requestID = value
	}
}

func (f importForm) problem(ctx context.Context, withRequestID bool) (field, message string) {
	switch {
	case !f.haveFile:
		return formFile, httpx.Text(ctx, "Yêu cầu cần đúng một tệp CSV hoặc XLSX.", "The request needs exactly one CSV or XLSX file.")
	case !f.defaultRole.Valid():
		return formDefaultRole, httpx.Text(ctx, "Vai trò mặc định phải là student hoặc teacher.", "The default role must be student or teacher.")
	case !f.delivery.Valid():
		return formDelivery, httpx.Text(ctx, "Cách gửi phải là email_link hoặc sheet.", "The delivery must be email_link or sheet.")
	case withRequestID:
		if _, err := uuid.Parse(f.requestID); err != nil {
			return formRequestID, httpx.Text(ctx, "requestId phải là một UUID.", "The requestId must be a UUID.")
		}
	}
	return "", ""
}

func unrefused(err error) error {
	if errors.Is(err, domain.ErrSetPasswordUnavailable) || errors.Is(err, domain.ErrImportSheetsOff) {
		return httpx.ErrNotImplemented
	}
	return err
}

type importRefusal struct {
	status  int
	code    openapi.ErrorCode
	message string
}

func (r importRefusal) response(ctx context.Context) openapi.ErrorResponse {
	return httpapi.Error(ctx, r.code, r.message)
}

// PreviewUserImport implements POST /admin/user-imports/preview.
func (h Identity) PreviewUserImport(ctx context.Context, request openapi.PreviewUserImportRequestObject) (openapi.PreviewUserImportResponseObject, error) {
	if h.app == nil || h.app.Commands.PreviewUserImport == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := studentRequest(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	form, err := readImportForm(request.Body)
	if overBodyLimit(err) {
		return openapi.PreviewUserImport413JSONResponse(tooLargeRefusal(ctx).response(ctx)), nil
	}
	if field, message := form.problem(ctx, false); err != nil || field != "" {
		if err != nil {
			field, message = formFile, httpx.Text(ctx, "Yêu cầu cần đúng một tệp CSV hoặc XLSX.", "The request needs exactly one CSV or XLSX file.")
		}
		return openapi.PreviewUserImport400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(httpapi.FieldError(ctx, field, message))}, nil
	}

	preview, err := h.app.Commands.PreviewUserImport.Handle(ctx, command.PreviewUserImport{
		Request: req, File: form.file, DefaultRole: form.defaultRole, Delivery: form.delivery,
	})
	if err != nil {
		refusal, ok := refuseImport(ctx, err)
		if !ok {
			return nil, unrefused(err)
		}
		switch refusal.status {
		case http.StatusForbidden:
			return openapi.PreviewUserImport403JSONResponse(refusal.response(ctx)), nil
		case http.StatusRequestEntityTooLarge:
			return openapi.PreviewUserImport413JSONResponse(refusal.response(ctx)), nil
		}
		return openapi.PreviewUserImport422JSONResponse{UserImportRefusedJSONResponse: openapi.UserImportRefusedJSONResponse(refusal.response(ctx))}, nil
	}
	return openapi.PreviewUserImport200JSONResponse(toAPIUserImportPreview(preview)), nil
}

// CreateUserImport implements POST /admin/user-imports.
func (h Identity) CreateUserImport(ctx context.Context, request openapi.CreateUserImportRequestObject) (openapi.CreateUserImportResponseObject, error) {
	if h.app == nil || h.app.Commands.CreateUserImport == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := studentRequest(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	form, err := readImportForm(request.Body)
	if overBodyLimit(err) {
		return openapi.CreateUserImport413JSONResponse(tooLargeRefusal(ctx).response(ctx)), nil
	}
	if field, message := form.problem(ctx, true); err != nil || field != "" {
		if err != nil {
			field, message = formFile, httpx.Text(ctx, "Yêu cầu cần đúng một tệp CSV hoặc XLSX.", "The request needs exactly one CSV or XLSX file.")
		}
		return openapi.CreateUserImport400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(httpapi.FieldError(ctx, field, message))}, nil
	}

	result, err := h.app.Commands.CreateUserImport.Handle(ctx, command.CreateUserImport{
		Request: req, RequestID: form.requestID, Filename: form.filename,
		File: form.file, DefaultRole: form.defaultRole, Delivery: form.delivery,
	})
	if err != nil {
		refusal, ok := refuseImport(ctx, err)
		if !ok {
			return nil, unrefused(err)
		}
		switch refusal.status {
		case http.StatusForbidden:
			return openapi.CreateUserImport403JSONResponse(refusal.response(ctx)), nil
		case http.StatusRequestEntityTooLarge:
			return openapi.CreateUserImport413JSONResponse(refusal.response(ctx)), nil
		}
		return openapi.CreateUserImport422JSONResponse(refusal.response(ctx)), nil
	}
	return openapi.CreateUserImport202JSONResponse(toAPIUserImportJob(result.Job, result.Sheet)), nil
}

// GetUserImport implements GET /admin/user-imports/{id}.
func (h Identity) GetUserImport(ctx context.Context, request openapi.GetUserImportRequestObject) (openapi.GetUserImportResponseObject, error) {
	if h.app == nil || h.app.Queries.GetUserImport == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := studentRequest(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	result, err := h.app.Queries.GetUserImport.Handle(ctx, query.GetUserImport{Request: req, ID: request.Id.String()})
	if errors.Is(err, domain.ErrUserImportNotFound) {
		return openapi.GetUserImport404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, httpx.Text(ctx, "Không tìm thấy lần nhập người dùng này.", "This user import was not found.")))}, nil
	}
	if err != nil {
		return nil, err
	}
	return openapi.GetUserImport200JSONResponse(toAPIUserImportJob(result.Job, result.Sheet)), nil
}

// DownloadUserImportSheet implements POST /admin/user-imports/{id}/sheet.
func (h Identity) DownloadUserImportSheet(ctx context.Context, request openapi.DownloadUserImportSheetRequestObject) (openapi.DownloadUserImportSheetResponseObject, error) {
	if h.app == nil || h.app.Commands.DownloadUserImportSheet == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := studentRequest(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	rows, err := h.app.Commands.DownloadUserImportSheet.Handle(ctx, command.DownloadUserImportSheet{Request: req, ID: request.Id.String()})
	switch {
	case errors.Is(err, domain.ErrUserImportNotFound):
		return openapi.DownloadUserImportSheet404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, httpx.Text(ctx, "Không tìm thấy lần nhập người dùng này.", "This user import was not found.")))}, nil
	case errors.Is(err, domain.ErrSheetUnavailable):
		return openapi.DownloadUserImportSheet410JSONResponse(httpapi.Error(ctx, openapi.SHEETUNAVAILABLE, httpx.Text(ctx,
			"Không còn bảng mật khẩu để tải: lần nhập này chưa xong, gửi qua email, không tạo tài khoản nào, đã được tải một lần hoặc đã quá 24 giờ.",
			"There is no sheet to download: this import is not finished, was sent by email, created no account, was already downloaded, or finished more than 24 hours ago."))), nil
	case err != nil:
		return nil, unrefused(err)
	}
	var file bytes.Buffer
	if err := writeSheet(&file, rows); err != nil {
		return nil, err
	}
	return sheetFile{openapi.DownloadUserImportSheet200TextcsvResponse{
		Body:          &file,
		ContentLength: int64(file.Len()),
		Headers: openapi.DownloadUserImportSheet200ResponseHeaders{
			CacheControl:       "no-store",
			ContentDisposition: fmt.Sprintf(`attachment; filename="new-accounts-%s.csv"`, h.today(ctx, req.ActorID)),
		},
	}}, nil
}

type sheetFile struct {
	openapi.DownloadUserImportSheet200TextcsvResponse
}

func (s sheetFile) VisitDownloadUserImportSheetResponse(w http.ResponseWriter) error {
	return s.DownloadUserImportSheet200TextcsvResponse.VisitDownloadUserImportSheetResponse(charsetWriter{w})
}

type charsetWriter struct{ http.ResponseWriter }

func (c charsetWriter) WriteHeader(status int) {
	if c.Header().Get("Content-Type") == "text/csv" {
		c.Header().Set("Content-Type", "text/csv; charset=utf-8")
	}
	c.ResponseWriter.WriteHeader(status)
}

func writeSheet(out *bytes.Buffer, rows []domain.SheetRow) error {
	file, err := tabular.NewCSV(out)
	if err != nil {
		return err
	}
	if err := file.Row("full_name", "email", "role", "class", "temporary_password"); err != nil {
		return err
	}
	for _, r := range rows {
		if err := file.Row(r.FullName, r.Email, r.Role, r.Class, r.Password); err != nil {
			return err
		}
	}
	return file.Flush()
}

func (h Identity) today(ctx context.Context, userID string) string {
	location := time.FixedZone("ICT", 7*60*60)
	if h.app.Queries.EffectiveZone == nil {
		return time.Now().In(location).Format("20060102")
	}
	if zone, err := h.app.Queries.EffectiveZone.Handle(ctx, query.EffectiveZone{UserID: userID}); err == nil && zone != "" && zone != "Local" {
		if loaded, err := time.LoadLocation(zone); err == nil {
			location = loaded
		}
	}
	return time.Now().In(location).Format("20060102")
}

func toAPIUserImportPreview(p command.UserImportPreview) openapi.UserImportPreview {
	out := openapi.UserImportPreview{
		Rows:         p.Rows,
		NewAccounts:  p.NewAccounts,
		Invalid:      make([]openapi.UserImportIssue, len(p.Issues)),
		InvalidCount: p.InvalidRows,
		Classes: make([]struct {
			Id   openapi.Uuid `json:"id"`
			Name string       `json:"name"`
		}, len(p.Classes)),
	}
	out.ExistingEmails.Count = p.ExistingCount
	out.ExistingEmails.Sample = make([]openapi_types.Email, len(p.ExistingSample))
	for i, email := range p.ExistingSample {
		out.ExistingEmails.Sample[i] = openapi_types.Email(email)
	}
	for i, issue := range p.Issues {
		out.Invalid[i] = openapi.UserImportIssue{
			Row: issue.Line, Field: openapi.UserImportField(issue.Field), Code: openapi.UserImportIssueCode(issue.Code),
		}
	}
	for i, class := range p.Classes {
		out.Classes[i].Id, out.Classes[i].Name = httpapi.ParseUUID(class.ID), class.Name
	}
	if p.Budget != nil {
		out.EmailBudget = &struct {
			Limit          int `json:"limit"`
			RemainingToday int `json:"remainingToday"`
		}{Limit: p.Budget.Limit, RemainingToday: p.Budget.Remaining}
	}
	return out
}

func toAPIUserImportJob(j domain.ImportJob, sheet *domain.ImportSheetState) openapi.UserImportJob {
	out := openapi.UserImportJob{
		Id:        httpapi.ParseUUID(j.ID),
		Status:    openapi.UserImportJobStatus(j.Status),
		Total:     j.Total,
		Processed: j.Processed(),
		Created:   j.Created,
		Skipped:   j.Skipped,
		Invalid:   j.Invalid,
		Delivery:  openapi.UserImportDelivery(j.Delivery),
		CreatedAt: j.CreatedAt,
	}
	if j.ErrorCode != "" {
		out.ErrorCode = httpapi.Ptr(j.ErrorCode)
	}
	if sheet != nil {
		out.Sheet = &struct {
			Available    bool       `json:"available"`
			DownloadedAt *time.Time `json:"downloadedAt"`
			ExpiresAt    *time.Time `json:"expiresAt"`
		}{Available: sheet.Available, DownloadedAt: sheet.DownloadedAt, ExpiresAt: sheet.ExpiresAt}
	}
	return out
}
