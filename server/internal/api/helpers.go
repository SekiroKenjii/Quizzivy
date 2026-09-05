package api

import (
	"context"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"quizzivy/gen/openapi"
	"quizzivy/internal/platform/httpapi"
)

func authError(ctx context.Context, code openapi.ErrorCode, message string) openapi.ErrorResponse {
	return httpapi.Error(ctx, code, message)
}

func fieldError(ctx context.Context, field, message string) openapi.ErrorResponse {
	return httpapi.FieldError(ctx, field, message)
}

func notFound(ctx context.Context, message string) openapi.ErrorResponse {
	return httpapi.NotFound(ctx, message)
}

func blankReason(ctx context.Context) openapi.ErrorResponse { return httpapi.BlankReason(ctx) }

func parseUUID(s string) openapi.Uuid { return httpapi.ParseUUID(s) }

func rawUUID(s string) openapi_types.UUID { return httpapi.RawUUID(s) }

func ptr[T any](v T) *T { return httpapi.Ptr(v) }

func deref[T any](v *[]T) []T { return httpapi.Deref(v) }

func derefString(v *string) string { return httpapi.DerefString(v) }
