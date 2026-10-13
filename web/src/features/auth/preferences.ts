import type { components } from "@/lib/api/schema";
import type { Locale } from "@/lib/i18n";

/** AiPreferences is every key of `UserPreferences.ai`, each with its value or its default. */
export type AiPreferences = Required<
  NonNullable<components["schemas"]["UserPreferences"]["ai"]>
>;

/**
 * aiPreferencesOf completes the stored `UserPreferences.ai` with the client's
 * defaults, as the contract describes them: the language of the interface,
 * AI drafts tagged, student answers read, imports not read.
 */
export function aiPreferencesOf(
  stored: components["schemas"]["UserPreferences"]["ai"],
  locale: Locale,
): AiPreferences {
  return {
    language: stored?.language ?? locale,
    tagDrafts: stored?.tagDrafts ?? true,
    readStudentAnswers: stored?.readStudentAnswers ?? true,
    readImports: stored?.readImports ?? false,
  };
}
