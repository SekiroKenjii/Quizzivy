package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/audit/application/command"
	"quizzivy/internal/modules/audit/application/query"
	"quizzivy/internal/modules/audit/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/platform/tabular"
)

const (
	formatCSV  = "csv"
	formatJSON = "json"

	exportWriteBudget = 10 * time.Minute
)

// ExportAuditLog implements GET /admin/audit-log/export. The count, the refusal
// of an export that is too large and the audit of the export all come before
// the first byte; the entries then follow in batches, each read as it is
// written, so the file is never held in memory. A failure after the first byte
// ends the connection without its last chunk, which a client reads as an
// incomplete download.
func (h Audit) ExportAuditLog(ctx context.Context, request openapi.ExportAuditLogRequestObject) (openapi.ExportAuditLogResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	caller, ok := httpapi.ActorFromContext(ctx)
	if !ok {
		return openapi.ExportAuditLog401JSONResponse{UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx))}, nil
	}
	c := exportCommand(caller, request.Params)
	plan, err := h.app.Commands.StartExport.Handle(ctx, c)
	if errors.Is(err, domain.ErrExportTooLarge) {
		return openapi.ExportAuditLog422JSONResponse(tooLarge(ctx)), nil
	}
	if refused, ok := refusal(ctx, err); ok {
		return openapi.ExportAuditLog400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(refused)}, nil
	}
	if err != nil {
		return nil, err
	}
	return h.file(ctx, plan, c.Format), nil
}

func exportCommand(caller httpapi.Actor, p openapi.ExportAuditLogParams) command.StartExport {
	c := command.StartExport{Actor: caller, Period: domain.Week, Format: formatCSV}
	if p.Period != nil {
		c.Period = domain.Range(*p.Period)
	}
	if p.Format != nil {
		c.Format = string(*p.Format)
	}
	if p.Category != nil {
		c.Category = domain.Category(*p.Category)
	}
	if p.Q != nil {
		c.Query = *p.Q
	}
	return c
}

func tooLarge(ctx context.Context) openapi.ErrorResponse {
	return httpapi.Error(ctx, openapi.EXPORTTOOLARGE, fmt.Sprintf(httpx.Text(ctx,
		"Khoảng thời gian này có hơn %d mục. Hãy chọn khoảng ngắn hơn.",
		"This period holds more than %d entries. Pick a shorter one."), domain.MaxExportRows))
}

type exportFile struct {
	contentType string
	filename    string
	logger      *slog.Logger
	write       func(w io.Writer, flush func()) error
}

func (f exportFile) VisitExportAuditLogResponse(w http.ResponseWriter) error {
	header := w.Header()
	header.Set("Content-Type", f.contentType)
	header.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, f.filename))
	header.Set("Cache-Control", "no-store")
	control := http.NewResponseController(w)
	_ = control.SetWriteDeadline(time.Now().Add(exportWriteBudget))
	w.WriteHeader(http.StatusOK)
	flush := func() { _ = control.Flush() }
	if err := f.write(w, flush); err != nil {
		f.logger.Error("audit export interrupted", "file", f.filename, "error", err)
		panic(http.ErrAbortHandler)
	}
	return nil
}

func (h Audit) file(ctx context.Context, plan domain.ExportPlan, format string) openapi.ExportAuditLogResponseObject {
	loc := domain.LocationOf(plan.Zone)
	name := "audit-log-" + plan.At.In(loc).Format("20060102")
	if format == formatJSON {
		return exportFile{contentType: "application/json", filename: name + ".json", logger: h.logger,
			write: func(w io.Writer, flush func()) error { return h.writeJSON(ctx, w, flush, plan) }}
	}
	return exportFile{contentType: "text/csv; charset=utf-8", filename: name + ".csv", logger: h.logger,
		write: func(w io.Writer, flush func()) error { return h.writeCSV(ctx, w, flush, plan, loc) }}
}

func (h Audit) eachBatch(ctx context.Context, plan domain.ExportPlan, emit func([]domain.Entry) error) error {
	var after *domain.Cursor
	for {
		batch, err := h.app.Queries.ExportBatch.Handle(ctx, query.ExportBatch{Filter: plan.Filter, After: after})
		if err != nil {
			return err
		}
		if err := emit(batch.Entries); err != nil {
			return err
		}
		if batch.Next == nil {
			return nil
		}
		after = batch.Next
	}
}

