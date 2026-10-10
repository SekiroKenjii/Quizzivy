import type { components } from "@/lib/api/schema";
import type { AdminQuestion, QuestionType } from "@/features/question-bank/api";

/** QuestionLevel is a question's CEFR level, Pre-A1 to C2 (DG-62). */
export type QuestionLevel = components["schemas"]["QuestionLevel"];

/** QuestionSkill is the skill a question practises. */
export type QuestionSkill = components["schemas"]["QuestionSkill"];

/** TagMatch says whether a question needs any or all of the chosen tags. */
export type TagMatch = "any" | "all";

/** BANK_TYPES lists the stored question types in the order the filter offers them. */
export const BANK_TYPES: readonly QuestionType[] = [
  "single_choice",
  "multiple_choice",
  "true_false",
  "fill_blank",
  "short_answer",
];

/** BANK_LEVELS lists the levels from Pre-A1 to C2. */
export const BANK_LEVELS: readonly QuestionLevel[] = [
  "pre_a1",
  "a1",
  "a2",
  "b1",
  "b2",
  "c1",
  "c2",
];

/** BANK_SKILLS lists the skills in the order the filter offers them. */
export const BANK_SKILLS: readonly QuestionSkill[] = [
  "grammar",
  "vocabulary",
  "reading",
  "listening",
  "writing",
  "speaking",
];

/**
 * BankFilters is the bank's filter state as the URL holds it: repeated
 * `type`, `level`, `skill` and `tag` parameters, `tagMatch=all` when every
 * tag must match, `hasAudio=true`, and `q`.
 */
export interface BankFilters {
  readonly types: readonly QuestionType[];
  readonly levels: readonly QuestionLevel[];
  readonly skills: readonly QuestionSkill[];
  readonly tags: readonly string[];
  readonly tagMatch: TagMatch;
  readonly audio: boolean;
  readonly query: string;
}

function known<T extends string>(
  allowed: readonly T[],
  values: readonly string[],
): T[] {
  return values.filter((value): value is T => allowed.some((item) => item === value));
}

/** readBankFilters reads the filters from search parameters, dropping unknown values. */
export function readBankFilters(params: URLSearchParams): BankFilters {
  return {
    types: known(BANK_TYPES, params.getAll("type")),
    levels: known(BANK_LEVELS, params.getAll("level")),
    skills: known(BANK_SKILLS, params.getAll("skill")),
    tags: params.getAll("tag"),
    tagMatch: params.get("tagMatch") === "all" ? "all" : "any",
    audio: params.get("hasAudio") === "true",
    query: params.get("q") ?? "",
  };
}

/** activeFilterCount counts the values chosen in the filter panel; the search is not one. */
export function activeFilterCount(filters: BankFilters): number {
  return (
    filters.types.length +
    filters.levels.length +
    filters.skills.length +
    filters.tags.length +
    (filters.audio ? 1 : 0)
  );
}

/** toggled adds `value` to `values`, or removes it when it is there. */
export function toggled<T>(values: readonly T[], value: T): readonly T[] {
  return values.includes(value)
    ? values.filter((item) => item !== value)
    : [...values, value];
}

/** promptLine is a question's prompt on one line, with a blank's marker drawn as a gap. */
export function promptLine(question: AdminQuestion): string {
  return question.prompt.replace(/\{\{\d+\}\}/g, "___");
}
