package domain

import "time"

// RequestStatus is where a request for more credits stands.
type RequestStatus string

// The three states of a request.
const (
	RequestPending  RequestStatus = "pending"
	RequestApproved RequestStatus = "approved"
	RequestDeclined RequestStatus = "declined"
)

// Request is a teacher's ask for more credits in a month. DecidedBy is empty when the console decided it.
type Request struct {
	ID        string
	UserID    string
	Month     time.Time
	Amount    int64
	Reason    string
	Status    RequestStatus
	DecidedBy string
	DecidedAt *time.Time
	CreatedAt time.Time
}

// PendingRequest is a request waiting for an Admin, with the standing of the teacher who made it.
type PendingRequest struct {
	Request
	TeacherName string
	Used        int64
	Allowance   int64
}

// CreateRequestInput asks to record Amount credits wanted by UserID in Month.
type CreateRequestInput struct {
	Month  time.Time
	UserID string
	Amount int64
	Reason string
}

// Filed is a request just recorded, with the people to tell: every user who holds ai.credits.manage.
type Filed struct {
	Request     Request
	TeacherName string
	Managers    []string
}

// DecideInput approves or declines a request. Month is the current month: an approval of an earlier month's request is refused.
type DecideInput struct {
	By      Author
	ID      string
	Month   time.Time
	Approve bool
}

// Decided is a request after a decision. Allowance is the teacher's allowance for the month after an approval, and zero after a decline.
type Decided struct {
	Request     Request
	TeacherName string
	Allowance   int64
}

// SetAllowanceInput changes how much one teacher may use in Month. A nil field keeps what is stored. Credits is the teacher's own allowance, before the month's grants, and is read only when Custom is true.
type SetAllowanceInput struct {
	By      Author
	Month   time.Time
	UserID  string
	Custom  *bool
	Credits int64
	Paused  *bool
}

// AddCreditsInput puts Credits into Month's pool.
type AddCreditsInput struct {
	By      Author
	Month   time.Time
	Credits int64
}

// SetRulesInput changes the centre's rules; a nil field keeps what is stored. Month is the current month, whose non-custom allowances are rewritten when the default changes.
type SetRulesInput struct {
	By               Author
	Month            time.Time
	DefaultAllowance *int64
	WarnPercent      *int
	OnTeacherLimit   *Limit
}

// RulesChange is the centre's rules after a change, and how many allowances the new default rewrote.
type RulesChange struct {
	Rules     Rules
	Rewritten int64
}

// CategorySum is the credits a teacher used in one category of features.
type CategorySum struct {
	Category Category
	Credits  int64
}

// MonthUsage is what one teacher sees of their month: Left is the least of what remains of the allowance and of the pool, never below zero. Requests says whether the centre takes requests for more, and Pending is the teacher's undecided request this month.
type MonthUsage struct {
	Month      time.Time
	Used       int64
	Allowance  int64
	Left       int64
	Paused     bool
	Requests   bool
	Pending    *Request
	Categories []CategorySum
}

// AdminCredits is the Admin's view of a month. OnPace is the credits the month is on course to use; WithinPool says it fits the pool. Allowances is the sum of the teachers' allowances, which may exceed the pool. Idle counts the teachers who hold ai.use and have used nothing.
type AdminCredits struct {
	Month      time.Time
	Pool       Pool
	DaysToGo   int
	OnPace     int64
	WithinPool bool
	Allowances int64
	Active     int
	Idle       int
	Teachers   []TeacherMonth
	Requests   []PendingRequest
	Rules      Rules
}

// Changed returns r with the rules in in changed and the others kept.
func (r Rules) Changed(in SetRulesInput) Rules {
	if in.DefaultAllowance != nil {
		r.DefaultAllowance = *in.DefaultAllowance
	}
	if in.WarnPercent != nil {
		r.WarnPercent = *in.WarnPercent
	}
	if in.OnTeacherLimit != nil {
		r.OnTeacherLimit = *in.OnTeacherLimit
	}
	return r
}
