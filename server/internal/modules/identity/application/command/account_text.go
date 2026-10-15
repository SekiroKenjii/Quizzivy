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
		refused.Add("fullName", "Họ và tên không được để trống.")
	case domain.FullNameTooLong:
		refused.Add("fullName", fmt.Sprintf("Nội dung không được dài quá %d ký tự.", domain.MaxFullNameLength))
	case domain.FullNameHidden:
		refused.Add("fullName", fmt.Sprintf("Họ và tên không được chứa ký tự điều khiển hoặc ký tự vô hình (U+%04X).", hidden))
	default:
		refused.Add("fullName", "Họ và tên phải có ít nhất một chữ cái, chữ số, dấu câu hoặc ký hiệu.")
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
