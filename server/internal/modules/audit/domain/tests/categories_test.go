package domain_test

import (
	"slices"
	"testing"

	"quizzivy/internal/modules/audit/domain"
)

func TestTheSixCategoriesAreValidAndNothingElseIs(t *testing.T) {
	if len(domain.Categories()) != 6 {
		t.Fatalf("%d categories, want 6", len(domain.Categories()))
	}
	for _, c := range domain.Categories() {
		if !c.Valid() {
			t.Errorf("%s is not valid", c)
		}
	}
	for _, c := range []domain.Category{"", "all", "Accounts", "organization"} {
		if c.Valid() {
			t.Errorf("%q is valid", c)
		}
	}
}

func TestEveryCategoryHoldsEntitiesAndEachEntityIsInOne(t *testing.T) {
	owner := map[string]domain.Category{}
	for _, c := range domain.Categories() {
		entities := domain.EntitiesOf(c)
		if len(entities) == 0 {
			t.Errorf("%s holds no entity", c)
		}
		for _, e := range entities {
			if other, dup := owner[e]; dup {
				t.Errorf("%s is in %s and in %s", e, other, c)
			}
			owner[e] = c
			if got, ok := domain.CategoryOf(e); !ok || got != c {
				t.Errorf("CategoryOf(%s) = %s, %v, want %s", e, got, ok, c)
			}
		}
	}
}

func TestTheEntitiesTheBriefNamesAreInTheirFamilies(t *testing.T) {
	cases := map[string]domain.Category{
		"user": domain.Accounts, "users": domain.Accounts, "student": domain.Accounts,
		"user_identity": domain.Accounts, "refresh_token_family": domain.Accounts,
		"class": domain.Classes, "class_member": domain.Classes, "class_join_code": domain.Classes,
		"attempt": domain.Attempts, "assignment": domain.Attempts,
		"test": domain.Tests, "test_version": domain.Tests, "question": domain.Tests, "question_group": domain.Tests,
		"media_asset": domain.Media, "word_import": domain.Imports, "user_import": domain.Imports,
	}
	for entity, want := range cases {
		if got, ok := domain.CategoryOf(entity); !ok || got != want {
			t.Errorf("%s is in %s (%v), want %s", entity, got, ok, want)
		}
	}
	for _, entity := range []string{"maintenance_window", "attempt_events", "ai_settings", "ai_teacher_credits", "ai_credit_periods", "ai_credit_requests", "audit_log", "organization"} {
		if _, ok := domain.CategoryOf(entity); ok {
			t.Errorf("%s is in a category: organization, export, maintenance, retention and AI entries are under All only", entity)
		}
	}
}

func TestFacetsCountEveryEntryInAllAndOnlyTheCategorisedOnesBeside(t *testing.T) {
	got := domain.FoldFacets(map[string]int{
		"user": 2, "student": 1, "class": 4, "class_member": 1, "attempt": 3, "test": 5,
		"media_asset": 2, "word_import": 7,
		"maintenance_window": 11, "audit_log": 1, "mystery": 13,
	})
	want := domain.Facets{All: 50, Accounts: 3, Classes: 5, Attempts: 3, Tests: 5, Media: 2, Imports: 7}
	if got != want {
		t.Errorf("facets %+v, want %+v", got, want)
	}
	if sum := got.Accounts + got.Classes + got.Attempts + got.Tests + got.Media + got.Imports; sum >= got.All {
		t.Errorf("the six add up to %d, which should be below all (%d): some entries are in no category", sum, got.All)
	}
}

func TestFacetsOfReadsTheCountOfACategoryAndAllForNone(t *testing.T) {
	f := domain.Facets{All: 9, Accounts: 1, Classes: 2, Attempts: 3, Tests: 4, Media: 5, Imports: 6}
	counts := []int{}
	for _, c := range domain.Categories() {
		counts = append(counts, f.Of(c))
	}
	if !slices.Equal(counts, []int{1, 2, 3, 4, 5, 6}) || f.Of("") != 9 {
		t.Errorf("Of gave %v and %d for none", counts, f.Of(""))
	}
}

func TestEntitiesListsEveryEntityOnce(t *testing.T) {
	all := domain.Entities()
	if got := len(slices.Compact(slices.Sorted(slices.Values(all)))); got != len(all) {
		t.Errorf("Entities repeats one: %d distinct of %d", got, len(all))
	}
}
