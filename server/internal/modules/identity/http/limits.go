package http

import (
	"context"
	"errors"
	"quizzivy/gen/openapi"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/limits"
)

func overLimit(ctx context.Context, err error) (openapi.ErrorResponse, bool) {
	switch {
	case errors.Is(err, limits.ErrSeatLimit):
		return httpapi.Error(ctx, openapi.SEATLIMIT,
			httpx.Text(ctx, "Trung tâm đã đạt giới hạn số tài khoản. Quản trị viên có thể nâng giới hạn trong Cài đặt → Gói và mức dùng.",
				"The centre has reached its account limit. An administrator can raise it in Settings → Plan & usage.")), true
	case errors.Is(err, limits.ErrTeacherLimit):
		return httpapi.Error(ctx, openapi.TEACHERLIMIT,
			httpx.Text(ctx, "Trung tâm đã đạt giới hạn số giáo viên. Quản trị viên có thể nâng giới hạn trong Cài đặt → Gói và mức dùng.",
				"The centre has reached its teacher limit. An administrator can raise it in Settings → Plan & usage.")), true
	}
	return openapi.ErrorResponse{}, false
}
