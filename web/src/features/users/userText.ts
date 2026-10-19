import type { TFunction } from "i18next";
import { ApiError } from "@/lib/api/errors";
import type { AdminUser } from "@/features/users/api";
import type { Locale } from "@/lib/i18n";
import { formatRelativeShort } from "@/lib/i18n/datetime";

/** StatusKey names an account's state in the `users.status` strings. */
export type StatusKey = "active" | "mustChangePassword" | "neverSignedIn" | "disabled";

/**
 * statusKeyOf is the state the list draws for an account, which the server
 * decides (DG-54): disabled, then on a temporary password, then never signed
 * in, then active. A state this build does not know, from a server one
 * release ahead, is read off the last activity, so the row still draws.
 */
export function statusKeyOf(
  user: Pick<AdminUser, "status" | "lastActiveAt">,
): StatusKey {
  switch (user.status) {
    case "active":
      return "active";
    case "must_change_password":
      return "mustChangePassword";
    case "never_signed_in":
      return "neverSignedIn";
    case "disabled":
      return "disabled";
    default:
      return user.lastActiveAt === null ? "neverSignedIn" : "active";
  }
}

/**
 * lastActiveText is when an account last made an authenticated request, as a
 * table cell says it: the bare value, "Never" before the first one.
 */
export function lastActiveText(
  user: Pick<AdminUser, "lastActiveAt">,
  locale: Locale,
  t: TFunction,
): string {
  const at = user.lastActiveAt;
  return at === null
    ? t("users.neverActive")
    : formatRelativeShort(at, locale, t("common.agoNow"));
}

const REFUSALS = new Map<string, string>([
  ["TARGET_ROLE_NOT_ALLOWED", "users.error.targetRoleNotAllowed"],
  ["LAST_ADMIN", "users.error.lastAdmin"],
  ["USER_OWNS_CONTENT", "users.error.ownsContent"],
  ["CANNOT_ACT_ON_SELF", "users.error.self"],
  ["ACCOUNT_DISABLED", "users.error.accountDisabled"],
  ["PASSWORD_SIGN_IN_DISABLED", "users.error.passwordSignInOff"],
  ["STALE_WRITE", "users.error.stale"],
  ["ROLE_NOT_ASSIGNABLE", "users.error.roleNotAssignable"],
  ["USER_NOT_DISABLED", "users.error.notDisabled"],
  ["INVALID_RECIPIENT", "users.error.invalidRecipient"],
  ["EMAIL_TAKEN", "users.addUser.emailTaken"],
  ["NOT_FOUND", "users.error.notFound"],
]);

/**
 * refusalText is what the sheet and its dialogs say when the server refused
 * a write: the reason for a refusal the Users screens know, how long to wait
 * after a rate limit, a missing link key (501) as such, and `fallback`
 * otherwise.
 */
export function refusalText(cause: unknown, t: TFunction, fallback: string): string {
  if (!(cause instanceof ApiError)) return fallback;
  const key = REFUSALS.get(cause.code);
  if (key !== undefined) return t(key);
  if (cause.status === 501) return t("users.error.noLinkKey");
  const seconds = cause.retryAfterSeconds;
  if (cause.isRateLimited)
    return seconds === undefined
      ? t("users.error.rateLimited")
      : t("users.error.rateLimitedSeconds", { count: Math.max(1, Math.ceil(seconds)) });
  return fallback;
}

/**
 * failureText says why a bulk action did not reach one account: a refusal
 * the Users screens know, or a fault on our side.
 */
export function failureText(code: string, t: TFunction): string {
  return t(REFUSALS.get(code) ?? "users.credential.reason.internal");
}
