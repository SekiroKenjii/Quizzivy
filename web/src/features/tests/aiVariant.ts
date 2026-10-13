import type { AiVariantChange } from "@/features/ai/api";
import { AiWriteError } from "@/features/ai/kind";
import {
  duplicateQuestion,
  getQuestion,
  toFormValues,
  updateQuestion,
  type AdminQuestion,
} from "@/features/question-bank/api";
import { withVariant } from "@/features/question-bank/aiDrafts";
import { questionSchema } from "@/features/question-bank/questionSchema";
import { duplicateTest, saveMixedOutline, toMixedOutline, type Test } from "./api";

const TITLE_MAX = 200;

function byQuestion(changes: readonly AiVariantChange[]) {
  const grouped = new Map<string, AiVariantChange[]>();
  for (const change of changes) {
    const list = grouped.get(change.questionId) ?? [];
    list.push(change);
    grouped.set(change.questionId, list);
  }
  return grouped;
}

function optionIds(from: AdminQuestion, to: AdminQuestion): Map<string, string> {
  const ids = new Map<string, string>();
  for (const [index, option] of (from.options ?? []).entries()) {
    const copy = to.options?.[index];
    if (copy) ids.set(option.id, copy.id);
  }
  return ids;
}

/**
 * VariantNames are the words a variant's writes need: the new test's title,
 * and the refusal for a change that cannot be saved, by the question's number.
 */
export interface VariantNames {
  title: string;
  invalid: (number: number) => string;
}

/**
 * saveVariant makes a variant of `test` from the kept `changes` through the
 * existing writes, and answers the new test's id. Every changed question is
 * checked first, so a change that cannot be saved stops it before anything
 * is written. Then `duplicateTest` copies the draft into a new draft test
 * named `names.title`. A copy's outline names the same bank questions as the
 * test it copies, so each changed question is duplicated, the changes are
 * written to the duplicate with `updateQuestion`, and the copy's outline is
 * saved with the duplicate in its place: the test and its questions are never
 * touched.
 */
export async function saveVariant(
  test: Pick<Test, "id">,
  changes: readonly AiVariantChange[],
  names: VariantNames,
): Promise<string> {
  const grouped = byQuestion(changes);
  const sources = new Map<string, AdminQuestion>();
  for (const [questionId, list] of grouped) {
    const source = await getQuestion(questionId);
    if (!questionSchema.safeParse(withVariant(toFormValues(source), list)).success)
      throw new AiWriteError(names.invalid(list[0]?.number ?? 0));
    sources.set(questionId, source);
  }
  const copy = await duplicateTest(test.id);
  const swaps = new Map<string, string>();
  for (const [questionId, list] of grouped) {
    const source = sources.get(questionId);
    if (!source) continue;
    const duplicate = await duplicateQuestion(questionId);
    const values = withVariant(
      toFormValues(duplicate),
      list,
      optionIds(source, duplicate),
    );
    const saved = await updateQuestion(duplicate.id, values);
    swaps.set(questionId, saved.id);
  }
  const sections = toMixedOutline(copy).map((section) => ({
    ...section,
    units: section.units.map((unit) =>
      unit.kind === "question" ? { ...unit, id: swaps.get(unit.id) ?? unit.id } : unit,
    ),
  }));
  await saveMixedOutline(
    copy.id,
    copy.updatedAt,
    names.title.slice(0, TITLE_MAX),
    sections,
  );
  return copy.id;
}
