//go:build integration

package application_test

import (
	"context"
	"errors"
	"testing"

	"quizzivy/internal/modules/classes/application/command"
	"quizzivy/internal/modules/classes/domain"
)

func TestARefusedClassDeletionNamesWhatHoldsIt(t *testing.T) {
	w := newClassWorld(t)
	w.sat(t, w.a, w.classA, w.studentA)
	w.exec(t, `UPDATE app.classes SET archived_at = now() WHERE id = $1`, w.classA)
	_, err := w.svc.Commands.Delete.Handle(context.Background(), command.Delete{ClassID: w.classA, Actor: w.who(w.a)})
	var refused *domain.ReferencedError
	if !errors.Is(err, domain.ErrReferenced) || !errors.As(err, &refused) || refused.By != domain.ReferencedByAssignments {
		t.Fatalf("deleting a class an assignment targets answered %v, want referenced by assignments", err)
	}
	var kept bool
	if err := w.tx.QueryRow(context.Background(), `SELECT EXISTS (SELECT 1 FROM app.classes WHERE id = $1)`, w.classA).Scan(&kept); err != nil || !kept {
		t.Fatalf("the class was deleted (%v)", err)
	}
}
