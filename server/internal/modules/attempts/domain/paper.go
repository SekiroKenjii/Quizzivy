package domain

import (
	"time"
)

type Question struct {
	ID        string
	SectionID string
	Type      string
	Prompt    string
	Points    float64
	Media     *Media
	Audio     *AudioPolicy
	Options   []Option
	Blanks    []Blank
}

// Section is one part of the paper in test order; Instructions is the
// teacher's text to the student for that part, nil when they wrote none.
type Section struct {
	ID           string
	Title        string
	Instructions *string
}

// Option carries no IsCorrect, and Blank no accepted answers: these types are
// the projection a student receives, and the surest way not to leak a grading
// key is to have nowhere to put one (§13.5). CaseSensitive stays because it is
// the rule the student is graded by, not the key.
type Option struct {
	ID   string
	Text string
}

type Blank struct {
	ID            string
	Ordinal       int
	CaseSensitive bool
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

// AudioPolicy is §11.4's per-question rules. A nil MaxPlays is unlimited.
type AudioPolicy struct {
	MaxPlays                  *int
	AllowSeek                 bool
	ShowTranscriptAfterSubmit bool
}

type Integrity struct {
	RequireFullscreen bool
	BlockCopyPaste    bool
	MaxFocusLoss      int
	OnLimitExceeded   string
	MinAwayMs         int
}

// Answer is one saved answer, kept as the raw JSON the contract defines.
type Answer struct {
	QuestionID string
	Payload    []byte
}

// BlankAnswer is one canonical accepted answer, never the full list.
type BlankAnswer struct {
	BlankID string
	Answer  string
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

// Plays is what the client renders "còn N lượt nghe" from.
type Plays struct {
	Plays    int
	MaxPlays *int
}
