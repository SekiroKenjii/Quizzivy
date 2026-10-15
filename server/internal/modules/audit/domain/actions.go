package domain

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Action is one thing the server writes to the audit log: its key, and the
// sentence that words it in Vietnamese and in English. The sentences are the
// ones the web client shows under audit.action.<key>; a search for an action
// matches them here so the client and the search agree on what an entry says.
type Action struct {
	Key string
	VI  string
	EN  string
}

var catalogue = []Action{
	{"ai.allowance.updated", "Đã cập nhật hạn mức AI của giáo viên", "Updated a teacher's AI allowance"},
	{"ai.credit_request.approved", "Đã duyệt yêu cầu thêm tín dụng AI", "Approved an AI credit request"},
	{"ai.credit_request.declined", "Đã từ chối yêu cầu thêm tín dụng AI", "Declined an AI credit request"},
	{"ai.credits.added", "Đã thêm tín dụng AI", "Added AI credits"},
	{"ai.rules.updated", "Đã cập nhật quy tắc AI", "Updated the AI rules"},
	{"assignment.closed", "Đã đóng bài giao", "Closed an assignment"},
	{"assignment.created", "Đã tạo bài giao", "Created an assignment"},
	{"assignment.deleted", "Đã xoá bài giao", "Deleted an assignment"},
	{"assignment.duplicated", "Đã nhân bản bài giao", "Duplicated an assignment"},
	{"assignment.extended", "Đã gia hạn bài giao vì bảo trì", "Extended an assignment for maintenance"},
	{"assignment.override_extended", "Đã gia hạn ngoại lệ của học viên vì bảo trì", "Extended a student's exception for maintenance"},
	{"assignment.override_removed", "Đã gỡ ngoại lệ của học viên", "Removed a student's exception"},
	{"assignment.override_set", "Đã đặt ngoại lệ cho học viên", "Set an exception for a student"},
	{"assignment.published", "Đã phát hành bài giao", "Published an assignment"},
	{"assignment.reopened", "Đã mở lại bài giao", "Reopened an assignment"},
	{"assignment.updated", "Đã cập nhật bài giao", "Updated an assignment"},
	{"attempt.extended", "Đã gia hạn lượt làm bài vì bảo trì", "Extended an attempt for maintenance"},
	{"attempt.flagged", "Đã gắn cờ lượt làm bài", "Flagged an attempt"},
	{"attempt.reset", "Đã đặt lại lượt làm bài", "Reset an attempt"},
	{"attempt.unflagged", "Đã bỏ cờ lượt làm bài", "Cleared the flag on an attempt"},
	{"attempt.voided", "Đã huỷ lượt làm bài", "Voided an attempt"},
	{"audit.exported", "Đã xuất nhật ký hoạt động", "Exported the audit log"},
	{"class.archived", "Đã lưu trữ lớp", "Archived a class"},
	{"class.created", "Đã tạo lớp", "Created a class"},
	{"class.deleted", "Đã xoá lớp", "Deleted a class"},
	{"class.join_code_issued", "Đã cấp mã tham gia lớp", "Issued a class join code"},
	{"class.join_code_revoked", "Đã thu hồi mã tham gia lớp", "Revoked a class join code"},
	{"class.join_code_rotated", "Đã đổi mã tham gia lớp", "Rotated a class join code"},
	{"class.member_added", "Đã thêm học viên vào lớp", "Added a student to a class"},
	{"class.member_enrolled", "Học viên đã vào lớp bằng mã tham gia", "A student joined a class with a code"},
	{"class.member_removed", "Đã xoá học viên khỏi lớp", "Removed a student from a class"},
	{"class.restored", "Đã khôi phục lớp", "Restored a class"},
	{"data_export.downloaded", "Đã tải bản xuất dữ liệu", "Downloaded a data export"},
	{"data_export.requested", "Đã yêu cầu bản xuất dữ liệu", "Requested a data export"},
	{"import.cancelled", "Đã huỷ lần nhập Word", "Cancelled a Word import"},
	{"import.closed_idle", "Đã tự đóng lần nhập Word không hoạt động", "Closed an idle Word import"},
	{"import.committed", "Đã lưu câu hỏi từ lần nhập Word", "Saved the questions of a Word import"},
	{"import.created", "Đã tạo lần nhập Word", "Created a Word import"},
	{"import.files_removed", "Đã xoá tệp của lần nhập Word", "Removed the files of a Word import"},
	{"import.queued", "Đã đưa lần nhập Word vào hàng chờ", "Queued a Word import"},
	{"import.reprocess_cancelled", "Đã huỷ xử lý lại lần nhập Word", "Cancelled reprocessing a Word import"},
	{"import.reprocessed_adopted", "Đã dùng kết quả xử lý lại của lần nhập Word", "Adopted the reprocessed result of a Word import"},
	{"import.source_completed", "Đã tải xong tệp nguồn của lần nhập Word", "Finished uploading a Word import's source"},
	{"import.source_reserved", "Đã giữ chỗ tệp nguồn cho lần nhập Word", "Reserved a Word import's source"},
	{"integrity.retained", "Đã xoá nhật ký chống gian lận quá hạn lưu", "Deleted integrity events past their retention"},
	{"maintenance.cancelled", "Đã huỷ lịch bảo trì", "Cancelled a maintenance window"},
	{"maintenance.ended", "Đã kết thúc bảo trì", "Ended a maintenance window"},
	{"maintenance.scheduled", "Đã lên lịch bảo trì", "Scheduled a maintenance window"},
	{"media.deleted", "Đã xoá tệp phương tiện", "Deleted a media file"},
	{"media.purged", "Đã xoá hẳn tệp phương tiện", "Permanently removed a media file"},
	{"media.replaced", "Đã thay tệp phương tiện", "Replaced a media file"},
	{"media.updated", "Đã cập nhật tệp phương tiện", "Updated a media file"},
	{"media.uploaded", "Đã tải lên tệp phương tiện", "Uploaded a media file"},
	{"organization.updated", "Đã cập nhật thông tin trung tâm", "Updated the centre's settings"},
	{"question.created", "Đã tạo câu hỏi", "Created a question"},
	{"question.deleted", "Đã xoá câu hỏi", "Deleted a question"},
	{"question.updated", "Đã cập nhật câu hỏi", "Updated a question"},
	{"question_group.archived", "Đã lưu trữ nhóm câu hỏi", "Archived a question group"},
	{"question_group.created", "Đã tạo nhóm câu hỏi", "Created a question group"},
	{"question_group.deleted", "Đã xoá nhóm câu hỏi", "Deleted a question group"},
	{"question_group.removed", "Đã gỡ nhóm câu hỏi khỏi bài kiểm tra", "Removed a question group from a test"},
	{"question_group.restored", "Đã khôi phục nhóm câu hỏi", "Restored a question group"},
	{"question_group.updated", "Đã cập nhật nhóm câu hỏi", "Updated a question group"},
	{"refresh_token.reuse_detected", "Phát hiện một phiên đăng nhập bị dùng lại", "Detected a reused sign-in session"},
	{"role.created", "Đã tạo vai trò", "Created a role"},
	{"role.permissions_changed", "Đã thay đổi quyền của vai trò", "Changed a role's permissions"},
	{"role.updated", "Đã cập nhật vai trò", "Updated a role"},
	{"session.revoked", "Đã thu hồi một phiên đăng nhập", "Revoked a sign-in session"},
	{"session.revoked_others", "Đã thu hồi các phiên đăng nhập khác", "Revoked the other sign-in sessions"},
	{"student.anonymized", "Đã ẩn danh học viên", "Anonymized a student"},
	{"student.created", "Đã tạo tài khoản học viên", "Created a student account"},
	{"student.deleted", "Đã xoá tài khoản học viên", "Deleted a student account"},
	{"student.password_reset", "Đã đặt lại mật khẩu của học viên", "Reset a student's password"},
	{"student.updated", "Đã cập nhật tài khoản học viên", "Updated a student account"},
	{"test.created", "Đã tạo bài kiểm tra", "Created a test"},
	{"test.current_version_changed", "Đã đổi phiên bản hiện hành của bài kiểm tra", "Changed a test's current version"},
	{"test.deleted", "Đã xoá bài kiểm tra", "Deleted a test"},
	{"test.draft_restored", "Đã khôi phục bản nháp từ một phiên bản", "Restored the draft from a version"},
	{"test.duplicated", "Đã nhân bản bài kiểm tra", "Duplicated a test"},
	{"test.published", "Đã phát hành bài kiểm tra", "Published a test"},
	{"test.question_moved", "Đã chuyển câu hỏi giữa các phần", "Moved a question between sections"},
	{"test.updated", "Đã cập nhật bài kiểm tra", "Updated a test"},
	{"test.version_deleted", "Đã xoá một phiên bản của bài kiểm tra", "Deleted a version of a test"},
	{"user.avatar_removed", "Đã gỡ ảnh đại diện", "Removed a profile photo"},
	{"user.avatar_set", "Đã đặt ảnh đại diện", "Set a profile photo"},
	{"user.created", "Đã tạo tài khoản", "Created an account"},
	{"user.google_linked", "Đã liên kết tài khoản Google", "Linked a Google account"},
	{"user.google_unlinked", "Đã huỷ liên kết tài khoản Google", "Unlinked a Google account"},
	{"user.password_changed", "Đã đổi mật khẩu", "Changed a password"},
	{"user.password_link_requested", "Đã yêu cầu liên kết đặt mật khẩu", "Asked for a set-password link"},
	{"user.password_set", "Đã đặt mật khẩu bằng liên kết", "Set a password from a link"},
	{"user.preferences_updated", "Đã cập nhật tuỳ chọn cá nhân", "Updated personal preferences"},
	{"user.profile_updated", "Đã cập nhật hồ sơ", "Updated a profile"},
	{"user.renamed", "Đã đổi tên tài khoản", "Renamed an account"},
}

