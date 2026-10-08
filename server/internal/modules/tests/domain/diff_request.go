package domain

import (
	"errors"
	"quizzivy/internal/shared/access"
	"strconv"
	"time"
)

// AgainstKind says which paper a version is compared with.
type AgainstKind string

const (
	AgainstPrevious AgainstKind = "previous"
	AgainstDraft    AgainstKind = "draft"
	AgainstVersion  AgainstKind = "version"
)

// ErrBadAgainst is a comparison target that is none of previous, draft or a
// version number, and ErrSameVersion one that names the version itself.
var (
	ErrBadAgainst  = errors.New("tests: the comparison target is not previous, draft or a version number")
	ErrSameVersion = errors.New("tests: a version cannot be compared with itself")
)

// ErrDraftUnreadable is a draft whose outline cannot be read back as a paper,
// because a group of it is refused. The error wrapping it names the cause.
var ErrDraftUnreadable = errors.New("tests: the draft cannot be read as a paper")

// Against names the paper a version is compared with: the draft, the version
// before it, or another version by number.
type Against struct {
	Kind    AgainstKind
	Version int
}

// ParseAgainst reads the value of the contract's against parameter for a
// comparison of version. It is ErrSameVersion for the number of version itself.
func ParseAgainst(raw string, version int) (Against, error) {
	switch raw {
	case string(AgainstPrevious):
		return Against{Kind: AgainstPrevious}, nil
	case string(AgainstDraft):
		return Against{Kind: AgainstDraft}, nil
	}
	number, err := strconv.Atoi(raw)
	if err != nil || number < 1 {
		return Against{}, ErrBadAgainst
	}
	if number == version {
		return Against{}, ErrSameVersion
	}
	return Against{Kind: AgainstVersion, Version: number}, nil
}

// DiffRequest asks for the two papers of a comparison. Version 0 means the
// test's latest version, which only the draft can be compared with.
type DiffRequest struct {
	TestID  string
	Version int
	Against Against
	Scope   access.Scope
}

// DiffSide identifies a paper of a comparison: the draft, or a version with
// the time it was published.
type DiffSide struct {
	Draft       bool
	Version     int
	PublishedAt time.Time
}

// DiffPaper is one side of a comparison with its content.
type DiffPaper struct {
	Side    DiffSide
	Content DraftContent
}

// DiffPapers is what a comparison reads. From is nil when nothing precedes
// To: the first version compared with the one before it.
type DiffPapers struct {
	From *DiffPaper
	To   DiffPaper
}

// Changes compares the two papers.
func (p DiffPapers) Changes() []Change {
	if p.From == nil {
		return Introduction(p.To.Content)
	}
	return Compare(p.From.Content, p.To.Content)
}
