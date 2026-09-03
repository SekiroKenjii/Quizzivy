package api

import (
	"context"
	"errors"
	"time"

	"quizzivy/gen/openapi"
	"quizzivy/internal/assignments"
	"quizzivy/internal/httpx"
)

// ListMyAssignments backs §9's /app: the three sections, already sorted.
func (s *Server) ListMyAssignments(ctx context.Context, _ openapi.ListMyAssignmentsRequestObject) (openapi.ListMyAssignmentsResponseObject, error) {
	if s.Deps.Assignments == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	now := time.Now()
	sections, err := s.Deps.Assignments.ForStudent(ctx, principal.UserID, now)
	if err != nil {
		return nil, err
	}
	return openapi.ListMyAssignments200JSONResponse{
		DueNow:    toAPIStudentCards(sections.DueNow, now),
		Upcoming:  toAPIStudentCards(sections.Upcoming, now),
		Completed: toAPIStudentCards(sections.Completed, now),
	}, nil
}

// GetMyAssignment backs the intro page: the policies stated before the clock
// starts (§10.2). Not targeted, not published and not found are one 403 --
// which assignments exist is not a student's to enumerate.
func (s *Server) GetMyAssignment(ctx context.Context, request openapi.GetMyAssignmentRequestObject) (openapi.GetMyAssignmentResponseObject, error) {
	if s.Deps.Assignments == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	d, err := s.Deps.Assignments.StudentDetail(ctx, request.Id.String(), principal.UserID)
	if errors.Is(err, assignments.ErrForbidden) {
		return openapi.GetMyAssignment403JSONResponse{
			ForbiddenJSONResponse: openapi.ForbiddenJSONResponse(
				authError(ctx, openapi.FORBIDDEN, "Bạn không có quyền xem bài này.")),
		}, nil
	}
	if err != nil {
		return nil, err
	}

	card := toAPIStudentCard(d.StudentCard, time.Now())
	out := openapi.StudentAssignmentDetail{
		Id:              card.Id,
		TestTitle:       card.TestTitle,
		ClassName:       card.ClassName,
		TeacherName:     d.TeacherName,
		Status:          card.Status,
		OpensAt:         card.OpensAt,
		ClosesAt:        card.ClosesAt,
		DurationMinutes: card.DurationMinutes,
		QuestionCount:   card.QuestionCount,
		TotalPoints:     card.TotalPoints,
		AttemptsUsed:    card.AttemptsUsed,
		MaxAttempts:     card.MaxAttempts,
		HasLiveAttempt:  card.HasLiveAttempt,
		LiveDeadlineAt:  card.LiveDeadlineAt,
		LastSubmittedAt: card.LastSubmittedAt,
		Score:           card.Score,
		Review: openapi.ReviewPolicy{
			ShowScore:          d.Review.ShowScore,
			ShowCorrectAnswers: d.Review.ShowCorrectAnswers,
			ShowExplanations:   d.Review.ShowExplanations,
		},
		Integrity: openapi.IntegrityPolicy{
			RequireFullscreen: d.Integrity.RequireFullscreen,
			BlockCopyPaste:    d.Integrity.BlockCopyPaste,
			MaxFocusLoss:      d.Integrity.MaxFocusLoss,
			OnLimitExceeded:   openapi.IntegrityPolicyOnLimitExceeded(d.Integrity.OnLimitExceeded),
			MinAwayMs:         d.Integrity.MinAwayMs,
		},
		HasAudio:        d.HasAudio,
		ShowsTranscript: d.ShowsTranscript,
		AudioMaxPlays:   d.AudioMaxPlays,
	}
	if d.LastAttemptID != nil {
		id := parseUUID(*d.LastAttemptID)
		out.LastAttemptId = &id
	}
	return openapi.GetMyAssignment200JSONResponse(out), nil
}

// ListMyClasses backs §9's /app/classes. Never a join code: the response type
// is shared with the admin list, and the store blanks it before it gets here.
func (s *Server) ListMyClasses(ctx context.Context, _ openapi.ListMyClassesRequestObject) (openapi.ListMyClassesResponseObject, error) {
	if s.Deps.Classes == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	found, err := s.Deps.Classes.ListMine(ctx, principal.UserID)
	if err != nil {
		return nil, err
	}
	items := make([]openapi.Class, 0, len(found))
	for _, c := range found {
		items = append(items, openapi.Class{
			Id:              parseUUID(c.ID),
			Name:            c.Name,
			Description:     c.Description,
			StudentCount:    c.StudentCount,
			SelfJoinEnabled: c.SelfJoinEnabled,
			CreatedAt:       c.CreatedAt,
		})
	}
	return openapi.ListMyClasses200JSONResponse{Items: items}, nil
}

func toAPIStudentCards(cards []assignments.StudentCard, now time.Time) []openapi.StudentAssignmentCard {
	out := make([]openapi.StudentAssignmentCard, 0, len(cards))
	for _, c := range cards {
		out = append(out, toAPIStudentCard(c, now))
	}
	return out
}

func toAPIStudentCard(c assignments.StudentCard, now time.Time) openapi.StudentAssignmentCard {
	live := c.HasLiveAttempt
	out := openapi.StudentAssignmentCard{
		Id:              parseUUID(c.ID),
		TestTitle:       c.TestTitle,
		ClassName:       c.ClassName,
		Status:          openapi.AssignmentStatus(assignments.StatusAt(now, c.PublishedAt, c.OpensAt, c.ClosesAt, c.ClosedAt)),
		OpensAt:         c.OpensAt,
		ClosesAt:        c.ClosesAt,
		DurationMinutes: c.DurationMin,
		QuestionCount:   c.QuestionCount,
		TotalPoints:     c.TotalPoints,
		AttemptsUsed:    c.AttemptsUsed,
		MaxAttempts:     c.MaxAttempts,
		HasLiveAttempt:  &live,
		LiveDeadlineAt:  c.LiveDeadlineAt,
		LastSubmittedAt: c.LastSubmittedAt,
	}
	if c.LastAttemptID != nil {
		id := parseUUID(*c.LastAttemptID)
		out.LastAttemptId = &id
	}
	if c.Score != nil {
		out.Score = &openapi.AttemptScore{
			Earned:        c.Score.Earned,
			Total:         c.Score.Total,
			PendingManual: c.Score.PendingManual,
		}
	}
	return out
}
