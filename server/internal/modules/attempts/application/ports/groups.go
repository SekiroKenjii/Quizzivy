package ports

import (
	testsquery "quizzivy/internal/modules/tests/application/query"
	testsdomain "quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/cqrs"
)

// GroupContexts reads safe frozen context after the attempt has authorized its version.
type GroupContexts = cqrs.QueryHandler[testsquery.GroupContexts, []testsdomain.PreviewGroup]
