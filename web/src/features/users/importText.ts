import type { TFunction } from "i18next";
import { ApiError } from "@/lib/api/errors";
import type { UserImportIssue, UserImportJob } from "@/features/users/api";

/** IMPORT_MAX_BYTES is the largest file the import takes, 5 MiB, the server's limit. */
export const IMPORT_MAX_BYTES = 5 * 1024 * 1024;

/** IMPORT_ACCEPT is what the file picker offers; the server reads the format from the bytes. */
export const IMPORT_ACCEPT =
  ".csv,.xlsx,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet";

/** IMPORT_ISSUES_SHOWN is how many refused cells the preview lists at most. */
export const IMPORT_ISSUES_SHOWN = 50;

type IssueCode = UserImportIssue["code"];

const ISSUE_KEYS: Record<IssueCode, string> = {
  REQUIRED: "users.import.reason.required",
  TOO_LONG: "users.import.reason.tooLong",
  INVALID: "users.import.reason.invalid",
  FORMULA_LIKE: "users.import.reason.formulaLike",
  DUPLICATE_IN_FILE: "users.import.reason.duplicateInFile",
  UNKNOWN_ROLE: "users.import.reason.unknownRole",
  AMBIGUOUS_ROLE: "users.import.reason.ambiguousRole",
  ROLE_NOT_ALLOWED: "users.import.reason.roleNotAllowed",
  ROLE_NOT_GIVABLE: "users.import.reason.roleNotGivable",
  UNKNOWN_CLASS: "users.import.reason.unknownClass",
  AMBIGUOUS_CLASS: "users.import.reason.ambiguousClass",
  CLASS_NOT_ALLOWED: "users.import.reason.classNotAllowed",
  OVER_SEAT_LIMIT: "users.import.reason.overSeatLimit",
  OVER_TEACHER_LIMIT: "users.import.reason.overTeacherLimit",
};

/** ISSUE_CODES are the reasons a row can be refused, in the contract's order. */
export const ISSUE_CODES = Object.keys(ISSUE_KEYS) as readonly IssueCode[];

/**
 * issueReason says why a cell was refused, in the language of the page. A code
 * this build does not know, from a server one release ahead, reads as "not
 * valid" so the row still draws.
 */
export function issueReason(code: string, t: TFunction): string {
  const key = (ISSUE_KEYS as Record<string, string | undefined>)[code];
  return t(key ?? ISSUE_KEYS.INVALID);
}

const FAILURE_KEYS: Record<string, string> = {
  IMPORTER_NOT_ALLOWED: "users.import.failed.importerNotAllowed",
  DELIVERY_UNAVAILABLE: "users.import.failed.deliveryUnavailable",
  WORKER_INTERRUPTED: "users.import.failed.workerInterrupted",
};

/** failureReason says why a job gave up; a code this build does not know reads as a plain stop. */
export function failureReason(job: Pick<UserImportJob, "errorCode">, t: TFunction) {
  const key = job.errorCode === null ? undefined : FAILURE_KEYS[job.errorCode];
  return t(key ?? "users.import.failed.unknown");
}

const TOO_LARGE_KEY = "users.import.refusal.tooLarge";

const REFUSAL_KEYS = new Map<string, string>([
  ["USER_IMPORT_FILE_TOO_LARGE", TOO_LARGE_KEY],
  ["USER_IMPORT_FILE_EMPTY", "users.import.refusal.empty"],
  ["USER_IMPORT_TOO_MANY_ROWS", "users.import.refusal.tooManyRows"],
  ["USER_IMPORT_NOTHING_TO_CREATE", "users.import.refusal.nothingToCreate"],
  ["CSV_NOT_UTF8", "users.import.refusal.csvNotUtf8"],
  ["XLSX_MACROS", "users.import.refusal.macros"],
  ["XLSX_ENCRYPTED", "users.import.refusal.encrypted"],
  ["PASSWORD_SIGN_IN_DISABLED", "users.import.refusal.passwordSignInDisabled"],
  ["TARGET_ROLE_NOT_ALLOWED", "users.import.refusal.roleNotAllowed"],
]);

/**
 * refusalMessage says why the server would not read or take a file. The
 * refusals a file can be mended for have a message of the page's own, in
 * both languages, and the one for a file over 5 MB (`413`, whoever sends it)
 * names the way out, CSV UTF-8 (D-013). The header and the unreadable
 * workbook carry the server's sentence, which names the column or the part at
 * fault.
 */
export function refusalMessage(cause: unknown, t: TFunction): string {
  if (!(cause instanceof ApiError)) return t("users.import.refusal.unknown");
  if (cause.status === 413) return t(TOO_LARGE_KEY);
  const key = REFUSAL_KEYS.get(cause.code);
  if (key !== undefined) return t(key);
  if (cause.status === 429)
    return t("users.import.refusal.rateLimited", {
      count: cause.retryAfterSeconds ?? 60,
    });
  if (cause.status === 501) return t("users.import.refusal.unavailable");
  return cause.message;
}

/** isTooLarge says a file is over the import's size limit, before anything is sent. */
export function isTooLarge(file: Pick<File, "size">): boolean {
  return file.size > IMPORT_MAX_BYTES;
}

/**
 * isRetryable says the same file may be accepted if sent again: nothing
 * answered, or the server was busy. A server not set up for imports (501)
 * stays so until someone configures it.
 */
export function isRetryable(cause: unknown): boolean {
  return cause instanceof ApiError ? cause.isRetryable && cause.status !== 501 : true;
}

/** isJobRunning says a job is still to be settled by the worker. */
export function isJobRunning(job: Pick<UserImportJob, "status"> | undefined): boolean {
  return job?.status === "queued" || job?.status === "running";
}
