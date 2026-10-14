package domain

import (
	"quizzivy/internal/shared/access"
	"time"
)

// Rules a refused move names in GroupError.Rule.
const (
	// RuleGroupGapBound refuses a member that a gap of its group's shared material is bound to.
	RuleGroupGapBound = "group_gap_bound"
	// RuleGroupLimits refuses a destination group or section that is already full.
	RuleGroupLimits = groupLimits
	// RuleMoveOutlineOnly refuses a move between two standalone places, which the outline writer owns.
	RuleMoveOutlineOnly = "move_outline_only"
	// RuleMoveGroupSection refuses a destination group that the destination section does not own.
	RuleMoveGroupSection = "move_group_section"
	// RuleMoveRevisions refuses an expected-revisions list that repeats a group or names one the move does not touch.
	RuleMoveRevisions = "move_revisions"
	// RuleMoveSourceAmbiguous refuses a standalone question that more than one section of the test holds.
	RuleMoveSourceAmbiguous = "move_source_ambiguous"
)

// MoveTarget is where a moved question lands: a unit of SectionID when
// GroupID is nil, otherwise a member of that group, which the section must
// own. Index counts positions once the question has left its old place, and
// a position past the end is the end.
type MoveTarget struct {
	SectionID string
	GroupID   *string
	Index     int
}

// GroupRevision is the revision of one group that a move observed.
type GroupRevision struct {
	GroupID  string
	Revision int64
}

// MoveQuestionInput moves one question of a test's draft across a group's
// boundary, between two groups, or within one group, under the test revision
// and the revision of every group the move touches. ExpectedGroups names
// each touched group once and no other. A test or id outside Scope is
// ErrNotFound, and Grants must hold the key RequireGroupWrite names for a
// section group.
type MoveQuestionInput struct {
	TestID                string
	QuestionID            string
	ExpectedTestUpdatedAt time.Time
	To                    MoveTarget
	ExpectedGroups        []GroupRevision
	ActorID               string
	Now                   time.Time
	IP                    string
	UserAgent             string
	Scope                 access.Scope
	Grants                access.Set
}

// MoveQuestionResult is the saved draft and every touched group at its new
// revision, source group first. QuestionID is the id the question has now: a
// new one after a copy into a group, the original bank question's after
// Restored, otherwise the moved question's own. Restored is a member that was
// an unedited copy going back as its original, the copy removed. EditedCopy is
// a member that was an edited copy of a bank question still in the bank
// leaving as a bank question of its own.
type MoveQuestionResult struct {
	Test       Test
	QuestionID string
	Copied     bool
	Restored   bool
	EditedCopy bool
	Groups     []StoredGroup
}
