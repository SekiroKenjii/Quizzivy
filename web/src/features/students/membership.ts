import { addMember, removeMember } from "@/features/classes/api";
import type { Student } from "@/features/students/api";

/**
 * moveStudent puts `student` in class `to` unless they are already there,
 * then, unless `keep`, takes them out of every other class in their row. It
 * adds before it removes, so a failure part way leaves the student in more
 * classes, never in none.
 */
export async function moveStudent(
  student: Student,
  to: string,
  keep: boolean,
): Promise<void> {
  if (!student.classes.some((klass) => klass.id === to))
    await addMember(to, student.id);
  if (keep) return;
  for (const klass of student.classes) {
    if (klass.id !== to) await removeMember(klass.id, student.id);
  }
}

/**
 * removeFromClasses takes `student` out of every class in their row. Their
 * account and results stay.
 */
export async function removeFromClasses(student: Student): Promise<void> {
  for (const klass of student.classes) await removeMember(klass.id, student.id);
}

/**
 * ClassesLeft is what a Remove or a Move takes students out of: how many
 * memberships end, and the names of their classes, each class once.
 */
export type ClassesLeft = { count: number; names: string };

/**
 * classesLeftBy is the memberships of `students` that end when they leave
 * every class in their rows except `stay`. A row lists the classes the caller
 * teaches, or every class for an Admin (`scope.all`), so the confirmations
 * name them.
 */
export function classesLeftBy(
  students: readonly Student[],
  stay?: string,
): ClassesLeft {
  const left = students
    .flatMap((student) => student.classes)
    .filter((klass) => klass.id !== stay);
  const names = new Map(left.map((klass) => [klass.id, klass.name]));
  return { count: left.length, names: [...names.values()].join(", ") };
}
