import type { TFunction } from "i18next";
import { nfc } from "@/lib/nfc";
import type { WordImport } from "./api";

/**
 * importName is how the teacher console names an import in its heading and
 * breadcrumb, as the deck does: the exam's file name, "Pasted text" for a
 * pasted one, or the import's title while it has no exam.
 */
export function importName(value: WordImport, t: TFunction): string {
  const exam = value.sources.find((source) => source.role === "exam");
  if (exam?.format === "text") return t("imports.detail.pastedText");
  return nfc(exam?.filename ?? value.title);
}
