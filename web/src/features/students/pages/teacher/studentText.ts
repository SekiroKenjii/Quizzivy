import type { TFunction } from "i18next";
import {
  scorePercent,
  type AccountStatus,
  type Student,
} from "@/features/students/api";
import type { Locale } from "@/lib/i18n";
import { formatRelative } from "@/lib/i18n/datetime";

/** StudentAccount is how the list names an account's state. */
export type StudentAccount = "active" | "notSignedIn" | "reset" | "suspended";

const ACCOUNT_OF: Record<AccountStatus, StudentAccount> = {
  active: "active",
  never_signed_in: "notSignedIn",
  must_change_password: "reset",
  disabled: "suspended",
};

const STATUS_OF: Record<StudentAccount, AccountStatus> = {
  active: "active",
  notSignedIn: "never_signed_in",
  reset: "must_change_password",
  suspended: "disabled",
};

/** accountOf is the list's name for a student's account state, which the server decides (DG-54): suspended, then on a temporary password, then not signed in yet, then active. */
export function accountOf(student: Student): StudentAccount {
  return ACCOUNT_OF[student.accountStatus];
}

/** statusOf is the server's name for a state the Status facet offers. */
export function statusOf(account: StudentAccount): AccountStatus {
  return STATUS_OF[account];
}

/** averageOf is a student's average as the list prints it, "—" before anything is graded. */
export function averageOf(student: Student, t: TFunction): string {
  const percent = scorePercent(student.stats);
  return percent === null ? "—" : t("students.percent", { value: percent });
}

/**
 * lastActiveOf is when a student last made an authenticated request, "taking
 * a test" while they are, and "—" before they ever have (DG-54).
 */
export function lastActiveOf(student: Student, locale: Locale, t: TFunction): string {
  if (student.stats.activity.live) return t("students.takingNow");
  const at = student.lastActiveAt;
  return at ? formatRelative(at, locale) : "—";
}

/** classLineOf is a student's classes on one line, "No class yet" for none. */
export function classLineOf(student: Student, t: TFunction): string {
  return student.classes.length === 0
    ? t("students.noClassYet")
    : student.classes.map((klass) => klass.name).join(", ");
}
