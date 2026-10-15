package http

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"quizzivy/gen/openapi"
	accessdomain "quizzivy/internal/modules/access/domain"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/platform/tabular"
)

func tooLargeRefusal(ctx context.Context) importRefusal {
	return importRefusal{http.StatusRequestEntityTooLarge, openapi.USERIMPORTFILETOOLARGE, httpx.Text(ctx,
		"Tệp lớn hơn 5 MB. Hãy lưu bảng tính thành CSV UTF-8 rồi tải tệp đó lên.",
		"The file is larger than 5 MB. Save the sheet as CSV UTF-8 and upload that.")}
}

// refuseImport words why an import was refused: the platform's file errors,
// the header, the rules of the request, and what the server cannot do. It
// answers false for any other error, which is a fault. No message carries
// text of the file, since an error of the reader can hold up to 64 KiB of it.
func refuseImport(ctx context.Context, err error) (importRefusal, bool) {
	var header *domain.ImportHeaderError
	switch {
	case errors.As(err, &header):
		return importRefusal{http.StatusUnprocessableEntity, openapi.USERIMPORTCOLUMNS, headerMessage(ctx, header.Problem)}, true
	case errors.Is(err, accessdomain.ErrTargetRoleNotAllowed):
		return importRefusal{http.StatusForbidden, openapi.TARGETROLENOTALLOWED, httpx.Text(ctx,
			"Bạn chỉ giao được những quyền bạn có.", "You can only give what you can do yourself.")}, true
	case errors.Is(err, domain.ErrImportNothingToCreate):
		return importRefusal{http.StatusUnprocessableEntity, openapi.USERIMPORTNOTHINGTOCREATE, httpx.Text(ctx,
			"Không có dòng nào tạo được tài khoản mới: các dòng hợp lệ đều dùng email đã có tài khoản, hoặc tệp không có dòng hợp lệ.",
			"No row would create an account: every valid row uses an email that already has one, or the file has no valid row.")}, true
	case errors.Is(err, domain.ErrPasswordSignInDisabled):
		return importRefusal{http.StatusUnprocessableEntity, openapi.PASSWORDSIGNINDISABLED, httpx.Text(ctx,
			"Trung tâm đã tắt đăng nhập bằng mật khẩu nên không phát được mật khẩu tạm. Hãy chọn gửi thông tin đăng nhập qua email.",
			"Your centre has turned off password sign-in, so temporary passwords cannot be handed out. Choose to send the sign-in details by email.")}, true
	}
	return refuseFile(ctx, err)
}

func refuseFile(ctx context.Context, err error) (importRefusal, bool) {
	for _, refuse := range []func(context.Context, error) (importRefusal, bool){refuseEncoding, refuseSize, refuseShape} {
		if refusal, ok := refuse(ctx, err); ok {
			return refusal, true
		}
	}
	return importRefusal{}, false
}

func unreadable(message string) importRefusal {
	return importRefusal{http.StatusUnprocessableEntity, openapi.USERIMPORTFILEUNREADABLE, message}
}

func refuseEncoding(ctx context.Context, err error) (importRefusal, bool) {
	switch {
	case errors.Is(err, tabular.ErrNotUTF8):
		return importRefusal{http.StatusUnprocessableEntity, openapi.CSVNOTUTF8, httpx.Text(ctx,
			`Tệp CSV không dùng mã hoá UTF-8. Hãy lưu lại thành "CSV UTF-8" rồi tải lên.`,
			`The CSV file is not UTF-8. Save it as "CSV UTF-8" and upload it again.`)}, true
	case errors.Is(err, tabular.ErrMacros):
		return importRefusal{http.StatusUnprocessableEntity, openapi.XLSXMACROS, httpx.Text(ctx,
			"Bảng tính có macro nên không nhập được. Hãy lưu thành .xlsx không macro hoặc CSV UTF-8 rồi tải lên.",
			"The workbook contains macros, so it cannot be imported. Save it as an .xlsx without macros or as CSV UTF-8 and upload that.")}, true
	case errors.Is(err, tabular.ErrEncrypted):
		return importRefusal{http.StatusUnprocessableEntity, openapi.XLSXENCRYPTED, httpx.Text(ctx,
			"Bảng tính được bảo vệ bằng mật khẩu. Hãy gỡ mật khẩu hoặc lưu thành CSV UTF-8 rồi tải lên.",
			"The workbook is protected by a password. Remove the password, or save the sheet as CSV UTF-8 and upload that.")}, true
	}
	return importRefusal{}, false
}

func refuseSize(ctx context.Context, err error) (importRefusal, bool) {
	switch {
	case errors.Is(err, tabular.ErrTooLarge):
		return tooLargeRefusal(ctx), true
	case errors.Is(err, tabular.ErrTooManyEntries):
		return unreadable(httpx.Text(ctx,
			"Bảng tính có hơn 1.000 phần bên trong, nhiều hơn hẳn một bảng tính thông thường. Hãy lưu bảng tính thành CSV UTF-8 rồi tải tệp đó lên.",
			"The workbook holds more than 1,000 parts, far more than an ordinary one. Save the sheet as CSV UTF-8 and upload that.")), true
	case errors.Is(err, tabular.ErrExpandedTooLarge):
		return unreadable(httpx.Text(ctx,
			"Bảng tính lớn hơn 32 MB khi giải nén. Hãy lưu bảng tính thành CSV UTF-8 rồi tải tệp đó lên.",
			"The workbook is larger than 32 MB once unpacked. Save the sheet as CSV UTF-8 and upload that.")), true
	case errors.Is(err, tabular.ErrSheetTooLarge):
		return unreadable(httpx.Text(ctx,
			"Một trang tính lớn hơn 8 MB khi giải nén, thường vì định dạng kéo dài xuống dưới phần dữ liệu. Hãy chép các dòng sang bảng tính mới, hoặc lưu bảng tính thành CSV UTF-8 rồi tải tệp đó lên.",
			"A sheet is larger than 8 MB once unpacked, usually because its formatting runs far below the data. Copy the rows into a new workbook, or save the sheet as CSV UTF-8 and upload that.")), true
	case errors.Is(err, tabular.ErrPartTooLarge):
		return unreadable(httpx.Text(ctx,
			"Một phần của bảng tính, không phải trang tính, quá lớn. Hãy chép các dòng sang bảng tính mới, hoặc lưu bảng tính thành CSV UTF-8 rồi tải tệp đó lên.",
			"A part of the workbook other than a sheet is too large. Copy the rows into a new workbook, or save the sheet as CSV UTF-8 and upload that.")), true
	}
	return importRefusal{}, false
}

