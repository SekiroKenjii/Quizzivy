package ports

import (
	"context"

	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/shared/access"
)

// Tests reads a test's draft through the tests module. Another teacher's test answers domain.ErrNotFound, as a missing one does.
type Tests interface {
	// Passage reads the shared material named stimulusID in the draft of testID.
	Passage(ctx context.Context, scope access.Scope, testID, stimulusID string) (domain.Passage, error)
	// Draft reads the draft's questions in the order the builder shows them, standalone questions and the members of its groups alike, each member carrying its group's id and passage. It skips a question the scope does not reach but keeps its number.
	Draft(ctx context.Context, scope access.Scope, testID string) (domain.Paper, error)
}

// Questions reads the bank through the questions module.
type Questions interface {
	// Get reads one stored question the scope reaches.
	Get(ctx context.Context, scope access.Scope, id string) (domain.Question, error)
	// Own reads up to limit of the questions the scope's user owns, newest first, whatever the scope widens to.
	Own(ctx context.Context, scope access.Scope, limit int) (domain.Bank, error)
}

// Grading reads one student answer through the attempts module.
type Grading interface {
	// Answer reads the answer to questionID on attemptID, under the scope the grading reads use. It answers domain.ErrNotFound for an attempt out of reach or a question that is not on its paper, domain.ErrAttemptInProgress and domain.ErrAttemptVoided for a paper that cannot be marked, and an empty Answer for a question left blank.
	Answer(ctx context.Context, scope access.Scope, attemptID, questionID string) (domain.GradedAnswer, error)
}

// Analysis reads how an assignment's papers did through the attempts module.
type Analysis interface {
	// Results reads the assignment under the scope the grading reads use.
	Results(ctx context.Context, scope access.Scope, assignmentID string) (domain.Results, error)
}

// People reads what the assistant needs to know of a user through the identity and access modules.
type People interface {
	// Preferences reads the user's AI preferences, with the defaults of an omitted key: the language of their interface and reading student answers on.
	Preferences(ctx context.Context, userID string) (domain.Preferences, error)
	// Permissions reads the user's effective permissions, and answers domain.ErrNotFound for an id that names no user.
	Permissions(ctx context.Context, userID string) (access.Set, error)
}
