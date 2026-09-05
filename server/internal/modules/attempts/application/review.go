package application

import (
	"context"

	"quizzivy/internal/modules/attempts/domain"
)

// Review is the teacher's use cases over one paper.
type Review struct {
	repo domain.ReviewRepository
}

func NewReview(repo domain.ReviewRepository) *Review {
	return &Review{repo: repo}
}

func (r *Review) Get(ctx context.Context, attemptID string) (domain.Review, error) {
	return r.repo.Get(ctx, attemptID)
}

func (r *Review) Grade(ctx context.Context, attemptID, graderID string, items []domain.GradeItem) (domain.Score, error) {
	return r.repo.Grade(ctx, attemptID, graderID, items)
}

func (r *Review) Finish(ctx context.Context, attemptID string) (domain.Attempt, error) {
	return r.repo.Finish(ctx, attemptID)
}

func (r *Review) SetNote(ctx context.Context, attemptID string, note *string) error {
	return r.repo.SetNote(ctx, attemptID, note)
}

func (r *Review) AnswersForQuestion(ctx context.Context, assignmentID, questionID string) (domain.ByQuestion, error) {
	return r.repo.AnswersForQuestion(ctx, assignmentID, questionID)
}

// Integrity is the teacher's reading of what happened while a paper was open.
type Integrity struct {
	repo domain.TimelineRepository
}

func NewIntegrity(repo domain.TimelineRepository) *Integrity {
	return &Integrity{repo: repo}
}

func (i *Integrity) Timeline(ctx context.Context, attemptID string) (domain.Timeline, error) {
	return i.repo.Timeline(ctx, attemptID)
}
