package domain

import "strings"

// Column is one column of an export file: its header, and what it holds in
// Vietnamese and in English, as the README of the ZIP words it.
type Column struct {
	Name string
	VI   string
	EN   string
}

// File is one CSV of the export, with its columns in order and the two
// sentences that say what it holds.
type File struct {
	Name    string
	VI      string
	EN      string
	Columns []Column
}

// Header lists the column names, the first row of the file.
func (f File) Header() []string {
	header := make([]string, len(f.Columns))
	for i, c := range f.Columns {
		header[i] = c.Name
	}
	return header
}

// The files of an export, by name.
const (
	UsersFile          = "users.csv"
	RolesFile          = "roles.csv"
	ClassesFile        = "classes.csv"
	ClassMembersFile   = "class_members.csv"
	TestsFile          = "tests.csv"
	TestVersionsFile   = "test_versions.csv"
	AssignmentsFile    = "assignments.csv"
	AttemptsFile       = "attempts.csv"
	AnswersFile        = "answers.csv"
	ReadmeFile         = "README.txt"
	ArchiveContentType = "application/zip"
)

func col(name, vi, en string) Column { return Column{Name: name, VI: vi, EN: en} }

var catalogue = []File{
	{UsersFile, "Mọi tài khoản: học viên, giáo viên và quản trị viên. Không có mật khẩu hay mã bí mật nào.",
		"Every account: students, teachers and admins. It holds no password and no secret.", []Column{
			col("id", "Mã tài khoản", "Account ID"),
			col("email", "Email đăng nhập", "Sign-in email"),
			col("full_name", "Họ và tên", "Full name"),
			col("display_name", "Tên hiển thị, nếu có", "Display name, if set"),
			col("phone", "Số điện thoại, nếu có", "Phone number, if set"),
			col("role_id", "Mã vai trò (roles.csv)", "Role ID (roles.csv)"),
			col("role_name", "Tên vai trò", "Role name"),
			col("disabled_at", "Lúc tài khoản bị vô hiệu hoá; trống nếu đang hoạt động", "When the account was disabled; empty while it is active"),
			col("must_change_password", "Phải đổi mật khẩu ở lần đăng nhập tới", "Must change the password at the next sign-in"),
			col("locale", "Ngôn ngữ đã chọn (vi hoặc en)", "Chosen language (vi or en)"),
			col("time_zone", "Múi giờ đã chọn", "Chosen time zone"),
			col("created_by", "Mã tài khoản đã tạo ra tài khoản này", "ID of the account that created this one"),
			col("created_at", "Lúc tạo tài khoản", "When the account was created"),
			col("last_sign_in_at", "Lần đăng nhập gần nhất", "Latest sign-in"),
			col("last_sign_in_method", "Cách đăng nhập gần nhất (password, google hoặc set_password_link)", "Method of the latest sign-in (password, google or set_password_link)"),
			col("last_active_at", "Lần hoạt động gần nhất", "Latest activity"),
			col("password_changed_at", "Lần đổi mật khẩu gần nhất", "Latest password change"),
		}},
	{RolesFile, "Các vai trò và quyền của từng vai trò.", "The roles and the permissions of each.", []Column{
		col("id", "Mã vai trò", "Role ID"),
		col("builtin_key", "Khoá của vai trò có sẵn (student, teacher, admin, assistant); trống với vai trò tự tạo", "Key of a built-in role (student, teacher, admin, assistant); empty for a custom role"),
		col("name", "Tên vai trò", "Role name"),
		col("description", "Mô tả", "Description"),
		col("permissions", "Các quyền, cách nhau bằng dấu chấm phẩy", "The permissions, separated by semicolons"),
		col("created_at", "Lúc tạo", "When it was created"),
		col("updated_at", "Lần sửa gần nhất", "Latest change"),
	}},
	{ClassesFile, "Các lớp. Mã tham gia lớp không có trong tệp này.", "The classes. Join codes are not in this file.", []Column{
		col("id", "Mã lớp", "Class ID"),
		col("name", "Tên lớp", "Class name"),
		col("description", "Mô tả", "Description"),
		col("teacher_id", "Mã tài khoản của giáo viên phụ trách (users.csv)", "Account ID of the class's teacher (users.csv)"),
		col("teacher_name", "Tên giáo viên phụ trách", "Name of the class's teacher"),
		col("self_join_enabled", "Học viên có thể tự vào lớp bằng mã hay không", "Whether students may join with a code"),
		col("schedule_label", "Lịch học, nếu có", "Schedule, if set"),
		col("room", "Phòng học, nếu có", "Room, if set"),
		col("created_at", "Lúc tạo lớp", "When the class was created"),
		col("updated_at", "Lần sửa gần nhất", "Latest change"),
		col("archived_at", "Lúc lưu trữ lớp; trống nếu lớp đang hoạt động", "When the class was archived; empty while it is active"),
	}},
	{ClassMembersFile, "Học viên của từng lớp.", "The students of each class.", []Column{
		col("class_id", "Mã lớp (classes.csv)", "Class ID (classes.csv)"),
		col("user_id", "Mã tài khoản của học viên (users.csv)", "Account ID of the student (users.csv)"),
		col("joined_via", "Cách vào lớp (join_code hoặc admin)", "How the student joined (join_code or admin)"),
		col("joined_at", "Lúc vào lớp", "When the student joined"),
		col("added_by", "Mã tài khoản đã thêm học viên, nếu có", "ID of the account that added the student, if any"),
	}},
	{TestsFile, "Các bài kiểm tra, kể cả bản nháp và bài đã xoá. Câu hỏi nằm trong từng phiên bản, không nằm trong tệp này.",
		"The tests, drafts and deleted ones included. The questions belong to the versions, not to this file.", []Column{
			col("id", "Mã bài kiểm tra", "Test ID"),
			col("title", "Tiêu đề", "Title"),
			col("description", "Mô tả", "Description"),
			col("status", "Trạng thái (draft, published hoặc archived)", "Status (draft, published or archived)"),
			col("current_version", "Phiên bản đang dùng", "The version in use"),
			col("last_published_version", "Phiên bản phát hành gần nhất", "The latest published version"),
			col("owner_id", "Mã tài khoản của chủ sở hữu (users.csv)", "Account ID of the owner (users.csv)"),
			col("created_by", "Mã tài khoản đã tạo bài", "ID of the account that created the test"),
			col("created_at", "Lúc tạo", "When it was created"),
			col("updated_at", "Lần sửa gần nhất", "Latest change"),
			col("deleted_at", "Lúc xoá; trống nếu chưa xoá", "When it was deleted; empty if it was not"),
		}},
	{TestVersionsFile, "Các phiên bản đã phát hành của từng bài kiểm tra.", "The published versions of each test.", []Column{
		col("id", "Mã phiên bản", "Version ID"),
		col("test_id", "Mã bài kiểm tra (tests.csv)", "Test ID (tests.csv)"),
		col("version", "Số phiên bản", "Version number"),
		col("total_points", "Tổng điểm", "Total points"),
		col("published_at", "Lúc phát hành", "When it was published"),
		col("published_by", "Mã tài khoản đã phát hành", "ID of the account that published it"),
		col("delivery_version", "Kiểu chia phần của bài", "How the paper is delivered"),
		col("change_note", "Ghi chú thay đổi, nếu có", "Change note, if any"),
	}},
	{AssignmentsFile, "Các bài giao và cách mỗi bài được làm và xem lại.", "The assignments, and how each is taken and reviewed.", []Column{
		col("id", "Mã bài giao", "Assignment ID"),
		col("test_id", "Mã bài kiểm tra (tests.csv)", "Test ID (tests.csv)"),
		col("test_version_id", "Mã phiên bản được giao (test_versions.csv)", "ID of the version assigned (test_versions.csv)"),
		col("class_ids", "Mã các lớp được giao, cách nhau bằng dấu chấm phẩy", "IDs of the classes it is assigned to, separated by semicolons"),
		col("student_ids", "Mã các học viên được giao riêng, cách nhau bằng dấu chấm phẩy", "IDs of the students it is assigned to one by one, separated by semicolons"),
		col("opens_at", "Lúc mở", "When it opens"),
		col("closes_at", "Lúc đóng theo lịch", "When it closes as scheduled"),
		col("closed_at", "Lúc đóng thực tế, nếu đóng sớm", "When it was closed, if closed early"),
		col("duration_minutes", "Thời gian làm bài, tính bằng phút", "Time allowed, in minutes"),
		col("max_attempts", "Số lần làm bài tối đa", "Most attempts allowed"),
		col("shuffle_questions", "Có xáo trộn câu hỏi", "Whether questions are shuffled"),
		col("shuffle_options", "Có xáo trộn đáp án", "Whether options are shuffled"),
		col("review_release", "Khi nào học viên được xem lại (on_submit hoặc after_close)", "When students may review (on_submit or after_close)"),
		col("review_show_score", "Cho xem điểm", "Whether the score is shown"),
		col("review_show_correct_answers", "Cho xem đáp án đúng", "Whether correct answers are shown"),
		col("review_show_explanations", "Cho xem lời giải thích", "Whether explanations are shown"),
		col("review_show_class_average", "Cho xem điểm trung bình lớp", "Whether the class average is shown"),
		col("integrity_require_fullscreen", "Yêu cầu toàn màn hình", "Whether full screen is required"),
		col("integrity_block_copy_paste", "Chặn sao chép và dán", "Whether copy and paste are blocked"),
		col("integrity_max_focus_loss", "Số lần rời khỏi bài được phép", "Times a student may leave the test"),
		col("integrity_on_limit_exceeded", "Việc xảy ra khi vượt số lần rời bài (warn, flag hoặc auto_submit)", "What happens past that limit (warn, flag or auto_submit)"),
		col("integrity_min_away_ms", "Thời gian rời bài tối thiểu để được tính, tính bằng mili giây", "Shortest time away that counts, in milliseconds"),
		col("student_note", "Lời nhắn cho học viên, nếu có", "Note to students, if any"),
		col("created_by", "Mã tài khoản đã tạo bài giao", "ID of the account that created it"),
		col("created_at", "Lúc tạo", "When it was created"),
		col("updated_at", "Lần sửa gần nhất", "Latest change"),
		col("published_at", "Lúc phát hành; trống nếu chưa phát hành", "When it was published; empty if it was not"),
	}},
	{AttemptsFile, "Các lượt làm bài. Không có phiên làm bài hay mã bí mật nào.", "The attempts. No session and no secret is in it.", []Column{
		col("id", "Mã lượt làm bài", "Attempt ID"),
		col("assignment_id", "Mã bài giao (assignments.csv)", "Assignment ID (assignments.csv)"),
		col("test_version_id", "Mã phiên bản (test_versions.csv)", "Version ID (test_versions.csv)"),
		col("student_id", "Mã tài khoản của học viên (users.csv)", "Account ID of the student (users.csv)"),
		col("attempt_no", "Lần làm thứ mấy của học viên", "The student's attempt number"),
		col("status", "Trạng thái (in_progress, submitted, timed_out, graded hoặc voided)", "Status (in_progress, submitted, timed_out, graded or voided)"),
		col("started_at", "Lúc bắt đầu", "When it started"),
		col("deadline_at", "Hạn nộp bài", "Deadline"),
		col("submitted_at", "Lúc nộp bài", "When it was submitted"),
		col("graded_at", "Lúc chấm xong", "When grading finished"),
		col("score_earned", "Điểm đạt được", "Points earned"),
		col("score_total", "Tổng điểm của bài", "Points available"),
		col("focus_loss_count", "Số lần rời khỏi bài", "Times the student left the test"),
		col("flagged", "Giáo viên cần xem lượt làm bài này", "Whether the attempt is flagged for the teacher's attention"),
		col("void_reason", "Lý do huỷ lượt làm bài, nếu có", "Reason the attempt was voided, if it was"),
		col("teacher_note", "Ghi chú của giáo viên, nếu có", "Teacher's note, if any"),
	}},
}

