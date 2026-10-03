// Package visibility holds the reach rules every teaching read shares: which
// classes, students, assignments and papers a teacher reaches. Each is one SQL
// fragment, so a list, a count, a lock and a figure all apply the same rule;
// callers lift it under scope.all.
package visibility

import "fmt"

func taught(n int) string {
	return fmt.Sprintf(`SELECT c.id FROM app.classes c WHERE c.teacher_id = $%d::uuid`, n)
}

func members(n int) string {
	return `SELECT m.user_id FROM app.class_members m WHERE m.class_id IN ` + TaughtClassIDs(n)
}

func authored(n int) string {
	return fmt.Sprintf(`SELECT x.id FROM app.assignments x WHERE x.created_by = $%d::uuid`, n)
}

// TaughtClassIDs is the one definition of the classes a teacher teaches, as a
// subquery of class ids for the teacher's id in placeholder $n. A NULL teacher
// teaches none.
func TaughtClassIDs(n int) string {
	return `(` + taught(n) + `)`
}

// AuthoredAssignmentIDs is the one definition of the assignments a teacher
// created, as a subquery of assignment ids for the teacher's id in placeholder
// $n. A NULL teacher created none.
func AuthoredAssignmentIDs(n int) string {
	return `(` + authored(n) + `)`
}

// TaughtMemberIDs is the one definition of the accounts in a teacher's
// classes, as a subquery of user ids for the teacher's id in placeholder $n:
// the members of every class the teacher teaches, archived ones included.
// Callers add the student-like condition. A NULL teacher has none.
func TaughtMemberIDs(n int) string {
	return `(` + members(n) + `)`
}

// StudentIDs is the one definition of the accounts a teacher reaches, as a
// subquery of user ids for the teacher's id in placeholder $n: members of a
// class the teacher teaches, accounts the teacher created, and individual
// targets of an assignment the teacher created. Callers add the student-like
// condition. A NULL teacher reaches no one.
func StudentIDs(n int) string {
	return `(` + members(n) + fmt.Sprintf(`
	 UNION ALL SELECT v.id FROM app.users v WHERE v.created_by = $%d::uuid`, n) + `
	 UNION ALL SELECT s.user_id FROM app.assignment_students s WHERE s.assignment_id IN ` + AuthoredAssignmentIDs(n) + `)`
}

// AssignmentIDs is the one definition of the assignments a teacher reaches, as
// a subquery of assignment ids for the teacher's id in placeholder $n: those
// the teacher created, and those targeting a class the teacher teaches. A NULL
// teacher reaches none.
func AssignmentIDs(n int) string {
	return `(` + authored(n) + `
	 UNION ALL SELECT ac.assignment_id FROM app.assignment_classes ac WHERE ac.class_id IN ` + TaughtClassIDs(n) + `)`
}

// Papers is the one definition of which attempts a list of papers shows, as a
// condition on the attempt's assignment and student columns, with scope.all
// in placeholder $all and the teacher's id in $viewer: every attempt on an
// assignment the teacher created, a student removed from the class after
// sitting it included, and otherwise only those of students the teacher
// reaches. The caller gates the assignment itself with AssignmentIDs.
func Papers(all, viewer int, assignment, student string) string {
	return fmt.Sprintf(`($%d::boolean OR %s IN %s OR %s IN %s)`, all, assignment, AuthoredAssignmentIDs(viewer), student, StudentIDs(viewer))
}
