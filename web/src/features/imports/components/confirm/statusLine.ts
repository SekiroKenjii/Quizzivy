import type { TFunction } from "i18next";
import type { ImportStatus } from "../../api";

/** statusLine says why an import that is not under review cannot become a draft. */
export function statusLine(status: ImportStatus, t: TFunction): string {
  if (status === "cancelled") return t("imports.confirm.cancelled");
  if (status === "failed") return t("imports.confirm.failed");
  return t("imports.confirm.notUnderReview");
}
