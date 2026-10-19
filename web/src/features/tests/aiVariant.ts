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
import {
  getGroup,
  updateGroup,
  type StoredGroup,
} from "@/features/question-groups/api";
import { memberValues } from "@/features/question-groups/model";
import { duplicateTest, saveMixedOutline, toMixedOutline, type Test } from "./api";

const TITLE_MAX = 200;

type MemberInput = StoredGroup["bundle"]["questions"][number]["input"];

interface OptionHolder {
  options?: readonly { id?: string | null }[] | null;
}

function byQuestion(changes: readonly AiVariantChange[]) {
  const grouped = new Map<string, AiVariantChange[]>();
  for (const change of changes) {
    const list = grouped.get(change.questionId) ?? [];
    list.push(change);
    grouped.set(change.questionId, list);
  }
  return grouped;
}

function optionIds(from: OptionHolder, to: OptionHolder): Map<string, string> {
  const ids = new Map<string, string>();
  for (const [index, option] of (from.options ?? []).entries()) {
    const copy = to.options?.[index];
    if (option.id && copy?.id) ids.set(option.id, copy.id);
  }
  return ids;
}

function groupUnitIds(test: Test): string[] {
  return toMixedOutline(test).flatMap((section) =>
    section.units.filter((unit) => unit.kind === "group").map((unit) => unit.id),
  );
}

function questionUnitIds(test: Test): Set<string> {
  return new Set(
    toMixedOutline(test).flatMap((section) =>
      section.units.filter((unit) => unit.kind === "question").map((unit) => unit.id),
    ),
  );
}

function memberInput(group: StoredGroup, questionId: string): MemberInput | undefined {
  return group.bundle.questions.find((question) => question.id === questionId)?.input;
}

function memberIndex(group: StoredGroup, questionId: string): number {
  return group.bundle.group.members.findIndex(
    (member) => member.questionId === questionId,
  );
}

function canSave(values: Parameters<typeof withVariant>[0], list: AiVariantChange[]) {
  return questionSchema.safeParse(withVariant(values, list)).success;
}

async function groupsHolding(test: Test, ids: readonly string[]) {
  const holding = new Map<number, StoredGroup>();
  const left = new Set(ids);
  for (const [index, id] of groupUnitIds(test).entries()) {
    if (left.size === 0) break;
    const group = await getGroup(id);
    const members = group.bundle.group.members.filter((member) =>
      left.has(member.questionId),
    );
    if (members.length === 0) continue;
    holding.set(index, group);
    for (const member of members) left.delete(member.questionId);
  }
  return holding;
}

async function varyCopiedGroup(
  copyId: string,
  original: StoredGroup,
  grouped: ReadonlyMap<string, AiVariantChange[]>,
) {
  const copy = await getGroup(copyId);
  const questions = copy.bundle.questions.map((question) => {
    const index = memberIndex(copy, question.id);
    const source = original.bundle.group.members[index]?.questionId;
    const list = source === undefined ? undefined : grouped.get(source);
    const from = source === undefined ? undefined : memberInput(original, source);
    if (!list || !from) return question;
    return {
      ...question,
      input: withVariant(
        memberValues(question.input),
        list,
        optionIds(from, question.input),
      ),
    };
  });
  await updateGroup(copyId, {
    bundle: { ...copy.bundle, questions },
    expectedRevision: copy.revision,
    expectedTestUpdatedAt: copy.testUpdatedAt ?? null,
  });
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
 * test it copies, so each changed standalone question is duplicated, the
 * changes are written to the duplicate with `updateQuestion`, and the copy's
 * outline is saved with the duplicate in its place. A copy's groups are its
 * own, so a changed member is written to the copy's group, found at the same
 * place in the outline and the member at the same place in the group, with
 * `updateGroup`. The test, its questions and its groups are never touched.
 */
export async function saveVariant(
  test: Test,
  changes: readonly AiVariantChange[],
  names: VariantNames,
): Promise<string> {
  const grouped = byQuestion(changes);
  const standalone = questionUnitIds(test);
  const memberIds = [...grouped.keys()].filter((id) => !standalone.has(id));
  const groups = await groupsHolding(test, memberIds);
  const sources = new Map<string, AdminQuestion>();
  for (const [questionId, list] of grouped) {
    const number = list[0]?.number ?? 0;
    if (standalone.has(questionId)) {
      const source = await getQuestion(questionId);
      if (!canSave(toFormValues(source), list))
        throw new AiWriteError(names.invalid(number));
      sources.set(questionId, source);
      continue;
    }
    const input = [...groups.values()]
      .map((group) => memberInput(group, questionId))
      .find((found) => found !== undefined);
    if (!input || !canSave(memberValues(input), list))
      throw new AiWriteError(names.invalid(number));
  }
  const copy = await duplicateTest(test.id);
  const swaps = new Map<string, string>();
  for (const [questionId, source] of sources) {
    const list = grouped.get(questionId) ?? [];
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
  const copiedGroups = groupUnitIds(copy);
  for (const [index, original] of groups) {
    const copyId = copiedGroups[index];
    if (copyId) await varyCopiedGroup(copyId, original, grouped);
  }
  return copy.id;
}
