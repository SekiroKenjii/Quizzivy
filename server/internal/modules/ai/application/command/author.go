package command

import (
	"errors"

	"quizzivy/internal/modules/ai/domain"
)

func checkAuthor(by domain.Author) error {
	if by.System() && by.Operator == "" {
		return errors.New("ai: a command run from the console names its operator")
	}
	return nil
}
