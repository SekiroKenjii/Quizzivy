package domain

import (
	"math"
	"slices"
)

// Essay is the one question type marked on a band.
const Essay = "essay"

// GradeShape names why a manual mark does not fit its question's type
// (D-063): an essay is marked on a band, every other type in points.
type GradeShape string

const (
	BandNotAllowed     GradeShape = "band_not_allowed"
	NeedsPoints        GradeShape = "needs_points"
	NeedsBand          GradeShape = "needs_band"
	CriteriaIncomplete GradeShape = "criteria_incomplete"
)

// GradeShapes is the order the refusals are answered in when a grade has
// items under more than one: the contract answers a band on a non-essay
// before missing points.
var GradeShapes = []GradeShape{BandNotAllowed, NeedsPoints, NeedsBand, CriteriaIncomplete}

// GradeShapeError lists the items of a grade that share the first refusal in
// GradeShapes; nothing was saved.
type GradeShapeError struct {
	Shape       GradeShape
	QuestionIDs []string
}

func (e *GradeShapeError) Error() string {
	return "review: grade items do not fit their question: " + string(e.Shape)
}

// ShapeOf says why it does not fit a question of type kind whose rubric
// criteria are rubric, or "" when it fits. An essay without criteria takes a
// band and nothing else; one with criteria takes a band for each of them,
// once, and nothing else; every other type takes points and no band.
func (GradingManager) ShapeOf(kind string, rubric []string, it GradeItem) GradeShape {
	essay := kind == Essay
	switch {
	case !essay && (it.Band != nil || it.Criteria != nil):
		return BandNotAllowed
	case !essay && it.Points == nil:
		return NeedsPoints
	case !essay:
		return ""
	case len(rubric) == 0 && (it.Band == nil || it.Points != nil || it.Criteria != nil):
		return NeedsBand
	case len(rubric) > 0 && (it.Points != nil || it.Band != nil || !coversRubric(rubric, it.Criteria)):
		return CriteriaIncomplete
	}
	return ""
}

func coversRubric(rubric []string, criteria []CriterionBand) bool {
	if len(criteria) != len(rubric) {
		return false
	}
	seen := make(map[string]bool, len(criteria))
	for _, c := range criteria {
		if seen[c.CriterionID] || !slices.Contains(rubric, c.CriterionID) {
			return false
		}
		seen[c.CriterionID] = true
	}
	return true
}

// BandOf is an essay item's band: the one given, or the mean of its
// criteria's bands rounded half up to 0.5, floor(mean × 2 + 0.5) ÷ 2 (DG-61
// interim). It counts in half bands, so 6.25 is 6.5 and 6.125 is 6.0 with no
// float error. Nil for an item marked in points.
func (GradingManager) BandOf(it GradeItem) *float64 {
	if it.Band != nil || len(it.Criteria) == 0 {
		return it.Band
	}
	halves := 0
	for _, c := range it.Criteria {
		halves += int(math.Round(c.Band * 2))
	}
	n := len(it.Criteria)
	band := float64((2*halves+n)/(2*n)) / 2
	return &band
}
