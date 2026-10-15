package domain_test

import (
	"slices"
	"testing"

	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/tabular"
	"quizzivy/internal/shared/validation"
)

func TestAnAddressASpreadsheetWouldRunIsRefusedWithACodeOfItsOwn(t *testing.T) {
	for _, cell := range []string{
		"+84901234567@x.test", "-mai@x.test", "=mai@x.test", "  +mai@x.test  ", `=cmd|'/C+calc'!A0@x.test`, "+mai+lop10@x.test",
	} {
		plan := domain.PlanImport([]domain.ImportRow{row(2, "A", cell, "", "")}, rules(true), nil)

		if got := issuesOf(plan)[2]; !slices.Equal(got, []string{"email:FORMULA_LIKE"}) || len(plan.Accepted) != 0 {
			t.Errorf("%q: issues = %v, accepted = %d, want email:FORMULA_LIKE and no row", cell, got, len(plan.Accepted))
		}
	}
}

func TestAnAddressThatIsNotAnAddressStaysInvalidWhateverItStartsWith(t *testing.T) {
	for _, cell := range []string{"+84901234567", "=1+1", "-", "@x.test", "@SUM(A1)"} {
		plan := domain.PlanImport([]domain.ImportRow{row(2, "A", cell, "", "")}, rules(true), nil)

		if got := issuesOf(plan)[2]; !slices.Equal(got, []string{"email:INVALID"}) {
			t.Errorf("%q: issues = %v, want email:INVALID", cell, got)
		}
	}
}

func TestAFormulaLikeAddressIsRefusedEvenWhenItBelongsToAnAccountOrIsListedTwice(t *testing.T) {
	rows := []domain.ImportRow{row(2, "A", "+mai@x.test", "", ""), row(3, "B", "+Mai@X.test", "", ""), row(4, "C", "an@x.test", "", "")}

	plan := domain.PlanImport(rows, rules(true), map[string]struct{}{"+mai@x.test": {}})

	got := issuesOf(plan)
	if !slices.Equal(got[2], []string{"email:FORMULA_LIKE"}) || !slices.Equal(got[3], []string{"email:FORMULA_LIKE"}) || len(got[4]) != 0 {
		t.Errorf("issues = %v, want both rows refused as formula-like and the third accepted", got)
	}
	if plan.InvalidRows != 2 || len(plan.Accepted) != 1 || plan.ExistingCount != 0 {
		t.Errorf("plan = %+v, want two invalid rows, one accepted and none existing", plan)
	}
}

func TestAnAcceptedAddressIsOneAnEscapedSheetWritesAsItIs(t *testing.T) {
	for _, lead := range []string{"", "+", "-", "=", "@", "＋", "＝", "－", "＠", "'", "_", ".", "a", "1", " ", "\t", " ", "　"} {
		for _, body := range []string{"x@x.test", "x+tag@x.test", "x=y@x.test", "x-y@x.test"} {
			plan := domain.PlanImport([]domain.ImportRow{row(2, "A", lead+body, "", "")}, rules(true), nil)

			for _, accepted := range plan.Accepted {
				if escaped := tabular.EscapeCell(accepted.Email); escaped != accepted.Email || validation.FormulaLike(accepted.Email) {
					t.Errorf("%q was accepted as %q, which a sheet writes as %q", lead+body, accepted.Email, escaped)
				}
			}
		}
	}
}

func TestATemporaryPasswordIsNeverOneASpreadsheetWouldRun(t *testing.T) {
	for range 500 {
		got, err := domain.Passwords.Temporary()
		if err != nil {
			t.Fatal(err)
		}
		if validation.FormulaLike(got) || tabular.EscapeCell(got) != got {
			t.Fatalf("%q would be written as %q", got, tabular.EscapeCell(got))
		}
	}
}
