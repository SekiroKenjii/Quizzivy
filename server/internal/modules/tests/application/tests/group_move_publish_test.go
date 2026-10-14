//go:build integration

package application_test

import (
	"reflect"
	"testing"
)

const frozenSection = `(SELECT s.id FROM app.test_version_sections s JOIN app.test_versions v ON v.id=s.test_version_id
	WHERE v.test_id=$1 AND v.version=$2 AND s.ordinal=0)`

func (w *moveWorld) versionRows() map[string][]string {
	w.t.Helper()
	sections := `(SELECT s.id FROM app.test_version_sections s JOIN app.test_versions v ON v.id=s.test_version_id WHERE v.test_id=$1)`
	questions := `(SELECT q.id FROM app.test_version_questions q WHERE q.test_version_section_id IN ` + sections + `)`
	groups := `(SELECT g.id FROM app.test_version_groups g WHERE g.test_version_section_id IN ` + sections + `)`
	blanks := `(SELECT b.id FROM app.test_version_blanks b WHERE b.test_version_question_id IN ` + questions + `)`
	tables := []struct{ name, where string }{
		{"test_versions", `t.test_id=$1`},
		{"test_version_sections", `t.test_version_id IN (SELECT id FROM app.test_versions WHERE test_id=$1)`},
		{"test_version_questions", `t.test_version_section_id IN ` + sections},
		{"test_version_options", `t.test_version_question_id IN ` + questions},
		{"test_version_blanks", `t.test_version_question_id IN ` + questions},
		{"test_version_blank_answers", `t.test_version_blank_id IN ` + blanks},
		{"test_version_units", `t.test_version_section_id IN ` + sections},
		{"test_version_groups", `t.test_version_section_id IN ` + sections},
		{"test_version_group_members", `t.group_id IN ` + groups},
		{"test_version_group_stimuli", `t.group_id IN ` + groups},
		{"test_version_group_gap_bindings", `t.group_id IN ` + groups},
		{"test_version_group_recordings", `t.group_id IN ` + groups},
		{"test_version_group_assets", `t.group_id IN ` + groups},
	}
	out := make(map[string][]string, len(tables))
	for _, table := range tables {
		rows, err := w.tx.Query(w.ctx, `SELECT to_jsonb(t)::text FROM app.`+table.name+` t WHERE `+table.where+` ORDER BY 1`, w.testID)
		if err != nil {
			w.t.Fatalf("%s: %v", table.name, err)
		}
		for rows.Next() {
			var row string
			if err := rows.Scan(&row); err != nil {
				w.t.Fatal(err)
			}
			out[table.name] = append(out[table.name], row)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			w.t.Fatal(err)
		}
	}
	return out
}

func (w *moveWorld) frozenUnits(version int) []string {
	w.t.Helper()
	return w.ordered(`SELECT CASE WHEN u.group_id IS NULL THEN 'q:'||q.source_question_id::text ELSE 'g:'||g.title END, u.ordinal
		FROM app.test_version_units u
		LEFT JOIN app.test_version_questions q ON q.id=u.question_id
		LEFT JOIN app.test_version_groups g ON g.id=u.group_id
		WHERE u.test_version_section_id=`+frozenSection+` ORDER BY u.ordinal`, w.testID, version)
}

func (w *moveWorld) frozenQuestions(version int) []string {
	w.t.Helper()
	return w.ordered(`SELECT source_question_id::text, ordinal FROM app.test_version_questions
		WHERE test_version_section_id=`+frozenSection+` ORDER BY ordinal`, w.testID, version)
}

func (w *moveWorld) frozenMembers(version int) []string {
	w.t.Helper()
	return w.ordered(`SELECT q.source_question_id::text, m.ordinal FROM app.test_version_group_members m
		JOIN app.test_version_questions q ON q.id=m.question_id
		WHERE m.test_version_section_id=`+frozenSection+` ORDER BY m.ordinal`, w.testID, version)
}

func TestMoveLeavesPublishedVersionsUntouched(t *testing.T) {
	w := newMoveWorld(t)
	lead, bank := w.question("lead"), w.question("bank")
	w.outline([]string{lead, bank})
	a, b := w.plainGroup(0, "a0", "a1"), w.plainGroup(0, "b0")
	aid, bid := a.Bundle.Group.ID, b.Bundle.Group.ID
	ma := memberIDs(a)
	w.publish()
	before := w.versionRows()
	for _, table := range []string{"test_versions", "test_version_sections", "test_version_questions", "test_version_units", "test_version_groups", "test_version_group_members"} {
		if len(before[table]) == 0 {
			t.Fatalf("the published version has no %s rows to compare", table)
		}
	}

	w.mustMove(bank, 0, ptr(aid), 1, aid)
	w.mustMove(ma[0], 0, nil, 0, aid)
	w.mustMove(ma[1], 0, ptr(bid), 0, aid, bid)
	w.mustMove(ma[1], 0, ptr(bid), 1, bid)

	if after := w.versionRows(); !reflect.DeepEqual(after, before) {
		for table := range before {
			if !reflect.DeepEqual(after[table], before[table]) {
				t.Errorf("%s changed:\nbefore %v\nafter  %v", table, before[table], after[table])
			}
		}
		t.Fatal("a move touched a published version")
	}
}

func TestPublishAfterMoveFreezesTheNewUnitsAndOrdinals(t *testing.T) {
	w := newMoveWorld(t)
	lead, bank := w.question("lead"), w.question("bank")
	w.outline([]string{lead, bank})
	group := w.plainGroup(0, "m0", "m1")
	members := memberIDs(group)
	id := group.Bundle.Group.ID
	first := w.publish()
	requireOrder(t, "first version units", w.frozenUnits(first), "q:"+lead, "q:"+bank, "g:Nhóm m0")
	requireOrder(t, "first version questions", w.frozenQuestions(first), lead, bank, members[0], members[1])

	copied := w.mustMove(bank, 0, ptr(id), 1, id).QuestionID
	w.mustMove(members[0], 0, nil, 1, id)
	second := w.publish()

	requireOrder(t, "second version units", w.frozenUnits(second), "q:"+lead, "q:"+members[0], "g:Nhóm m0")
	requireOrder(t, "second version questions", w.frozenQuestions(second), lead, members[0], copied, members[1])
	requireOrder(t, "second version members", w.frozenMembers(second), copied, members[1])
	requireOrder(t, "first version units, after the second publish", w.frozenUnits(first), "q:"+lead, "q:"+bank, "g:Nhóm m0")
	requireOrder(t, "first version questions, after the second publish", w.frozenQuestions(first), lead, bank, members[0], members[1])
}
