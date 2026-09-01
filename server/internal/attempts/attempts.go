// Package attempts runs §9's take-test engine server-side: what a student is
// allowed to start, what they are shown, and when their time is up. Every one
// of those is decided here rather than by the client, because the client is the
// party with an incentive.
package attempts

import (
	"errors"
	"time"
)

type Status string

const (
	InProgress Status = "in_progress"
	Submitted  Status = "submitted"
	TimedOut   Status = "timed_out"
	Graded     Status = "graded"
	Voided     Status = "voided"
)

// Event kinds this package writes itself. The rest of §10.1's list arrives from
// the client and is never enumerated in Go -- see 00023 for why kind is not an
// enum.
const (
	KindResume          = "resume"
	KindSessionTakeover = "session_takeover"
)

// sessionLiveWindow decides whether a resume supersedes a tab that was still
// open (`session_takeover`) or merely re-enters one that had gone -- a reload,
// a crash, a closed laptop -- which is `resume` alone.
//
// There is no heartbeat to ask, so recency of the session's last CLIENT write
// stands in for liveness. Two minutes is several autosave debounces: long
// enough that a brief network stall does not read as a departed tab, short
// enough that reopening a test an hour later is not reported to the teacher as
// a second device.
const sessionLiveWindow = 2 * time.Minute

var (
	ErrNotFound = errors.New("attempts: not found")
	// ErrForbidden covers both "not your attempt" and "not assigned to you".
	// They are one answer to the caller and deliberately indistinguishable to
	// the student: which assignments exist is not theirs to enumerate.
	ErrForbidden        = errors.New("attempts: not yours")
	ErrAssignmentClosed = errors.New("attempts: assignment is not open")
	ErrLimitReached     = errors.New("attempts: attempt limit reached")

	// ErrSessionSuperseded is how the tab that lost finds out. It learns on its
	// next write rather than being told at the moment it happened, because
	// there is nothing pushing to it -- §10.1, E2E 7.
	ErrSessionSuperseded = errors.New("attempts: session superseded")
	ErrDeadlinePassed    = errors.New("attempts: deadline passed")
	ErrAttemptClosed     = errors.New("attempts: attempt is no longer in progress")

	// ErrBeaconExpired is separate from ErrForbidden so the difference between
	// a wrong token and a spent one stays legible here. Both answer 403 on the
	// wire -- telling a caller which one it was is telling it whether the token
	// it holds is real.
	ErrBeaconExpired = errors.New("attempts: beacon token has expired")
)

// Answer is one saved answer, kept as the raw JSON the contract defines.
//
// The server has no reason to understand a choice from a text answer until
// grading, and decoding it here would be a second place for the shape to drift
// from api/openapi.yaml. The column is jsonb, so Postgres validates that it is
// an object and the CHECK on the table enforces the rest.
type Answer struct {
	QuestionID string
	Payload    []byte
}

// Event is one client-sourced integrity event (§10.1). Kind is deliberately not
// an enum here for the same reason it is not one in the column.
type Event struct {
	Kind       string
	OccurredAt time.Time
	ClientSeq  int
	QuestionID *string
	Meta       []byte
}

type SaveInput struct {
	AttemptID string
	StudentID string
	SessionID string
	Answers   []Answer
	Events    []Event
}

// SaveResult reports what actually landed. Saved can be lower than the number
// of answers submitted -- see Store.Save for why that is not an error.
type SaveResult struct {
	SavedAt time.Time
	Saved   int
}

type Integrity struct {
	RequireFullscreen bool
	BlockCopyPaste    bool
	MaxFocusLoss      int
	OnLimitExceeded   string
	MinAwayMs         int
}

// AudioPolicy is §11.4's per-question rules. A nil MaxPlays is unlimited.
type AudioPolicy struct {
	MaxPlays                  *int
	AllowSeek                 bool
	ShowTranscriptAfterSubmit bool
}

// Media is the asset metadata a paper carries. The signed URL is not here:
// it expires, so it is minted per response by the media service rather than
// read alongside rows that do not (§11.2).
type Media struct {
	ID         string
	Kind       string
	MimeType   string
	Filename   string
	Bytes      int
	DurationMs *int
	CreatedAt  time.Time
}

// Option carries no IsCorrect, and Blank no accepted answers: these types are
// the projection a student receives, and the surest way not to leak a grading
// key is to have nowhere to put one (§13.5).
type Option struct {
	ID   string
	Text string
}

type Blank struct {
	ID      string
	Ordinal int
}

type Question struct {
	ID      string
	Type    string
	Prompt  string
	Points  float64
	Media   *Media
	Audio   *AudioPolicy
	Options []Option
	Blanks  []Blank
}

type Attempt struct {
	ID             string
	AssignmentID   string
	StudentID      string
	TestVersionID  string
	AttemptNo      int
	Status         Status
	StartedAt      time.Time
	DeadlineAt     time.Time
	SubmittedAt    *time.Time
	GradedAt       *time.Time
	FocusLossCount int
	Flagged        bool
}

// Session is everything the engine needs to run authoritatively: the attempt,
// the paper in presentation order, and the identity of this tab.
type Session struct {
	Attempt     Attempt
	Questions   []Question
	SessionID   string
	BeaconToken string
	ServerTime  time.Time
	AudioPlays  map[string]int
	Answers     map[string][]byte
	Integrity   Integrity
}
