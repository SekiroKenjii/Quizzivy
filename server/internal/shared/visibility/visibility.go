// Package visibility holds the two reach rules every teaching read shares:
// which students and which assignments a teacher reaches. Each is one SQL
// subquery of ids, so a list, a count, a lock and a figure all apply the same
// rule; callers lift it under scope.all.
package visibility

import "fmt"

// StudentIDs is the one definition of the accounts a teacher reaches, as a
// subquery of user ids for the teacher's id in placeholder $n: members of a
// class the teacher teaches, accounts the teacher created, and individual
// targets of an assignment the teacher created. Callers add the student-like
// condition. A NULL teacher reaches no one.
func StudentIDs(n int) string {
	return fmt.Sprintf(`(SELECT m.user_id FROM app.classes c JOIN app.class_members m ON m.class_id = c.id WHERE c.teacher_id = $%[1]d::uuid
	 UNION ALL SELECT v.id FROM app.users v WHERE v.created_by = $%[1]d::uuid
	 UNION ALL SELECT s.user_id FROM app.assignments a JOIN app.assignment_students s ON s.assignment_id = a.id WHERE a.created_by = $%[1]d::uuid)`, n)
}

// AssignmentIDs is the one definition of the assignments a teacher reaches, as
// a subquery of assignment ids for the teacher's id in placeholder $n: those
// the teacher created, and those targeting a class the teacher teaches. A NULL
// teacher reaches none.
func AssignmentIDs(n int) string {
	return fmt.Sprintf(`(SELECT x.id FROM app.assignments x WHERE x.created_by = $%[1]d::uuid
	 UNION ALL SELECT ac.assignment_id FROM app.classes c JOIN app.assignment_classes ac ON ac.class_id = c.id WHERE c.teacher_id = $%[1]d::uuid)`, n)
}