// refuseShape words a file whose rows or layout the reader could not take.
// ErrLayout and ErrNotWorksheet wrap ErrFormat, so they are asked first.
func refuseShape(ctx context.Context, err error) (importRefusal, bool) {
	switch {
	case errors.Is(err, tabular.ErrEmpty):
		return importRefusal{http.StatusUnprocessableEntity, openapi.USERIMPORTFILEEMPTY, httpx.Text(ctx,
			"Tệp không có dòng tiêu đề. Dòng đầu tiên cần có các cột full_name và email.",
			"The file has no header row. The first row needs the columns full_name and email.")}, true
	case errors.Is(err, tabular.ErrTooManyRows):
		return importRefusal{http.StatusUnprocessableEntity, openapi.USERIMPORTTOOMANYROWS, httpx.Text(ctx,
			"Tệp có hơn 2.000 dòng dữ liệu. Hãy chia thành nhiều tệp, mỗi tệp tối đa 2.000 dòng.",
			"The file has more than 2,000 data rows. Split it into several files of at most 2,000 rows each.")}, true
	case errors.Is(err, tabular.ErrTooManyColumns):
		return unreadable(httpx.Text(ctx,
			"Có dòng dùng hơn 20 cột. Chỉ cần các cột full_name, email, role và class; hãy bỏ các cột còn lại.",
			"A row uses more than 20 columns. Only full_name, email, role and class are needed; remove the others.")), true
	case errors.Is(err, tabular.ErrCellTooLong):
		return unreadable(httpx.Text(ctx,
			"Có ô dài hơn 1.000 ký tự. Hãy kiểm tra lại xem đây có đúng là danh sách người dùng không.",
			"A cell is longer than 1,000 characters. Check that this is the list of users.")), true
	case errors.Is(err, tabular.ErrTooManySheets):
		return unreadable(httpx.Text(ctx,
			"Bảng tính có hơn 256 trang tính. Hãy lưu danh sách vào một tệp riêng, hoặc lưu bảng tính thành CSV UTF-8 rồi tải tệp đó lên.",
			"The workbook has more than 256 sheets. Save the list on its own, or save the sheet as CSV UTF-8 and upload that.")), true
	case errors.Is(err, tabular.ErrNotWorksheet):
		return unreadable(httpx.Text(ctx,
			"Trang tính đầu tiên của bảng tính là biểu đồ, không phải bảng dữ liệu. Hãy đặt danh sách ở trang tính đầu tiên, hoặc lưu bảng tính thành CSV UTF-8 rồi tải tệp đó lên.",
			"The workbook's first sheet is a chart, not a table. Put the list on the first sheet, or save the sheet as CSV UTF-8 and upload that.")), true
	case errors.Is(err, tabular.ErrLayout):
		return unreadable(httpx.Text(ctx,
			"Bảng tính này không dùng cấu trúc tệp chuẩn của Excel nên không đọc được. Hãy mở nó bằng Excel hoặc LibreOffice, lưu lại thành .xlsx hoặc CSV UTF-8 rồi tải lên.",
			"This workbook does not use the standard Excel file layout, so it cannot be read. Open it in Excel or LibreOffice, save it again as .xlsx or CSV UTF-8, and upload that.")), true
	case errors.Is(err, tabular.ErrFormat):
		return unreadable(httpx.Text(ctx,
			"Không đọc được tệp này như một bảng. Tệp có thể bị hỏng hoặc không phải CSV hay XLSX. Hãy lưu bảng tính thành CSV UTF-8 rồi tải tệp đó lên.",
			"This file cannot be read as a table. It may be damaged or not a CSV or XLSX file. Save the sheet as CSV UTF-8 and upload that.")), true
	}
	return importRefusal{}, false
}

func headerMessage(ctx context.Context, p domain.ImportHeaderProblem) string {
	names := func(fields []domain.ImportField) string {
		out := make([]string, len(fields))
		for i, f := range fields {
			out[i] = string(f)
		}
		return strings.Join(out, ", ")
	}
	var problems []string
	if len(p.Missing) > 0 {
		problems = append(problems, httpx.Text(ctx, "thiếu cột bắt buộc: ", "is missing the required column: ")+names(p.Missing))
	}
	if len(p.Repeated) > 0 {
		problems = append(problems, httpx.Text(ctx, "có cột bị lặp: ", "repeats a column: ")+names(p.Repeated))
	}
	return httpx.Text(ctx, "Dòng tiêu đề ", "The header row ") + strings.Join(problems, "; ") +
		httpx.Text(ctx, ". Cần các cột full_name và email; role và class là tuỳ chọn.", ". The columns full_name and email are required; role and class are optional.")
}
