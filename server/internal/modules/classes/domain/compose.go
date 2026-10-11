package domain

import "quizzivy/internal/shared/content"

// MaxClassName, MaxScheduleLabel, MaxRoom and MaxMemberName are the most characters a class name, a schedule label, a room
// and a member's full name hold, as the contract and the classes and users tables bound them.
const (
	MaxClassName     = 120
	MaxScheduleLabel = 120
	MaxRoom          = 60
	MaxMemberName    = 200
)

// Composed returns the change with the name, description, schedule label and room it carries composed to NFC. A value that
// composing leaves over its limit comes back as a *validation.Error naming the field.
func (in UpdateInput) Composed() (UpdateInput, error) {
	var c content.Composer
	out := in
	out.Name = c.Optional("name", in.Name, MaxClassName)
	out.Description = c.Optional("description", in.Description, 0)
	out.ScheduleLabel.Value = c.Optional("scheduleLabel", in.ScheduleLabel.Value, MaxScheduleLabel)
	out.Room.Value = c.Optional("room", in.Room.Value, MaxRoom)
	if err := c.Err(); err != nil {
		return in, err
	}
	return out, nil
}
