package command

import (
	"fmt"

	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/validation"
)

func cleanName(refused *validation.Error, raw string) string {
	name, fault, hidden := domain.CleanFullName(raw)
	switch fault {
	case domain.FullNameOK:
	case domain.FullNameEmpty:
		refused.AddIn("fullName", "Họ và tên không được để trống.", "The full name cannot be empty.")
	case domain.FullNameTooLong:
		refused.AddIn("fullName",
			fmt.Sprintf("Nội dung không được dài quá %d ký tự.", domain.MaxFullNameLength),
			fmt.Sprintf("The full name cannot be longer than %d characters.", domain.MaxFullNameLength))
	case domain.FullNameHidden:
		refused.AddIn("fullName",
			fmt.Sprintf("Họ và tên không được chứa ký tự điều khiển hoặc ký tự vô hình (U+%04X).", hidden),
			fmt.Sprintf("The full name cannot contain a control character or an invisible character (U+%04X).", hidden))
	default:
		refused.AddIn("fullName",
			"Họ và tên phải có ít nhất một chữ cái, chữ số, dấu câu hoặc ký hiệu.",
			"The full name must have at least one letter, digit, punctuation mark or symbol.")
	}
	return name
}

func checkAddress(refused *validation.Error, address string) {
	if hidden, found := domain.HiddenInEmail(address); found {
		refused.AddIn("email",
			fmt.Sprintf("Email không được chứa khoảng trắng, ký tự điều khiển hoặc ký tự vô hình (U+%04X).", hidden),
			fmt.Sprintf("The email cannot contain a space, a control character or an invisible character (U+%04X).", hidden))
	}
}
