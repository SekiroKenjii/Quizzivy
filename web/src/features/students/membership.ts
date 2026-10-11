import { addMember, removeMember } from "@/features/classes/api";
import type { Student } from "@/features/students/api";

/**
 * moveStudent puts `student` in class `to` unless they are already there,
 * then, unless `keep`, takes them out of every other class of theirs the
 * caller teaches. It adds before it removes, so a failure part way leaves
 * the student in more classes, never in none.
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
 * removeFromClasses takes `student` out of every class of theirs the caller
 * teaches. Their account and results stay.
 */
export async function removeFromClasses(student: Student): Promise<void> {
  for (const klass of student.classes) await removeMember(klass.id, student.id);
}
