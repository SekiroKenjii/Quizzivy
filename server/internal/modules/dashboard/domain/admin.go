package domain

import (
	"errors"
	"time"

	auditdomain "quizzivy/internal/modules/audit/domain"
	"quizzivy/internal/shared/limits"
)

// ErrAdminUnavailable means the Admin readings have no organization port to name the centre from.
var ErrAdminUnavailable = errors.New("dashboard: admin readings unavailable")

// DefaultZone is the calendar zone the centre's days are counted in when none is configured.
const DefaultZone = "Asia/Ho_Chi_Minh"

// ActiveDays is how many local days the Overview's chart covers, today included.
const ActiveDays = 14

// ActiveWeekDays is how many local days make "this week", today included.
const ActiveWeekDays = 7

// RecentChanges is how many audit entries the Overview quotes.
const RecentChanges = 5

// AdminViewer says which parts of the Admin readings a caller's keys open: Users is people.users.manage, Audit system.audit.read, Settings system.settings.write, Credits ai.credits.manage and All scope.all.
type AdminViewer struct {
	UserID   string
	Users    bool
	Audit    bool
	Settings bool
	Credits  bool
	All      bool
}

// Accounts is what one pass over the users counts. A student is a student target of app.student_like_roles and a teacher is every other account, an Admin included: Teachers are the enabled ones and TeachersDisabled the disabled, so the two never overlap. Total and Students count disabled accounts, Enabled does not. Never signed in and must change password apply DG-54's order to the enabled accounts.
type Accounts struct {
	Total              int
	Enabled            int
	Students           int
	StudentsNew        int
	Teachers           int
	TeachersDisabled   int
	MustChangePassword int
	NeverSignedIn      int
	// OldestNeverSignedIn is when the oldest of the NeverSignedIn accounts was created, nil when there are none.
	OldestNeverSignedIn *time.Time
}

// People is the Overview's count of the centre's accounts.
type People struct {
	Total                int
	ActiveThisWeek       int
	Teachers             int
	TeachersDisabled     int
	Students             int
	StudentsNewThisMonth int
}

// ClassCounts is how many classes are open and how many are archived.
type ClassCounts struct{ Active, Archived int }

// DisabledTeacherClasses is how many classes have a disabled teacher, and the newest of them.
type DisabledTeacherClasses struct {
	Count       int
	ClassName   string
	TeacherName string
}

// ActiveDay is who made an authenticated request on one local day: Staff is every account that is not a student target.
type ActiveDay struct {
	Date     time.Time
	Students int
	Staff    int
}

// DailyActive is the Overview's chart: ActiveDays days, oldest first and today last, with the mean of students plus staff rounded half up.
type DailyActive struct {
	Days    []ActiveDay
	Average int
}