var answersFile = File{AnswersFile, "Mọi câu trả lời của mọi lượt làm bài. Chỉ có khi bạn chọn kèm câu trả lời.",
	"Every answer of every attempt. It is present only when you chose to include answers.", []Column{
		col("attempt_id", "Mã lượt làm bài (attempts.csv)", "Attempt ID (attempts.csv)"),
		col("question_id", "Mã câu hỏi trong phiên bản", "ID of the question in the version"),
		col("answer", "Câu trả lời của học viên, dạng JSON", "The student's answer, as JSON"),
		col("removed", "true nếu câu trả lời đã bị xoá theo thời hạn lưu giữ; điểm vẫn còn", "true if the answer was removed after its retention period; the scores stay"),
		col("requires_manual", "Cần giáo viên chấm tay", "Needs the teacher to grade it"),
		col("auto_score", "Điểm tự động", "Automatic score"),
		col("manual_score", "Điểm giáo viên chấm", "Teacher's score"),
		col("final_score", "Điểm cuối cùng (điểm giáo viên nếu có, nếu không là điểm tự động)", "Final score (the teacher's if there is one, else the automatic one)"),
		col("band", "Band của bài luận đã chấm, từ 3 đến 9; trống với các dạng câu khác", "The band of a graded essay, 3 to 9; empty for other types"),
		col("criteria", "Band của từng tiêu chí chấm, dạng mã tiêu chí=band, cách nhau bằng dấu chấm phẩy", "The band of each rubric criterion, as criterion ID=band, separated by semicolons"),
		col("grader_comment", "Nhận xét khi chấm, nếu có", "Grader's comment, if any"),
		col("graded_by", "Mã tài khoản đã chấm", "ID of the account that graded it"),
		col("graded_at", "Lúc chấm", "When it was graded"),
		col("updated_at", "Lần lưu gần nhất", "Latest save"),
	}}

