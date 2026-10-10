package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/notifications/application"
	"quizzivy/internal/modules/notifications/application/command"
	"quizzivy/internal/modules/notifications/application/query"
	"quizzivy/internal/modules/notifications/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
)

type Notifications struct {
	app    *application.Application
	logger *slog.Logger
	now    func() time.Time
}

func NewNotifications(app *application.Application) Notifications {
	return Notifications{app: app, logger: slog.New(slog.DiscardHandler), now: time.Now}
}

// WithLogger returns the transport logging to logger what it does not answer
// the caller with: a due notification it could not write.
func (h Notifications) WithLogger(logger *slog.Logger) Notifications {
	h.logger = logger
	return h
}

func (h Notifications) materialise(ctx context.Context, userID string) {
	if h.app.Commands.MaterialiseDue == nil {
		return
	}
	if _, err := h.app.Commands.MaterialiseDue.Handle(ctx, command.MaterialiseDue{UserID: userID, Now: h.now()}); err != nil {
		h.logger.WarnContext(ctx, "due notifications not materialised", "error", err)
	}
}

func (h Notifications) caller(ctx context.Context) (string, bool) {
	if h.app == nil {
		return "", false
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	return principal.UserID, ok && principal.UserID != ""
}

// ListNotifications implements GET /me/notifications for the caller alone.
func (h Notifications) ListNotifications(ctx context.Context, request openapi.ListNotificationsRequestObject) (openapi.ListNotificationsResponseObject, error) {
	userID, ok := h.caller(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	h.materialise(ctx, userID)
	q := query.List{UserID: userID}
	if request.Params.Before != nil {
		q.Before = request.Params.Before.String()
	}
	if request.Params.Limit != nil {
		q.Limit = *request.Params.Limit
	}
	page, err := h.app.Queries.List.Handle(ctx, q)
	if err != nil {
		return nil, err
	}
	out := openapi.ListNotifications200JSONResponse{Items: make([]openapi.Notification, len(page.Items))}
	for i, n := range page.Items {
		if out.Items[i], err = toAPINotification(n); err != nil {
			return nil, err
		}
	}
	if page.NextBefore != "" {
		out.NextBefore = new(httpapi.RawUUID(page.NextBefore))
	}
	return out, nil
}

func toAPINotification(n domain.Notification) (openapi.Notification, error) {
	out := openapi.Notification{
		Id:        httpapi.ParseUUID(n.ID),
		Kind:      openapi.NotificationKind(n.Kind),
		CreatedAt: n.CreatedAt,
		ReadAt:    n.ReadAt,
	}
	if err := json.Unmarshal(n.Params, &out.Params); err != nil {
		return openapi.Notification{}, fmt.Errorf("notifications: params of %s: %w", n.ID, err)
	}
	if n.Target != nil {
		out.Target = &openapi.NotificationTarget{
			Route:        openapi.NotificationTargetRoute(n.Target.Route),
			AssignmentId: optionalID(n.Target.AssignmentID),
			AttemptId:    optionalID(n.Target.AttemptID),
		}
	}
	return out, nil
}

func optionalID(id string) *openapi.Uuid {
	if id == "" {
		return nil
	}
	return new(httpapi.ParseUUID(id))
}

// MarkNotificationsRead implements POST /me/notifications/read. It answers
// 204 whatever matched, so an id that is not the caller's is answered as an
// id that does not exist.
func (h Notifications) MarkNotificationsRead(ctx context.Context, request openapi.MarkNotificationsReadRequestObject) (openapi.MarkNotificationsReadResponseObject, error) {
	userID, ok := h.caller(ctx)
	if !ok || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	var err error
	if request.Body.Ids == nil {
		_, err = h.app.Commands.MarkAllRead.Handle(ctx, command.MarkAllRead{UserID: userID})
	} else {
		ids := make([]string, len(*request.Body.Ids))
		for i, id := range *request.Body.Ids {
			ids[i] = id.String()
		}
		_, err = h.app.Commands.MarkRead.Handle(ctx, command.MarkRead{UserID: userID, IDs: ids})
	}
	switch {
	case errors.Is(err, domain.ErrNoIDs):
		return openapi.MarkNotificationsRead400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(
			httpapi.FieldError(ctx, "ids", httpx.Text(ctx, "Cần chọn ít nhất một thông báo.", "At least one notification is needed.")))}, nil
	case errors.Is(err, domain.ErrTooManyIDs):
		return openapi.MarkNotificationsRead400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(
			httpapi.FieldError(ctx, "ids", fmt.Sprintf(httpx.Text(ctx, "Chỉ đánh dấu được tối đa %d thông báo mỗi lần.", "At most %d notifications can be marked at a time."), domain.MaxMarkedIDs)))}, nil
	case err != nil:
		return nil, err
	}
	return openapi.MarkNotificationsRead204Response{}, nil
}

// GetMySummary implements GET /me/summary: the caller's unread count.
func (h Notifications) GetMySummary(ctx context.Context, _ openapi.GetMySummaryRequestObject) (openapi.GetMySummaryResponseObject, error) {
	userID, ok := h.caller(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	h.materialise(ctx, userID)
	summary, err := h.app.Queries.Summary.Handle(ctx, query.Summary{UserID: userID})
	if err != nil {
		return nil, err
	}
	return openapi.GetMySummary200JSONResponse{UnreadNotifications: summary.UnreadNotifications}, nil
}
