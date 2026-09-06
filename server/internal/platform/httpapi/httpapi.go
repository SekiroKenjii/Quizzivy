package httpapi

import (
	"context"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"quizzivy/gen/openapi"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/stats"
)

// Actor is who is calling and from where, as every audited write records it.
type Actor struct {
	ID        string
	IP        string
	UserAgent string
}

func ActorFromContext(ctx context.Context) (Actor, bool) {
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return Actor{}, false
	}
	meta := httpx.RequestMetaFromContext(ctx)
	return Actor{ID: principal.UserID, IP: meta.IP, UserAgent: meta.UserAgent}, true
}

func ActorID(ctx context.Context) string {
	principal, _ := httpx.PrincipalFromContext(ctx)
	return principal.UserID
}

// Error is the contract's envelope, carrying the request id the logs use.
func Error(ctx context.Context, code openapi.ErrorCode, message string) openapi.ErrorResponse {
	return openapi.ErrorResponse{
		Error: struct {
			Code      openapi.ErrorCode       `json:"code"`
			Details   *map[string]interface{} `json:"details,omitempty"`
			Message   string                  `json:"message"`
			RequestId openapi.Uuid            `json:"requestId"`
		}{
			Code:      code,
			Message:   message,
			RequestId: ParseUUID(httpx.RequestIDFromContext(ctx)),
		},
	}
}

func ErrorWithDetails(ctx context.Context, code openapi.ErrorCode, message string, details map[string]interface{}) openapi.ErrorResponse {
	resp := Error(ctx, code, message)
	resp.Error.Details = &details
	return resp
}

func FieldError(ctx context.Context, field, message string) openapi.ErrorResponse {
	return ErrorWithDetails(ctx, openapi.VALIDATIONFAILED, message, map[string]interface{}{field: message})
}

func NotFound(ctx context.Context, message string) openapi.ErrorResponse {
	return Error(ctx, openapi.NOTFOUND, message)
}

func BlankReason(ctx context.Context) openapi.ErrorResponse {
	return ErrorWithDetails(ctx, openapi.VALIDATIONFAILED, "Cần ghi lý do.",
		map[string]interface{}{"reason": "Lý do không được để trống."})
}

// ParseUUID renders a stored id; ids never come from user input, so a bad one is the zero value, not a 500.
func ParseUUID(s string) openapi.Uuid {
	id, err := uuid.Parse(s)
	if err != nil {
		return openapi.Uuid{}
	}
	return openapi.Uuid(id)
}

func RawUUID(s string) openapi_types.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		return openapi_types.UUID{}
	}
	return id
}

// StudentStats renders the shared roster figures in the contract's shape.
func StudentStats(in stats.Student) openapi.StudentStats {
	out := openapi.StudentStats{
		SubmittedCount: in.SubmittedCount,
		FlaggedCount:   in.FlaggedCount,
		Activity: openapi.StudentActivity{
			Live:          in.LiveAttempt,
			LastAttemptAt: in.LastAttemptAt,
		},
	}
	if in.ScoreEarned != nil && in.ScoreTotal != nil {
		out.Score = &openapi.AttemptScore{
			Earned:        *in.ScoreEarned,
			Total:         *in.ScoreTotal,
			PendingManual: in.PendingManual,
		}
	}
	return out
}

func Ptr[T any](v T) *T { return &v }

func Deref[T any](v *[]T) []T {
	if v == nil {
		return nil
	}
	return *v
}

func DerefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