func (h Audit) writeJSON(ctx context.Context, w io.Writer, flush func(), plan domain.ExportPlan) error {
	if _, err := io.WriteString(w, "["); err != nil {
		return err
	}
	encoder := json.NewEncoder(w)
	written := 0
	err := h.eachBatch(ctx, plan, func(entries []domain.Entry) error {
		for _, entry := range entries {
			if written > 0 {
				if _, err := io.WriteString(w, ","); err != nil {
					return err
				}
			}
			if err := encoder.Encode(toAPIEntry(entry)); err != nil {
				return err
			}
			written++
		}
		flush()
		return nil
	})
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, "]\n")
	return err
}

func (h Audit) writeCSV(ctx context.Context, w io.Writer, flush func(), plan domain.ExportPlan, loc *time.Location) error {
	file, err := tabular.NewCSV(w)
	if err != nil {
		return err
	}
	lang := domain.Language(httpx.Text(ctx, string(domain.Vietnamese), string(domain.English)))
	if err := file.Row(csvHeader(ctx, loc)...); err != nil {
		return err
	}
	err = h.eachBatch(ctx, plan, func(entries []domain.Entry) error {
		for _, entry := range entries {
			if err := file.Row(csvCells(ctx, entry, lang, loc)...); err != nil {
				return err
			}
		}
		if err := file.Flush(); err != nil {
			return err
		}
		flush()
		return nil
	})
	if err != nil {
		return err
	}
	return file.Flush()
}

func csvHeader(ctx context.Context, loc *time.Location) []string {
	return []string{
		"ID",
		httpx.Text(ctx, "Thời gian", "Time") + " (" + loc.String() + ")",
		httpx.Text(ctx, "Người thực hiện", "Actor"),
		httpx.Text(ctx, "Mã người thực hiện", "Actor ID"),
		httpx.Text(ctx, "Hành động", "Action"),
		httpx.Text(ctx, "Mã hành động", "Action key"),
		httpx.Text(ctx, "Nhóm", "Category"),
		httpx.Text(ctx, "Loại đối tượng", "Entity type"),
		httpx.Text(ctx, "Đối tượng", "Entity"),
		httpx.Text(ctx, "Mã đối tượng", "Entity ID"),
		"IP",
		httpx.Text(ctx, "Thiết bị", "Device"),
		httpx.Text(ctx, "Thay đổi", "Changes"),
		httpx.Text(ctx, "Chi tiết", "Details"),
	}
}

func csvCells(ctx context.Context, e domain.Entry, lang domain.Language, loc *time.Location) []string {
	actor, actorID := httpx.Text(ctx, "Hệ thống", "System"), ""
	if e.Actor != nil {
		actor, actorID = e.Actor.Name, e.Actor.ID
	}
	var category, label, entityID, ip string
	if e.Category != nil {
		category = categoryName(ctx, *e.Category)
	}
	if e.Label != nil {
		label = *e.Label
	}
	if e.EntityID != nil {
		entityID = *e.EntityID
	}
	if e.IP != nil {
		ip = *e.IP
	}
	return []string{
		strconv.FormatInt(e.ID, 10), e.OccurredAt.In(loc).Format(time.RFC3339), actor, actorID,
		domain.Sentence(e.Action, lang), e.Action, category, e.EntityType, label, entityID,
		ip, deviceOf(e.UserAgent), changesCell(e.Changes), detailsCell(e.Details),
	}
}

func categoryName(ctx context.Context, c domain.Category) string {
	switch c {
	case domain.Accounts:
		return httpx.Text(ctx, "Tài khoản", "Accounts")
	case domain.Classes:
		return httpx.Text(ctx, "Lớp học", "Classes")
	case domain.Attempts:
		return httpx.Text(ctx, "Bài làm", "Attempts")
	case domain.Tests:
		return httpx.Text(ctx, "Bài kiểm tra", "Tests")
	case domain.Media:
		return httpx.Text(ctx, "Phương tiện", "Media")
	case domain.Imports:
		return httpx.Text(ctx, "Nhập liệu", "Imports")
	}
	return string(c)
}

func changesCell(changes []domain.Change) string {
	parts := make([]string, len(changes))
	for i, change := range changes {
		parts[i] = change.Field + ": " + valueText(change.From) + " -> " + valueText(change.To)
	}
	return strings.Join(parts, "; ")
}

func detailsCell(details map[string]any) string {
	if details == nil {
		return ""
	}
	encoded, err := json.Marshal(details)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func valueText(v any) string {
	if text, ok := v.(string); ok {
		return text
	}
	encoded, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(encoded)
}