// Actions lists every action the server writes, in key order, each with its
// two sentences. The slice is a copy.
func Actions() []Action { return append([]Action(nil), catalogue...) }

// Language is a language the log is worded in.
type Language string

// The languages a sentence is written in.
const (
	Vietnamese Language = "vi"
	English    Language = "en"
)

// Sentence is what an entry of the action key says in lang. An action the
// catalogue does not hold is worded from its key, so an entry written by a
// newer server is still readable.
func Sentence(key string, lang Language) string {
	for _, a := range catalogue {
		if a.Key != key {
			continue
		}
		if lang == Vietnamese {
			return a.VI
		}
		return a.EN
	}
	return strings.ReplaceAll(strings.ReplaceAll(key, ".", " "), "_", " ")
}

// Fold lower-cases s and removes its accents, đ included, the way
// app.immutable_unaccent(lower(…)) does in the database, so a text folded here
// can be compared with one folded there.
func Fold(s string) string {
	var out strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		switch {
		case unicode.Is(unicode.Mn, r):
		case r == 'đ':
			out.WriteRune('d')
		default:
			out.WriteRune(r)
		}
	}
	return out.String()
}

// MatchingActions lists the keys of the actions whose Vietnamese or English
// sentence contains q, without regard to case or accents, in key order. An
// empty q matches none.
func MatchingActions(q string) []string {
	needle := Fold(strings.TrimSpace(q))
	if needle == "" {
		return nil
	}
	var keys []string
	for _, a := range catalogue {
		if strings.Contains(Fold(a.VI), needle) || strings.Contains(Fold(a.EN), needle) {
			keys = append(keys, a.Key)
		}
	}
	return keys
}
