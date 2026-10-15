package validation_test

import (
	"testing"

	"quizzivy/internal/shared/validation"
)

func TestFormulaLikeNamesWhatASpreadsheetRunsAsAFormula(t *testing.T) {
	for _, value := range []string{
		"=1", "+84901234567@example.com", "-x@example.com", "@SUM(A1)", "\t=1", "\r=1",
		"  =1+1", " =1", "　@SUM(A1)", "＝1", "＋1", "－2", "＠x", " \t=1", "=", "-", "+", "@",
		`=cmd|'/C+calc'!A0@x.com`,
	} {
		if !validation.FormulaLike(value) {
			t.Errorf("FormulaLike(%q) = false, want true", value)
		}
	}
}

func TestFormulaLikeLeavesOrdinaryTextAlone(t *testing.T) {
	for _, value := range []string{
		"", " ", "a@example.com", "x+y@example.com", "tên-họ@example.com", "a=b", "'=1", "・=1", "\n=1", "1+1", "_x@example.com",
	} {
		if validation.FormulaLike(value) {
			t.Errorf("FormulaLike(%q) = true, want false", value)
		}
	}
}
