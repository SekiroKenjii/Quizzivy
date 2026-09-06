package validation

import "fmt"

// Field is one refused field and the message the client shows beside it.
type Field struct {
	Field   string
	Message string
}

// Error carries every refused field at once, so a form is corrected in one round.
type Error struct{ Fields []Field }

func (e *Error) Error() string {
	return fmt.Sprintf("validation failed on %d field(s)", len(e.Fields))
}

func (e *Error) Add(field, message string) {
	e.Fields = append(e.Fields, Field{Field: field, Message: message})
}

// OrNil is e when it holds anything, else nil, so a validator can build one and return it in one line.
func (e *Error) OrNil() *Error {
	if e == nil || len(e.Fields) == 0 {
		return nil
	}
	return e
}
