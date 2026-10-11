package domain

import "quizzivy/internal/shared/content"

// MaxClassName and MaxMemberName are the most characters a class name and a member's full name hold, as the contract and the
// classes and users tables bound them.
const (
	MaxClassName  = 120
	MaxMemberName = 200
)

// Composed returns the change with the name and description it carries composed to NFC. A name that composing leaves over
// MaxClassName comes back as a *validation.Error naming the field.
func (in UpdateInput) Composed() (UpdateInput, error) {
	var c content.Composer
	out := in
	out.Name = c.Optional("name", in.Name, MaxClassName)
	out.Description = c.Optional("description", in.Description, 0)
	if err := c.Err(); err != nil {
		return in, err
	}
	return out, nil
}
