package domain

import (
	"errors"
	"time"

	auditdomain "quizzivy/internal/modules/audit/domain"
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

// StorageWarnPercent is the share of a stated limit at which a storage meter needs attention.
const StorageWarnPercent = 80

// AdminViewer says which parts of the Admin readings a caller's keys open: Users is people.users.manage, Audit system.audit.read, Settings system.settings.write, Credits ai.credits.manage and All scope.all.
type AdminViewer struct {
	UserID   string
	Users    bool
	Audit    bool
	Settings bool
	Credits  bool
	All      bool
}

// Accounts is what one pass over the users counts. A student is a student target of app.student_like_roles, an admin holds the built-in Admin role and a teacher is everyone else; disabled accounts are counted in their role. Never signed in and must change password apply DG-54's order to the enabled accounts.
type Accounts struct {
	Total              int
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
)

// AttentionDetail holds the facts a row quotes; a field its kind does not use is nil.
type AttentionDetail struct {
	OldestCreatedAt *time.Time
	ClassName       *string
	TeacherName     *string
	MediaPercent    *int
	ImportsPercent  *int
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

// Plan is the plan the centre states: a name, a renewal date and the limits it holds. A zero limit is no limit and nothing is enforced (DG-55).
type Plan struct {
	Name         string
	RenewsOn     time.Time
	SeatLimit    int
	TeacherLimit int
	MediaBytes   int64
	ImportBytes  int64
}

// Held is what the centre keeps in storage and in the database, counted from its tables.
type Held struct {
	MediaBytes  int64
	ImportBytes int64
	ImportCount int
	Attempts    int64
}

// Credits is the month's AI credits used against the centre pool.
type Credits struct{ Used, Pool int64 }

// PlanInfo is the named plan; RenewsOn is nil when none is stated.
type PlanInfo struct {
	Name     string
	RenewsOn *time.Time
}

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

// ImportBytes is the import documents' size and number against a size limit, nil when there is none.
type ImportBytes struct {
	Bytes int64
	Count int
	Limit *int64
}

// Usage is what the centre uses against its plan. AICredits is nil without ai.credits.manage and while the assistant is off.
type Usage struct {
	Plan         *PlanInfo
	Seats        Count
	Teachers     Count
	Media        Bytes
	Imports      ImportBytes
	AttemptsKept int64
	AICredits    *Credits
}

// NewUsage reads the figures against the plan.
func NewUsage(plan Plan, accounts Accounts, held Held, credits *Credits) Usage {
	out := Usage{
		Seats:        Count{Used: accounts.Total, Limit: limitOf(plan.SeatLimit)},
		Teachers:     Count{Used: accounts.Teachers, Limit: limitOf(plan.TeacherLimit)},
		Media:        Bytes{Bytes: held.MediaBytes, Limit: bytesOf(plan.MediaBytes)},
		Imports:      ImportBytes{Bytes: held.ImportBytes, Count: held.ImportCount, Limit: bytesOf(plan.ImportBytes)},
		AttemptsKept: held.Attempts,
		AICredits:    credits,
	}
	if plan.Name != "" {
		out.Plan = &PlanInfo{Name: plan.Name}
		if !plan.RenewsOn.IsZero() {
			renews := plan.RenewsOn
			out.Plan.RenewsOn = &renews
		}
	}
	return out
}

func limitOf(n int) *int {
	if n < 1 {
		return nil
	}
	return &n
}

func bytesOf(n int64) *int64 {
	if n < 1 {
		return nil
	}
	return &n
}

// Percent is used as a share of limit, rounded half up.
func Percent(used, limit int64) int {
	return int((200*used + limit) / (2 * limit))
}

// StorageAttention is the storage row: how many of the media and import meters with a limit hold StorageWarnPercent or more of it, and nil when none does.
func StorageAttention(u Usage) *Attention {
	detail := AttentionDetail{}
	count := 0
	if u.Media.Limit != nil {
		percent := Percent(u.Media.Bytes, *u.Media.Limit)
		detail.MediaPercent = &percent
		if nearly(u.Media.Bytes, *u.Media.Limit) {
			count++
		}
	}
	if u.Imports.Limit != nil {
		percent := Percent(u.Imports.Bytes, *u.Imports.Limit)
		detail.ImportsPercent = &percent
		if nearly(u.Imports.Bytes, *u.Imports.Limit) {
			count++
		}
	}
	if count == 0 {
		return nil
	}
	return &Attention{Kind: AttentionStorage, Count: count, Detail: &detail}
}

func nearly(used, limit int64) bool { return used*100 >= limit*StorageWarnPercent }
