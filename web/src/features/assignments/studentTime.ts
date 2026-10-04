import type { TFunction } from "i18next";
import type { Locale } from "@/lib/i18n";

/** "Minh" from "Nguyễn Đức Minh": a Vietnamese given name is the last word. */
export function givenName(fullName: string): string {
  const parts = fullName.trim().split(/\s+/);
  return parts.at(-1) ?? "";
}

/** "27/30", with the decimals the numbers actually have. */
export function scoreText(
  earned: number,
  total: number,
  locale: Locale,
  t: TFunction,
): string {
  const n = new Intl.NumberFormat(locale, { maximumFractionDigits: 2 });
  return t("student.score", { earned: n.format(earned), total: n.format(total) });
}
