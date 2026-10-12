import type { TFunction } from "i18next";

const DECK_ORDER: readonly string[] = ["docx", "doc", "pdf"];

function rank(format: string): number {
  const index = DECK_ORDER.indexOf(format);
  return index === -1 ? DECK_ORDER.length : index;
}

/**
 * orderFormats returns the formats the server accepts in the deck's order,
 * DOCX, DOC, PDF, with any other format after them in the server's order.
 */
export function orderFormats(formats: readonly string[]): string[] {
  return [...formats].sort((a, b) => rank(a) - rank(b));
}

/**
 * listFormats writes the server's formats as the deck does: in its order,
 * separated by commas, with "or" or "and" before the last and no comma
 * before that word. `shape` writes each one, as "DOCX" or ".docx".
 */
export function listFormats(
  formats: readonly string[],
  shape: "name" | "extension",
  joiner: "or" | "and",
  t: TFunction,
): string {
  const items = orderFormats(formats).map((format) =>
    shape === "name" ? format.toUpperCase() : `.${format}`,
  );
  const last = items.at(-1) ?? "";
  if (items.length < 2) return last;
  const head = items.slice(0, -1).join(", ");
  return joiner === "or"
    ? t("imports.formatList.or", { head, last })
    : t("imports.formatList.and", { head, last });
}
