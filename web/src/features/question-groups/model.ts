import type { TFunction } from "i18next";
import type { components } from "@/lib/api/schema";
import type { ContentBlock, SemanticContent } from "@/components/shared/content/model";
import { validateContent } from "@/components/shared/content/validation";
import { contentStringLength } from "@/components/shared/content/unicode";
import { questionGaps } from "@/components/shared/content/gaps";
import {
  questionSchema,
  type QuestionValues,
} from "@/features/question-bank/questionSchema";
import type { GroupBundle } from "./api";

type GroupQuestionInput = components["schemas"]["QuestionInput"];
export type GroupMaterial = GroupBundle["group"]["stimuli"][number];
export type GroupRecording = GroupBundle["group"]["recordings"][number];
export type GroupGapBinding = GroupMaterial["gaps"][number];

export function emptyGroup(title: string): GroupBundle {
  return {
    group: { id: crypto.randomUUID(), title, members: [], stimuli: [], recordings: [] },
    questions: [],
  };
}

export function emptyMaterial(title: string): GroupMaterial {
  return {
    id: crypto.randomUUID(),
    title,
    content: { format: "semantic_v1", blocks: [{ type: "paragraph", content: [] }] },
    gaps: [],
  };
}

export function memberValues(input: GroupQuestionInput): QuestionValues {
  return {
    ...input,
    promptContent: input.promptContent ?? null,
    explanationContent: input.explanationContent ?? null,
    mediaAssetId: input.mediaAssetId ?? null,
    audio: input.audio ?? null,
    transcript: input.transcript ?? null,
    explanation: input.explanation ?? null,
    sampleAnswer: input.sampleAnswer ?? null,
    tags: input.tags ?? [],
    options: (input.options ?? []).map((option) => ({
      ...option,
      id: option.id ?? null,
      content: option.content ?? null,
    })),
    blanks: (input.blanks ?? []).map((blank) => ({
      ...blank,
      id: blank.id ?? null,
      caseSensitive: blank.caseSensitive ?? false,
    })),
  };
}

export function newGroupQuestion(t: TFunction): GroupBundle["questions"][number] {
  return {
    id: crypto.randomUUID(),
    input: {
      type: "single_choice",
      prompt: t("builder.starterPrompt"),
      points: 1,
      options: [
        { text: t("builder.starterOption", { n: 1 }), isCorrect: true },
        { text: t("builder.starterOption", { n: 2 }), isCorrect: false },
      ],
      blanks: [],
      tags: [],
    },
  };
}

export function materialAssets(materials: readonly GroupMaterial[]) {
  const found = new Map<string, "image" | "audio">();
  const visit = (block: ContentBlock) => {
    if (block.type === "image" || block.type === "audio")
      found.set(block.assetId, block.type);
    else if (block.type === "list") block.items.forEach((item) => item.forEach(visit));
    else if (block.type === "table")
      block.rows.forEach((row) => row.forEach((cell) => cell.content.forEach(visit)));
  };
  for (const material of materials)
    if (material.content.format === "semantic_v1")
      material.content.blocks.forEach(visit);
  return found;
}

export function materialGaps(material: GroupMaterial) {
  return material.content.format === "semantic_v1"
    ? questionGaps(material.content)
    : [];
}

export function changeMaterialContent(
  material: GroupMaterial,
  content: SemanticContent,
): GroupMaterial {
  const ids = new Set(questionGaps(content).map((gap) => gap.id));
  return {
    ...material,
    content,
    gaps: material.gaps.filter((gap) => ids.has(gap.gapId)),
  };
}

function bindingIssue(
  binding: GroupGapBinding,
  question: GroupQuestionInput | undefined,
) {
  if (!question) return "groups.unboundGaps";
  if (binding.kind === "blank") {
    return question.type === "fill_blank" &&
      question.blanks?.some((blank) => blank.gapId === binding.blankGapId)
      ? null
      : "groups.unboundGaps";
  }
  return ["single_choice", "multiple_choice", "true_false"].includes(question.type)
    ? null
    : "groups.choiceTargetRequired";
}

function materialBindingsIssue(
  material: GroupMaterial,
  gapIds: Set<string>,
  members: Map<string, GroupQuestionInput>,
  used: Set<string>,
): string | null {
  for (const binding of material.gaps) {
    if (!gapIds.delete(binding.gapId)) return "groups.unboundGaps";
    const issue = bindingIssue(binding, members.get(binding.questionId));
    if (issue) return issue;
    const target = JSON.stringify([
      binding.questionId,
      binding.kind === "blank" ? binding.blankGapId : null,
    ]);
    if (used.has(target)) return "groups.duplicateTarget";
    used.add(target);
  }
  return null;
}

function materialsIssue(bundle: GroupBundle): string | null {
  const members = new Map(
    bundle.questions.map((question) => [question.id, question.input]),
  );
  const used = new Set<string>();
  for (const material of bundle.group.stimuli) {
    if (!material.title.trim() || contentStringLength(material.title) > 200)
      return "groups.materialTitleRequired";
    if (!validateContent(material.content).ok) return "groups.limitExceeded";
    const gapIds = new Set(materialGaps(material).map((gap) => gap.id));
    if (gapIds.size !== material.gaps.length) return "groups.unboundGaps";
    const issue = materialBindingsIssue(material, gapIds, members, used);
    if (issue) return issue;
  }
  return null;
}

export function groupIssue(bundle: GroupBundle): string | null {
  const { group } = bundle;
  if (!group.title.trim() || contentStringLength(group.title) > 200)
    return "groups.titleRequired";
  if (
    group.members.length > 200 ||
    group.stimuli.length > 16 ||
    group.recordings.length > 16
  )
    return "groups.limitExceeded";
  const ids = new Set(bundle.questions.map((question) => question.id));
  if (
    ids.size !== bundle.questions.length ||
    ids.size !== group.members.length ||
    group.members.some((member) => !ids.delete(member.questionId))
  )
    return "groups.invalidQuestion";
  for (const question of bundle.questions) {
    const parsed = questionSchema.safeParse(memberValues(question.input));
    if (!parsed.success)
      return parsed.error.issues[0]?.message ?? "groups.invalidQuestion";
  }
  const issue = materialsIssue(bundle);
  if (issue) return issue;
  if (new TextEncoder().encode(JSON.stringify(bundle)).byteLength > 4 * 1024 * 1024)
    return "groups.limitExceeded";
  return null;
}
