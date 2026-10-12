import type { TFunction } from "i18next";
import type { LibraryAsset, MediaRetirement } from "@/features/media/api";
import { ApiError } from "@/lib/api/errors";
import { dayMonthYear } from "@/lib/i18n/datetime";
import type { Locale } from "@/lib/i18n";

type Usage = Pick<LibraryAsset, "usedIn" | "usageCount">;

/**
 * publishedTestCount is how many published tests use a library file, each
 * counted once: the row lists one entry per version, so a test with three
 * versions that hold the file is still one test.
 */
export function publishedTestCount(asset: Usage): number {
  const tests = new Set((asset.usedIn ?? []).map((test) => test.id));
  return tests.size > 0 ? tests.size : (asset.usageCount ?? 0);
}

/**
 * replaceNote says, before a file is replaced, what becomes of the old one:
 * kept for the published tests that use it, or deleted for good seven days
 * after the replacement.
 */
export function replaceNote(t: TFunction, asset: Usage): string {
  const tests = publishedTestCount(asset);
  return tests > 0 ? t("media.replaceKept", { count: tests }) : t("media.replaceGone");
}

/**
 * removalToast words the toast after a replace or a delete, from the server's
 * answer: the date the old file's object goes when nothing uses it now, how
 * many published tests hold it, or that it stays until nothing uses it. An
 * answer a server of an earlier release does not send leaves the plain line.
 */
export function removalToast(
  t: TFunction,
  locale: Locale,
  action: "replaced" | "deleted",
  retired: Pick<MediaRetirement, "deleteAt" | "usedIn"> | undefined,
): string {
  const replaced = action === "replaced";
  const plain = t(replaced ? "media.replaced" : "media.deleted");
  if (retired === undefined) return plain;
  if (retired.deleteAt != null) {
    const date = dayMonthYear(retired.deleteAt, locale);
    return `${plain}. ${t(replaced ? "media.replacedOn" : "media.deletedOn", { date })}`;
  }
  const tests = new Set(retired.usedIn.map((test) => test.id)).size;
  if (replaced && tests > 0)
    return `${plain}. ${t("media.replacedKept", { count: tests })}`;
  return `${plain}. ${t(replaced ? "media.replacedHeld" : "media.deletedHeld")}`;
}

/**
 * replaceRefusal rewords, for the replace dialog, the two refusals whose
 * server message does not tell the whole story: a 409 `MEDIA_QUOTA_EXCEEDED`
 * for a file published tests still use, where the old file stays in the quota
 * and the library needs room for both, and a 429 that says how long to wait.
 * Any other failure comes back as it was.
 */
export function replaceRefusal(t: TFunction, asset: Usage, cause: unknown): unknown {
  if (!(cause instanceof ApiError)) return cause;
  const message = refusalMessage(t, publishedTestCount(asset) > 0, cause);
  if (message === null) return cause;
  return new ApiError({
    status: cause.status,
    code: cause.code,
    message,
    requestId: cause.requestId,
    details: cause.details,
    retryAfterSeconds: cause.retryAfterSeconds,
  });
}

function refusalMessage(t: TFunction, held: boolean, cause: ApiError): string | null {
  if (held && cause.status === 409 && cause.code === "MEDIA_QUOTA_EXCEEDED")
    return t("media.replaceQuotaFull");
  const seconds = cause.retryAfterSeconds;
  if (!cause.isRateLimited || seconds === undefined) return null;
  return seconds < 60
    ? t("media.replaceRateLimitedSeconds", { count: Math.max(1, Math.ceil(seconds)) })
    : t("media.replaceRateLimitedMinutes", { count: Math.ceil(seconds / 60) });
}
