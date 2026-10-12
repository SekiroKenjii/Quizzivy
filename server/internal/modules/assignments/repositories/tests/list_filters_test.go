//go:build integration

package repositories_test

import (
	"context"
	"slices"
	"testing"

	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/shared/access"
)

type filterWorld struct {
	*reachWorld
	pronunciation, grammar string
}

func newFilterWorld(t *testing.T) filterWorld {
	t.Helper()
	w := newReachWorld(t)
	ctx := context.Background()
	if _, err := w.tx.Exec(ctx, `UPDATE app.tests SET title = 'Phát âm Unit 3' WHERE id = (SELECT test_id FROM app.test_versions WHERE id = $1::uuid)`, w.versionA); err != nil {
		t.Fatal(err)
	}
	grammarVersion := w.version(t, w.a)
	if _, err := w.tx.Exec(ctx, `UPDATE app.tests SET title = '100% ngữ_pháp' WHERE id = (SELECT test_id FROM app.test_versions WHERE id = $1::uuid)`, grammarVersion); err != nil {
		t.Fatal(err)
	}
	grammar := w.create(t, as(w.a, false), input(grammarVersion, []string{w.classA}, nil))
	return filterWorld{reachWorld: w, pronunciation: w.mineA, grammar: grammar}
}

func (w filterWorld) idsOf(t *testing.T, in domain.ListInput) []string {
	t.Helper()
	in.Limit = 100
	found, _, err := w.store.List(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	mine := []string{w.mineA, w.mineB, w.shared, w.grammar}
	ids := []string{}
	for _, a := range found {
		if slices.Contains(mine, a.ID) {
			ids = append(ids, a.ID)
		}
	}
	slices.Sort(ids)
	return ids
}

func TestTheListFindsAnAssignmentByTheTitleOfItsTestOrTheNameOfAClassTheReaderReaches(t *testing.T) {
	w := newFilterWorld(t)
	teacher := access.Scope{UserID: w.a}
	own := access.Scope{UserID: w.admin}
	for name, c := range map[string]struct {
		scope access.Scope
		every bool
		query string
		want  []string
	}{
		"the title without its accents":            {teacher, false, "phat am", sortedIDs(w.mineA, w.shared)},
		"the title in capitals with its accents":   {teacher, false, "PHÁT ÂM UNIT", sortedIDs(w.mineA, w.shared)},
		"another title without its accents":        {teacher, false, "ngu_phap", sortedIDs(w.grammar)},
		"a title that holds a percent and a mark":  {teacher, false, "100% ngữ_pháp", sortedIDs(w.grammar)},
		"a percent is a character, not a wildcard": {teacher, false, "%", sortedIDs(w.grammar)},
		"an underscore is not a wildcard":          {teacher, false, "phat_am", []string{}},
		"the name of the class the reader teaches": {teacher, false, "lớp a", sortedIDs(w.mineA, w.shared, w.grammar)},
		"the name of a class the reader does not":  {teacher, false, "lop b", []string{}},
		"that class for the Admin's own list":      {own, true, "lop b", sortedIDs(w.shared)},
		"text nothing contains":                    {teacher, false, "không có gì", []string{}},
		"only spaces filter nothing":               {teacher, false, "   ", sortedIDs(w.mineA, w.shared, w.grammar)},
	} {
		in := domain.ListInput{Scope: c.scope, EveryTarget: c.every, Query: c.query}
		if got := w.idsOf(t, in); !slices.Equal(got, c.want) {
			t.Errorf("%s: q=%q lists %v, want %v", name, c.query, got, c.want)
		}
	}
}

func TestTheFacetsFollowTheSearchAndTheClasses(t *testing.T) {
	w := newFilterWorld(t)
	ctx := context.Background()
	teacher := access.Scope{UserID: w.a}

	everything, err := w.store.Facets(ctx, domain.ListInput{Scope: teacher})
	if err != nil {
		t.Fatal(err)
	}
	searched, err := w.store.Facets(ctx, domain.ListInput{Scope: teacher, Query: "phat am"})
	if err != nil {
		t.Fatal(err)
	}
	if searched.All != 2 || searched.Open != 2 || everything.All < 3 {
		t.Errorf("the tabs count %+v for a search that lists two assignments (all of them: %+v)", searched, everything)
	}
	byClass, err := w.store.Facets(ctx, domain.ListInput{Scope: teacher, ClassIDs: []string{w.classA}, Query: "phat am"})
	if err != nil || byClass.All != 2 {
		t.Errorf("the tabs count %+v (%v) for a search within a class, want 2", byClass, err)
	}
}

func TestTheListTakesSeveralClassesAtOnce(t *testing.T) {
	w := newFilterWorld(t)
	for name, c := range map[string]struct {
		scope access.Scope
		every bool
		ids   []string
		want  []string
	}{
		"a teacher naming their class and another's":     {access.Scope{UserID: w.a}, false, []string{w.classA, w.classB}, sortedIDs(w.mineA, w.shared, w.grammar)},
		"the other teacher naming both":                  {access.Scope{UserID: w.b}, false, []string{w.classA, w.classB}, sortedIDs(w.mineB, w.shared)},
		"the Admin's own list naming both":               {access.Scope{UserID: w.admin}, true, []string{w.classA, w.classB}, sortedIDs(w.shared)},
		"the Admin's own list naming one and a missing":  {access.Scope{UserID: w.admin}, true, []string{w.classB, "00000000-0000-7000-8000-000000000000"}, sortedIDs(w.shared)},
		"a teacher naming only a class they do not have": {access.Scope{UserID: w.a}, false, []string{w.classB}, []string{}},
	} {
		in := domain.ListInput{Scope: c.scope, EveryTarget: c.every, ClassIDs: c.ids}
		if got := w.idsOf(t, in); !slices.Equal(got, c.want) {
			t.Errorf("%s lists %v, want %v", name, got, c.want)
		}
	}
}

func TestAnAssignmentCountsTheQuestionsOfItsVersion(t *testing.T) {
	w := newReachWorld(t)
	ctx := context.Background()
	var first, second string
	for i, section := range []*string{&first, &second} {
		if err := w.tx.QueryRow(ctx, `INSERT INTO app.test_version_sections (test_version_id, ordinal, title) VALUES ($1::uuid, $2, 'Phần') RETURNING id::text`,
			w.versionA, i).Scan(section); err != nil {
			t.Fatal(err)
		}
	}
	for _, q := range []struct {
		section string
		ordinal int
	}{{first, 0}, {first, 1}, {second, 0}} {
		if _, err := w.tx.Exec(ctx, `INSERT INTO app.test_version_questions (test_version_section_id, ordinal, type, prompt, points)
			VALUES ($1::uuid, $2, 'short_answer', 'Câu hỏi', 1)`, q.section, q.ordinal); err != nil {
			t.Fatal(err)
		}
	}

	for name, c := range map[string]struct {
		id   string
		want int
	}{"a version with three questions": {w.mineA, 3}, "the same version assigned again": {w.shared, 3}, "a version with none": {w.mineB, 0}} {
		got, err := w.store.Get(ctx, access.Scope{All: true}, c.id)
		if err != nil {
			t.Fatal(err)
		}
		if got.QuestionCount != c.want {
			t.Errorf("%s: questionCount = %d, want %d", name, got.QuestionCount, c.want)
		}
	}
	listed, _, err := w.store.List(ctx, domain.ListInput{Scope: access.Scope{UserID: w.a}, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range listed {
		if a.ID == w.mineA && a.QuestionCount != 3 {
			t.Errorf("the list says %d questions for a version with three", a.QuestionCount)
		}
	}
}
