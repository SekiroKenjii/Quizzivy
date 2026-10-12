import type { components } from "@/lib/api/schema";

/**
 * OneTimeCredential is a temporary password the server returned once, with
 * whose it is. It lives only in the state of the dialog that shows it: never
 * in a query cache, the URL, storage or a log.
 */
export type OneTimeCredential = Readonly<{
  studentId: string;
  fullName: string;
  email: string;
  temporaryPassword: string;
}>;

/** ResetFailure is a student a reset did not reach, named for the reader. */
export type ResetFailure = Readonly<{
  studentId: string;
  fullName: string;
  code: components["schemas"]["ErrorCode"];
}>;

const FAILURE_KEYS: Partial<Record<ResetFailure["code"], string>> = {
  NOT_FOUND: "students.reset.reason.notFound",
  FORBIDDEN: "students.reset.reason.forbidden",
  STUDENT_SHARED: "students.reset.reason.shared",
};

/**
 * failureKey is the message key for why a student's password was not reset:
 * not found, beyond the caller's permissions, shared with another teacher
 * (so an admin resets it), or anything else, which reads as a fault on our
 * side.
 */
export function failureKey(code: ResetFailure["code"]): string {
  return FAILURE_KEYS[code] ?? "students.reset.reason.internal";
}
