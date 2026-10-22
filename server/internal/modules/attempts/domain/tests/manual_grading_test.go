package domain_test

import (
	"quizzivy/internal/modules/attempts/domain"
	"testing"
)

func band(b float64) *float64 { return &b }

func criteria(bands ...float64) []domain.CriterionBand {
	out := make([]domain.CriterionBand, len(bands))
	for i, b := range bands {
		out[i] = domain.CriterionBand{CriterionID: string(rune('a' + i)), Band: b}
	}
	return out
}

func TestTheBandOfARubricIsItsMeanRoundedHalfUpToAHalf(t *testing.T) {
	for _, tc := range []struct {
		name  string
		bands []float64
		want  float64
	}{
		{"6.25 rounds up", []float64{6, 6.5, 6.5, 6}, 6.5},
		{"6.75 rounds up", []float64{6.5, 7}, 7},
		{"6.125 rounds down", []float64{6, 6, 6, 6.5}, 6},
		{"6.375 rounds up", []float64{6, 6.5, 6.5, 6.5}, 6.5},
		{"a third rounds down", []float64{6, 6, 6.5}, 6},
		{"two thirds round up", []float64{6, 6.5, 6.5}, 6.5},
		{"one criterion", []float64{3}, 3},
		{"six nines", []float64{9, 9, 9, 9, 9, 9}, 9},
	} {
		got := domain.Grading.BandOf(domain.GradeItem{Criteria: criteria(tc.bands...)})
		if got == nil || *got != tc.want {
			t.Errorf("%s: band %v, want %v", tc.name, got, tc.want)
		}
	}
	if got := domain.Grading.BandOf(domain.GradeItem{Band: band(7.5)}); *got != 7.5 {
		t.Errorf("a band given directly became %v", *got)
	}
	if got := domain.Grading.BandOf(domain.GradeItem{Points: band(2)}); got != nil {
		t.Errorf("a mark in points has band %v", *got)
	}
}

func TestEachTypeTakesTheMarkItIsGradedOn(t *testing.T) {
	rubric := []string{"a", "b"}
	for _, tc := range []struct {
		name   string
		kind   string
		rubric []string
		item   domain.GradeItem
		want   domain.GradeShape
	}{
		{"points on a choice", "single_choice", nil, domain.GradeItem{Points: band(1)}, ""},
		{"nothing on a short answer", "short_answer", nil, domain.GradeItem{}, domain.NeedsPoints},
		{"a band on a short answer", "short_answer", nil, domain.GradeItem{Points: band(1), Band: band(6)}, domain.BandNotAllowed},
		{"criteria on a matching", "matching", nil, domain.GradeItem{Criteria: criteria(6)}, domain.BandNotAllowed},
		{"a band on an essay", "essay", nil, domain.GradeItem{Band: band(6)}, ""},
		{"points on an essay", "essay", nil, domain.GradeItem{Points: band(6)}, domain.NeedsBand},
		{"points and a band on an essay", "essay", nil, domain.GradeItem{Points: band(6), Band: band(6)}, domain.NeedsBand},
		{"nothing on an essay", "essay", nil, domain.GradeItem{}, domain.NeedsBand},
		{"criteria on an essay without a rubric", "essay", nil, domain.GradeItem{Band: band(6), Criteria: criteria(6)}, domain.NeedsBand},
		{"every criterion", "essay", rubric, domain.GradeItem{Criteria: criteria(6, 7)}, ""},
		{"a criterion missing", "essay", rubric, domain.GradeItem{Criteria: criteria(6)}, domain.CriteriaIncomplete},
		{"a criterion it does not have", "essay", rubric, domain.GradeItem{Criteria: criteria(6, 7, 8)}, domain.CriteriaIncomplete},
		{"a criterion twice", "essay", rubric, domain.GradeItem{Criteria: []domain.CriterionBand{{CriterionID: "a", Band: 6}, {CriterionID: "a", Band: 7}}}, domain.CriteriaIncomplete},
		{"a band beside a rubric", "essay", rubric, domain.GradeItem{Band: band(6)}, domain.CriteriaIncomplete},
		{"criteria and a band", "essay", rubric, domain.GradeItem{Band: band(6), Criteria: criteria(6, 7)}, domain.CriteriaIncomplete},
		{"criteria and points", "essay", rubric, domain.GradeItem{Points: band(6), Criteria: criteria(6, 7)}, domain.CriteriaIncomplete},
	} {
		if got := domain.Grading.ShapeOf(tc.kind, tc.rubric, tc.item); got != tc.want {
			t.Errorf("%s: shape %q, want %q", tc.name, got, tc.want)
		}
	}
}
