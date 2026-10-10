package domain

import "time"

// Extension is who a teacher's extension of an assignment's close reached,
// and the close each of them now has: the title the notification names, and
// the students it is for.
type Extension struct {
	Title    string
	Students []StudentClose
}

// StudentClose is one student's close as it now stands, their override and
// an early close included.
type StudentClose struct {
	StudentID string
	ClosesAt  time.Time
}