// LocalDay is the calendar date t falls on in loc, at midnight UTC: the form a date column is read in.
func LocalDay(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// NewDailyActive lays found over the ActiveDays days ending on today, a day with no row counting zero, and averages them.
func NewDailyActive(today time.Time, found []ActiveDay) DailyActive {
	byDay := make(map[time.Time]ActiveDay, len(found))
	for _, d := range found {
		byDay[d.Date] = d
	}
	out := DailyActive{Days: make([]ActiveDay, ActiveDays)}
	sum := 0
	for i := range out.Days {
		date := today.AddDate(0, 0, i-ActiveDays+1)
		day := byDay[date]
		day.Date = date
		out.Days[i] = day
		sum += day.Students + day.Staff
	}
	out.Average = (2*sum + ActiveDays) / (2 * ActiveDays)
	return out
}

// AttentionKind names what an attention row is about.
type AttentionKind string

// The rows the Overview can show, in the order it shows them.
const (
	AttentionCreditRequests   AttentionKind = "ai_credit_requests"
	AttentionMustChange       AttentionKind = "must_change_password"
	AttentionNeverSignedIn    AttentionKind = "never_signed_in"
	AttentionDisabledTeachers AttentionKind = "disabled_teacher_classes"
	AttentionStorage          AttentionKind = "storage"
	AttentionEmailHeld        AttentionKind = "email_held"
)

// AttentionDetail holds the facts a row quotes; a field its kind does not use is nil.
type AttentionDetail struct {
	OldestCreatedAt *time.Time
	ClassName       *string
	TeacherName     *string
	MediaPercent    *int
	ImportsPercent  *int
	StoragePercent  *int
	DatabasePercent *int
	HeldUntil       *time.Time
	Used            *int
	Allowance       *int
}

// Attention is one thing that needs doing. Count is at least 1.
type Attention struct {
	Kind   AttentionKind
	Count  int
	Detail *AttentionDetail
}

// WaitingRequest is a teacher's request for more AI credits that no Admin has answered, with the standing it was made at.
type WaitingRequest struct {
	TeacherName string
	Used        int64
	Allowance   int64
}

// CreditsView is the AI credits as the Admin readings quote them: the month's use of the pool and the requests waiting, oldest first.
type CreditsView struct {
	Credits Credits
	Waiting []WaitingRequest
}

// Attention is the row that says teachers are waiting for credits, and nil when none is.
func (c CreditsView) Attention() *Attention {
	if len(c.Waiting) == 0 {
		return nil
	}
	first := c.Waiting[0]
	used, allowance := int(first.Used), int(first.Allowance)
	return &Attention{
		Kind: AttentionCreditRequests, Count: len(c.Waiting),
		Detail: &AttentionDetail{TeacherName: &first.TeacherName, Used: &used, Allowance: &allowance},
	}
}

// AdminOverview is the Admin console's Overview. A section the viewer's keys do not open is nil; RecentChanges is nil when withheld and empty when the log is.
type AdminOverview struct {
	OrganizationName string
	People           *People
	Classes          *ClassCounts
	DailyActive      *DailyActive
	Attention        []Attention
	RecentChanges    []auditdomain.Entry
	Storage          *Usage
}

// AdminSummary is the Admin sidebar's counts. MustChangePassword is nil without people.users.manage.
type AdminSummary struct{ MustChangePassword *int }

// EmailWindow is the local day and month the email counts read, and now.
type EmailWindow struct {
	Now                   time.Time
	DayStart, NextDay     time.Time
	MonthStart, NextMonth time.Time
}

// Held is what the centre holds and uses, counted from its tables: the seats
// and teachers as limits.CountAccounts counts them, the bytes of its stored
// files as limits.CountHeld sums them, the fullest teacher's media library,
// the documents of the imports still open, the mail sent and held, and the
// database's approximate size.
type Held struct {
	Seats           int
	Teachers        int
	MediaBytes      int64
	ImportBytes     int64
	ExportBytes     int64
	ImportCount     int
	LargestLibrary  int64
	OpenImportBytes int64
	EmailsToday     int
	EmailsMonth     int
	EmailsHeld      int
	HeldUntil       *time.Time
	DatabaseBytes   int64
	Attempts        int64
}

// Credits is the month's AI credits used against the centre pool.
type Credits struct{ Used, Pool int64 }

// Count is a number of accounts against a limit, nil when there is none.
type Count struct {
	Used  int
	Limit *int
}

// Bytes is a size against a limit, nil when there is none.
type Bytes struct {
	Bytes int64
	Limit *int64
}

// Media is the media library's size, the fullest teacher's library and the limit one library may hold.
type Media struct {
	Bytes           int64
	LargestLibrary  int64
	PerTeacherLimit *int64
}

// Imports is the kept import documents and their imports, the documents of the open imports and their limits.
type Imports struct {
	Bytes           int64
	Count           int
	OpenBytes       int64
	Limit           *int64
	PerTeacherLimit *int64
}

// Email is the mail sent today and this month, the messages a cap holds and the caps.
type Email struct {
	Today, Month, Held       int
	HeldUntil                *time.Time
	DailyLimit, MonthlyLimit *int
}

// Usage is what the centre uses against its limits. AICredits is nil without ai.credits.manage and while the assistant is off.
type Usage struct {
	Seats         Count
	Teachers      Count
	Storage       Bytes
	Media         Media
	Imports       Imports
	Email         Email
	Database      Bytes
	AttemptsKept  int64
	AICredits     *Credits
	WarnAtPercent int
}

// NewUsage sets what is held against the resolved limits.
func NewUsage(l limits.Limits, held Held, credits *Credits) Usage {
	return Usage{
		Seats:    Count{Used: held.Seats, Limit: intOf(l, limits.Seats)},
		Teachers: Count{Used: held.Teachers, Limit: intOf(l, limits.Teachers)},
		Storage:  Bytes{Bytes: held.MediaBytes + held.ImportBytes + held.ExportBytes, Limit: int64Of(l, limits.StorageBytes)},
		Media:    Media{Bytes: held.MediaBytes, LargestLibrary: held.LargestLibrary, PerTeacherLimit: int64Of(l, limits.MediaPerTeacherBytes)},
		Imports: Imports{Bytes: held.ImportBytes, Count: held.ImportCount, OpenBytes: held.OpenImportBytes,
			Limit: int64Of(l, limits.ImportBytes), PerTeacherLimit: int64Of(l, limits.ImportPerTeacherBytes)},
		Email: Email{Today: held.EmailsToday, Month: held.EmailsMonth, Held: held.EmailsHeld, HeldUntil: held.HeldUntil,
			DailyLimit: intOf(l, limits.EmailsPerDay), MonthlyLimit: intOf(l, limits.EmailsPerMonth)},
		Database:      Bytes{Bytes: held.DatabaseBytes, Limit: int64Of(l, limits.DatabaseBytes)},
		AttemptsKept:  held.Attempts,
		AICredits:     credits,
		WarnAtPercent: l.WarnPercent(),
	}
}

func intOf(l limits.Limits, n limits.Name) *int {
	v, ok := l.Of(n)
	if !ok {
		return nil
	}
	i := int(v)
	return &i
}

func int64Of(l limits.Limits, n limits.Name) *int64 {
	v, ok := l.Of(n)
	if !ok {
		return nil
	}
	return &v
}

// Percent is used as a share of limit, rounded half up.
func Percent(used, limit int64) int {
	return int((200*used + limit) / (2 * limit))
}

// StorageAttention is the storage row: the file storage, fullest media
// library, open imports and database readings with a limit that are at
// u.WarnAtPercent or more once rounded to the percent the page shows, each
// quoted, and nil when none is.
func StorageAttention(u Usage) *Attention {
	detail := AttentionDetail{}
	count := 0
	reading := func(used int64, limit *int64, into **int) {
		if limit == nil {
			return
		}
		if percent := Percent(used, *limit); percent >= u.WarnAtPercent {
			*into = &percent
			count++
		}
	}
	reading(u.Storage.Bytes, u.Storage.Limit, &detail.StoragePercent)
	reading(u.Media.LargestLibrary, u.Media.PerTeacherLimit, &detail.MediaPercent)
	reading(u.Imports.OpenBytes, u.Imports.Limit, &detail.ImportsPercent)
	reading(u.Database.Bytes, u.Database.Limit, &detail.DatabasePercent)
	if count == 0 {
		return nil
	}
	return &Attention{Kind: AttentionStorage, Count: count, Detail: &detail}
}

// EmailAttention is the row that says a cap holds mail for later, and nil when none is held.
func EmailAttention(u Usage) *Attention {
	if u.Email.Held == 0 {
		return nil
	}
	return &Attention{Kind: AttentionEmailHeld, Count: u.Email.Held, Detail: &AttentionDetail{HeldUntil: u.Email.HeldUntil}}
}
