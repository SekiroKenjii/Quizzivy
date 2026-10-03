package domain

// Reference names what still holds a class whose permanent deletion was
// refused. Its values are the contract's ReferencedBy enum.
type Reference string

// ReferencedByAssignments and the other values are the reasons a refused
// deleteClass names in details.referencedBy.
const (
	ReferencedByAssignments Reference = "assignments"
	ReferencedByMembers     Reference = "members"
	ReferencedByOther       Reference = "other"
)

// ReferencedError is ErrReferenced naming what still holds the class;
// errors.Is(err, ErrReferenced) holds for it.
type ReferencedError struct{ By Reference }

func (e *ReferencedError) Error() string { return ErrReferenced.Error() + ": " + string(e.By) }

func (e *ReferencedError) Is(target error) bool { return target == ErrReferenced }
