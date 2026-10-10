//go:build integration

package repositories_test

import (
	"context"
	"testing"
	"time"

	"quizzivy/internal/modules/assignments/domain"
)

func TestAnAttemptPastItsDeadlineIsNotRevivedByAnyWriterOfTheWindow(t *testing.T) {
	ctx := context.Background()
	writers := []struct {
		name string
		run  func(t *testing.T, w *reachWorld, req domain.Request, closes time.Time, students []string) error
	}{
		{"Extend", func(_ *testing.T, w *reachWorld, req domain.Request, _ time.Time, _ []string) error {
			_, err := w.store.Extend(ctx, req, 120, false, time.Now())
			return err
		}},
		{"Update", func(t *testing.T, w *reachWorld, req domain.Request, closes time.Time, _ []string) error {
			_, err := w.store.Update(ctx, req, w.updateWindow(t, closes.Add(2*time.Hour)))
			return err
		}},
		{"SetOverrides", func(_ *testing.T, w *reachWorld, req domain.Request, _ time.Time, students []string) error {
			_, err := w.store.SetOverrides(ctx, req, domain.OverrideInput{
				StudentIDs: students, ClosesAt: after(2 * time.Hour), DurationMin: ptr(120), Reason: "thêm giờ", Now: time.Now(),
			})
			return err
		}},
		{"Reopen", func(_ *testing.T, w *reachWorld, req domain.Request, closes time.Time, _ []string) error {
			if _, err := w.tx.Exec(ctx, `UPDATE app.assignments SET closed_at = now() - interval '1 minute' WHERE id = $1::uuid`, req.ID); err != nil {
				return err
			}
			_, err := w.store.Reopen(ctx, req, closes.Add(3*time.Hour), "mất điện", time.Now())
			return err
		}},
	}

	for _, writer := range writers {
		t.Run(writer.name, func(t *testing.T) {
			w := newReachWorld(t)
			assignment := w.closingSoon(t, w.a, w.classA, 20*time.Minute, 45)
			overdueStudent, liveStudent := w.classmate(t, w.classA, w.a), w.classmate(t, w.classA, w.a)
			closes := w.closesAt(t, assignment)
			overdue := w.sitting(t, assignment, overdueStudent, 30*time.Minute, time.Now().Add(-time.Minute), "in_progress")
			live := w.sitting(t, assignment, liveStudent, 5*time.Minute, closes, "in_progress")
			overdueBefore, liveBefore := w.deadlineOf(t, overdue), w.deadlineOf(t, live)

			req := as(w.a, false)
			req.ID = assignment
			if err := writer.run(t, w, req, closes, []string{overdueStudent, liveStudent}); err != nil {
				t.Fatalf("%s: %v", writer.name, err)
			}

			if got := w.deadlineOf(t, overdue); !got.Equal(overdueBefore) {
				t.Errorf("an attempt a minute past its deadline was moved to %v from %v", got, overdueBefore)
			}
			if n, _, _ := w.auditedMoves(t, overdue); n != 0 {
				t.Errorf("%d audit entries for an attempt that was already over", n)
			}
			if got := w.deadlineOf(t, live); !got.After(liveBefore) {
				t.Errorf("the attempt still ahead of its deadline was not lengthened: %v from %v", got, liveBefore)
			}
		})
	}
}