// Files lists the CSV files of an export in the order the ZIP holds them:
// answers.csv closes the list, and only when includeAnswers is true.
func Files(includeAnswers bool) []File {
	out := make([]File, len(catalogue), len(catalogue)+1)
	copy(out, catalogue)
	if includeAnswers {
		out = append(out, answersFile)
	}
	return out
}

// Readme words the README.txt of a ZIP of files: Vietnamese first, then
// English, each saying what the ZIP is, how its files are written and what
// every column of every file holds.
func Readme(files []File) string {
	var b strings.Builder
	for i, lang := range []struct {
		intro    []string
		describe func(File) string
		column   func(Column) string
	}{
		{viIntro, func(f File) string { return f.VI }, func(c Column) string { return c.VI }},
		{enIntro, func(f File) string { return f.EN }, func(c Column) string { return c.EN }},
	} {
		if i > 0 {
			b.WriteString("\r\n==========\r\n\r\n")
		}
		for _, line := range lang.intro {
			b.WriteString(line + "\r\n")
		}
		for _, f := range files {
			b.WriteString("\r\n" + f.Name + "\r\n")
			b.WriteString(lang.describe(f) + "\r\n")
			for _, c := range f.Columns {
				b.WriteString("  " + c.Name + ": " + lang.column(c) + "\r\n")
			}
		}
	}
	return b.String()
}

