package domain

import (
	"encoding/json"
	"time"
	"unicode/utf8"
)

const (
	maxTitle      = 300
	maxName       = 200
	maxClassNames = 5
)

// Params is what one kind's sentence is built from: plain values under the
// contract's NotificationParams names, never markup, never a score, a band or
// an answer. Only this package's types implement it, one for each Kind, so a
// producer cannot store a field the contract does not have.
type Params interface {
	Kind() Kind
	fields() fields
}

type fields struct {
	Title        *string    `json:"title,omitempty"`
	Count        *int       `json:"count,omitempty"`
	ToGrade      *int       `json:"toGrade,omitempty"`
	FocusLost    *int       `json:"focusLost,omitempty"`
	NotSubmitted *int       `json:"notSubmitted,omitempty"`
	StudentName  *string    `json:"studentName,omitempty"`
	ClassName    *string    `json:"className,omitempty"`
	ClassNames   *[]string  `json:"classNames,omitempty"`
	ClosesAt     *time.Time `json:"closesAt,omitempty"`
}

// Encode returns p as the object the store keeps and the reader is sent. It
// answers ErrInvalidParams when a value is outside the bounds the contract
// gives its field: an empty or overlong title or name, a negative count, no
// class name or more than five, a count below the number of names, or a zero
// time.
func Encode(p Params) ([]byte, error) {
	f := p.fields()
	if !f.valid() {
		return nil, ErrInvalidParams
	}
	return json.Marshal(f)
}

func (f fields) valid() bool {
	return within(f.Title, maxTitle) && within(f.StudentName, maxName) && within(f.ClassName, maxName) &&
		counted(f.Count) && counted(f.ToGrade) && counted(f.FocusLost) && counted(f.NotSubmitted) &&
		named(f.ClassNames, f.Count) && (f.ClosesAt == nil || !f.ClosesAt.IsZero())
}

func named(names *[]string, count *int) bool {
	if names == nil {
		return true
	}
	if len(*names) < 1 || len(*names) > maxClassNames || count != nil && *count < len(*names) {
		return false
	}
	for _, name := range *names {
		if !within(&name, maxName) {
			return false
		}
	}
	return true
}

func within(text *string, limit int) bool {
	if text == nil {
		return true
	}
	n := utf8.RuneCountInString(*text)
	return n >= 1 && n <= limit
}

func counted(n *int) bool {
	return n == nil || *n >= 0
}

func moment(at time.Time) *time.Time {
	utc := at.UTC()
	return &utc
}

// Submitted is AttemptSubmitted's params: the test's title, how many papers
// were handed in on the assignment, and how many of them wait for a mark.
type Submitted struct {
	Title   string
	Count   int
	ToGrade int
}

func (Submitted) Kind() Kind { return AttemptSubmitted }

func (p Submitted) fields() fields {
	return fields{Title: &p.Title, Count: &p.Count, ToGrade: &p.ToGrade}
}

// Flagged is AttemptFlagged's params: who was flagged, on which test, and
// how many times they left it.
type Flagged struct {
	StudentName string
	Title       string
	FocusLost   int
}

func (Flagged) Kind() Kind { return AttemptFlagged }

func (p Flagged) fields() fields {
	return fields{StudentName: &p.StudentName, Title: &p.Title, FocusLost: &p.FocusLost}
}

// Closing is AssignmentClosing's params: the test's title and how many
// students have not handed in.
type Closing struct {
	Title        string
	NotSubmitted int
}

func (Closing) Kind() Kind { return AssignmentClosing }

func (p Closing) fields() fields {
	return fields{Title: &p.Title, NotSubmitted: &p.NotSubmitted}
}

// Joined is ClassJoined's params: who joined which class.
type Joined struct {
	StudentName string
	ClassName   string
}

func (Joined) Kind() Kind { return ClassJoined }

func (p Joined) fields() fields {
	return fields{StudentName: &p.StudentName, ClassName: &p.ClassName}
}

// CodesRotated is JoinCodesRotated's params: how many classes had their join
// code replaced, and the names of at least one and at most five of them.
type CodesRotated struct {
	Count      int
	ClassNames []string
}

func (CodesRotated) Kind() Kind { return JoinCodesRotated }

func (p CodesRotated) fields() fields {
	return fields{Count: &p.Count, ClassNames: &p.ClassNames}
}

// Opened is AssignmentOpened's params: the test's title and when the
// assignment closes.
type Opened struct {
	Title    string
	ClosesAt time.Time
}

func (Opened) Kind() Kind { return AssignmentOpened }

func (p Opened) fields() fields {
	return fields{Title: &p.Title, ClosesAt: moment(p.ClosesAt)}
}

// DueSoon is AssignmentDueSoon's params: the test's title and when the
// assignment closes.
type DueSoon struct {
	Title    string
	ClosesAt time.Time
}

func (DueSoon) Kind() Kind { return AssignmentDueSoon }

func (p DueSoon) fields() fields {
	return fields{Title: &p.Title, ClosesAt: moment(p.ClosesAt)}
}

// Extended is AssignmentExtended's params: the test's title and the time the
// assignment now closes.
type Extended struct {
	Title    string
	ClosesAt time.Time
}

func (Extended) Kind() Kind { return AssignmentExtended }

func (p Extended) fields() fields {
	return fields{Title: &p.Title, ClosesAt: moment(p.ClosesAt)}
}

// Ready is ResultReady's params: the test's title and nothing else. The
// reader's console shows the score from the result itself, under the review
// policy in force when it is read.
type Ready struct {
	Title string
}

func (Ready) Kind() Kind { return ResultReady }

func (p Ready) fields() fields {
	return fields{Title: &p.Title}
}
