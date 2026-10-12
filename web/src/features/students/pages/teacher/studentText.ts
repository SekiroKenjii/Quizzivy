import type { TFunction } from "i18next";
import { scorePercent, type Student } from "@/features/students/api";
import type { Locale } from "@/lib/i18n";
import { formatRelative } from "@/lib/i18n/datetime";

/** StudentAccount is how the list names an account's state. */
export type StudentAccount = "active" | "reset" | "suspended";

/** accountOf is a student's account state: suspended, on a temporary password, or active. */
export function accountOf(student: Student): StudentAccount {
  if (student.disabledAt !== null) return "suspended";
  return student.mustChangePassword ? "reset" : "active";
}

/** averageOf is a student's average as the list prints it, "—" before anything is graded. */
export function averageOf(student: Student, t: TFunction): string {
  const percent = scorePercent(student.stats);
  return percent === null ? "—" : t("students.percent", { value: percent });
}

/**
 * lastActiveOf is when a student last handed in, "taking a test" while they
 * are, and "—" before they ever have: the last test until R5 records sign-in
 * activity.
 */
export function lastActiveOf(student: Student, locale: Locale, t: TFunction): string {
  if (student.stats.activity.live) return t("students.takingNow");
  const at = student.stats.activity.lastAttemptAt;
  return at ? formatRelative(at, locale) : "—";
}

/** classLineOf is a student's classes on one line, "No class yet" for none. */
export function classLineOf(student: Student, t: TFunction): string {
  return student.classes.length === 0
    ? t("students.noClassYet")
    : student.classes.map((klass) => klass.name).join(", ");
}
