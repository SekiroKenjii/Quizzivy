package domain

// GroupOwnership makes a question an ordered, exclusively owned member of a shared-context group.
type GroupOwnership struct {
	GroupID     string
	Ordinal     int
	OptionOrder string
}

// OwnedQuestion is a resolved group member, including its authored ordering policy.
type OwnedQuestion struct {
	Question    Question
	Ordinal     int
	OptionOrder string
}
