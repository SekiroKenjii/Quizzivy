//go:build integration

package application_test

import (
	attemptsdomain "quizzivy/internal/modules/attempts/domain"
	attemptsrepo "quizzivy/internal/modules/attempts/repositories"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/platform/db"
	"slices"
	"strings"
	"testing"
)

func TestMovedQuestionsDealInsideAndOutsideTheirGroups(t *testing.T) {
	w := newMoveWorld(t)
	lead, bank, stray := w.question("lead"), w.question("bank"), w.question("stray")
	w.outline([]string{lead, bank, stray})
	group := w.plainGroup(0, "m0", "m1")
	members := memberIDs(group)
	id := group.Bundle.Group.ID
	copied := w.mustMove(bank, 0, ptr(id), 0, id).QuestionID
	w.mustMove(members[0], 0, nil, 1, id)
	number := w.publish()

	var versionID string
	if err := w.tx.QueryRow(w.ctx, `SELECT id::text FROM app.test_versions WHERE test_id=$1 AND version=$2`, w.testID, number).Scan(&versionID); err != nil {
		t.Fatal(err)
	}
	frozen := map[string]string{}
	rows, err := w.tx.Query(w.ctx, `SELECT q.source_question_id::text, q.id::text FROM app.test_version_questions q
		JOIN app.test_version_sections s ON s.id=q.test_version_section_id WHERE s.test_version_id=$1`, versionID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var source, version string
		if err := rows.Scan(&source, &version); err != nil {
			t.Fatal(err)
		}
		frozen[source] = version
	}
	rows.Close()
	if _, held := frozen[bank]; held || len(frozen) != 5 {
		t.Fatalf("the version froze %d questions and holds the bank question: %v, want 5 without it", len(frozen), held)
	}

	papers := attemptsrepo.NewPostgres(db.NewContext(w.tx), nil)
	sections, err := papers.Sections(w.ctx, versionID)
	if err != nil {
		t.Fatal(err)
	}
	questions, err := papers.Questions(w.ctx, versionID)
	if err != nil {
		t.Fatal(err)
	}
	delivery, err := papers.DeliveryVersion(w.ctx, versionID)
	if err != nil {
		t.Fatal(err)
	}
	if delivery != domain.DeliveryGroupV1 {
		t.Fatalf("the version is delivered as %q", delivery)
	}

	unitOrders := map[string]bool{}
	for seed := range int64(30) {
		paper, err := attemptsdomain.Deal.PresentVersion(delivery, seed, true, false, sections, questions)
		if err != nil {
			t.Fatal(err)
		}
		if len(paper) != 5 {
			t.Fatalf("seed %d dealt %d questions, want 5", seed, len(paper))
		}
		positions := map[string]int{}
		for i, q := range paper {
			positions[q.ID] = i
		}
		inCopy, inM1, outM0 := paper[positions[frozen[copied]]], paper[positions[frozen[members[1]]]], paper[positions[frozen[members[0]]]]
		if inCopy.GroupID == "" || inCopy.GroupID != inM1.GroupID || inCopy.GroupOrdinal != 0 || inM1.GroupOrdinal != 1 {
			t.Fatalf("seed %d: the copy is not in its group's run: copy %+v, member %+v", seed, inCopy, inM1)
		}
		if positions[inM1.ID] != positions[inCopy.ID]+1 {
			t.Fatalf("seed %d: the group's members are not one run in member order: %v", seed, positions)
		}
		if outM0.GroupID != "" {
			t.Fatalf("seed %d: the question that left the group still deals as a member of %s", seed, outM0.GroupID)
		}
		var order []string
		for _, q := range paper {
			if q.GroupID != "" {
				if len(order) == 0 || order[len(order)-1] != "group" {
					order = append(order, "group")
				}
				continue
			}
			order = append(order, q.ID)
		}
		if len(order) != 4 || slices.Contains(order, frozen[copied]) {
			t.Fatalf("seed %d: the units are %v", seed, order)
		}
		unitOrders[strings.Join(order, ",")] = true
	}
	if len(unitOrders) < 2 {
		t.Fatal("the shuffle never moved a unit, so the test proves nothing about the run")
	}
}
