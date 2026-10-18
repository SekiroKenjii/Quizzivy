package validation_test

import (
	"testing"

	"quizzivy/internal/shared/validation"
)

func TestAFieldRefusedInBothLanguagesKeepsBothAndAPlainOneKeepsOne(t *testing.T) {
	var refused validation.Error
	refused.AddIn("email", "Email không hợp lệ.", "The email is not valid.")
	refused.Add("fullName", "Họ và tên không được để trống.")

	if got := refused.Fields[0]; got.Message != "Email không hợp lệ." || got.English != "The email is not valid." {
		t.Errorf("email = %+v, want both sentences", got)
	}
	if got := refused.Fields[1]; got.English != "" {
		t.Errorf("fullName = %+v, want no English sentence", got)
	}
}
