package http

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/platform/tabular"
	"strconv"
	"time"
)

const exportTimeLayout = "2006-01-02 15:04"

// ExportResultsCsv writes the roster rows of up to 50 assignments as a CSV
// file a spreadsheet opens, times in the caller's own calendar zone.
func (h Attempts) ExportResultsCsv(ctx context.Context, request openapi.ExportResultsCsvRequestObject) (openapi.ExportResultsCsvResponseObject, error) {
	if h.app == nil || h.app.Queries.ResultsExport == nil {
		return nil, httpx.ErrNotImplemented
	}
	ids := make([]string, len(request.Params.Ids))
	for i, id := range request.Params.Ids {
		ids[i] = id.String()
	}
	export, err := h.app.Queries.ResultsExport.Handle(ctx, query.ResultsExport{AssignmentIDs: ids, Scope: httpapi.ScopeFromContext(ctx)})
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return openapi.ExportResultsCsv404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, httpx.Text(ctx, "Không tìm thấy bài giao.", "The assignment was not found.")))}, nil
	case errors.Is(err, domain.ErrExportTooLarge):
		return openapi.ExportResultsCsv422JSONResponse(tooLargeExport(ctx)), nil
	case err != nil:
		return nil, err
	}

	location := exportLocation(export.Zone)
	var file bytes.Buffer
	if err := writeResultsCsv(ctx, &file, export.Rows, location); err != nil {
		return nil, err
	}
	return openapi.ExportResultsCsv200TextcsvResponse{
		Body:          &file,
		ContentLength: int64(file.Len()),
		Headers: openapi.ExportResultsCsv200ResponseHeaders{
			CacheControl:       "private, no-store",
			ContentDisposition: fmt.Sprintf(`attachment; filename="results-%s.csv"`, time.Now().In(location).Format("20060102")),
		},
	}, nil
}

func tooLargeExport(ctx context.Context) openapi.ErrorResponse {
	message := httpx.Text(ctx,
		fmt.Sprintf("Các bài giao này có hơn %d dòng. Hãy chọn ít bài giao hơn.", domain.MaxExportRows),
		fmt.Sprintf("These assignments hold more than %d rows. Pick fewer.", domain.MaxExportRows))
	resp := httpapi.Error(ctx, openapi.VALIDATIONFAILED, message)
	details := map[string]interface{}{"ids": message}
	resp.Error.Details = &details
	return resp
}

func exportLocation(zone string) *time.Location {
	if location, err := time.LoadLocation(zone); err == nil && zone != "" && zone != "Local" {
		return location
	}
	return time.FixedZone(query.DefaultZone, 7*60*60)
}

func writeResultsCsv(ctx context.Context, out *bytes.Buffer, rows []domain.ResultRow, location *time.Location) error {
	file, err := tabular.NewCSV(out)
	if err != nil {
		return err
	}
	header := []string{
		httpx.Text(ctx, "Bài giao", "Assignment"),
		httpx.Text(ctx, "Phiên bản", "Version"),
		httpx.Text(ctx, "Lớp", "Class"),
		httpx.Text(ctx, "Học viên", "Student"),
		"Email",
		httpx.Text(ctx, "Trạng thái", "Status"),
		httpx.Text(ctx, "Điểm", "Score"),
		httpx.Text(ctx, "Điểm tối đa", "Max"),
		httpx.Text(ctx, "Phần trăm", "Percent"),
		httpx.Text(ctx, "Nộp bài", "Submitted") + " (" + location.String() + ")",
		httpx.Text(ctx, "Số lần rời trang", "Focus lost"),
		httpx.Text(ctx, "Gắn cờ", "Flagged"),
	}
	if err := file.Row(header...); err != nil {
		return err
	}
	for _, r := range rows {
		if err := file.Row(resultCells(ctx, r, location)...); err != nil {
			return err
		}
	}
	return file.Flush()
}

func resultCells(ctx context.Context, r domain.ResultRow, location *time.Location) []string {
	var score, total, percent, submitted, focus string
	if r.Earned != nil && r.Total != nil && *r.Total > 0 {
		score = strconv.FormatFloat(*r.Earned, 'f', -1, 64)
		total = strconv.FormatFloat(*r.Total, 'f', -1, 64)
		percent = strconv.FormatFloat(*r.Earned / *r.Total * 100, 'f', 1, 64)
	}
	if r.SubmittedAt != nil {
		submitted = r.SubmittedAt.In(location).Format(exportTimeLayout)
	}
	if r.FocusLoss != nil {
		focus = strconv.Itoa(*r.FocusLoss)
	}
	flagged := httpx.Text(ctx, "Không", "No")
	if r.Flagged {
		flagged = httpx.Text(ctx, "Có", "Yes")
	}
	return []string{
		r.AssignmentTitle, strconv.Itoa(r.Version), r.Classes, r.StudentName, r.Email,
		stateWord(ctx, r.State), score, total, percent, submitted, focus, flagged,
	}
}

func stateWord(ctx context.Context, state string) string {
	switch state {
	case "not_started":
		return httpx.Text(ctx, "Chưa làm", "Not started")
	case "in_progress":
		return httpx.Text(ctx, "Đang làm", "In progress")
	case "submitted":
		return httpx.Text(ctx, "Đã nộp", "Submitted")
	case "timed_out":
		return httpx.Text(ctx, "Hết giờ", "Timed out")
	case "graded":
		return httpx.Text(ctx, "Đã chấm", "Graded")
	case "voided":
		return httpx.Text(ctx, "Đã huỷ", "Voided")
	default:
		return state
	}
}
