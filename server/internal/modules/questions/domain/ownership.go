package domain

// GroupOwnership makes a question an ordered, exclusively owned member of a shared-context group.
// CopiedFromID, when set on a create, names the bank question the member is a copy of; an update never changes it.
type GroupOwnership struct {
	GroupID      string
	Ordinal      int
	OptionOrder  string
	CopiedFromID string
}

// OwnedQuestion is a resolved group member, including its authored ordering policy.
type OwnedQuestion struct {
	Question    Question
	Ordinal     int
	OptionOrder string
}