var viIntro = []string{
	"BẢN XUẤT DỮ LIỆU QUIZZIVY",
	"",
	"Tệp ZIP này gồm các tệp CSV, mỗi tệp một loại dữ liệu. Mọi tệp đều mã hoá UTF-8 và mở được bằng Excel hoặc Google Trang tính.",
	"Mọi thời điểm theo giờ UTC, dạng ISO 8601 (ví dụ 2026-10-14T09:30:00Z). Ô trống nghĩa là không có giá trị.",
	"Giá trị đúng sai viết là true hoặc false. Giá trị bắt đầu bằng = + - hoặc @ được thêm dấu nháy đơn ở đầu để bảng tính không chạy nó như công thức.",
	"Các cột có đuôi _id nối các tệp với nhau: cột id của tệp này là cột _id của tệp kia.",
	"Tệp này chứa dữ liệu cá nhân. Hãy lưu ở nơi an toàn và xoá khi không còn cần.",
}

var enIntro = []string{
	"QUIZZIVY DATA EXPORT",
	"",
	"This ZIP holds CSV files, one for each kind of data. Every file is UTF-8 and opens in Excel or Google Sheets.",
	"Every time is in UTC, written in ISO 8601 (for example 2026-10-14T09:30:00Z). An empty cell means there is no value.",
	"Yes/no values are written true or false. A value that begins with = + - or @ has a single quote put before it, so a spreadsheet does not run it as a formula.",
	"Columns ending in _id join the files: a file's id column is another file's _id column.",
	"This ZIP holds personal data. Keep it somewhere safe and delete it when you no longer need it.",
}
