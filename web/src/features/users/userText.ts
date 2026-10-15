import type { TFunction } from "i18next";
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
