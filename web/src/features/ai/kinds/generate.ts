import type { TFunction } from "i18next";
import { BookOpenText, Eraser, FilePlus, Library, Plus, Sparkles } from "lucide-react";
import { trueFalseLabelKey } from "@/components/shared/content/trueFalse";
import {
  generateAiQuestions,
  type AiGeneratedQuestion,
  type AiGenerateRequest,
} from "../api";
import type { AiCard, AiKind } from "../kind";

/** AiPassage is a shared-content passage of the open test that a generation can read. */
export interface AiPassage {
  id: string;
  title: string;
  text: string;
}

/** AiLevel is a level a generation writes for. */
export type AiLevel = AiGenerateRequest["level"];

/** GeneratedDraft is one question the teacher kept, with the level it was written for. */
export interface GeneratedDraft {
  question: AiGeneratedQuestion;
  level: AiLevel;
}

type QuestionType = AiGeneratedQuestion["type"];

/**
 * GenerateContext is what a page gives the Generate panel: the test whose
 * passages it offers, the question types to start from when not the default,
 * where the kept questions go (a section, the bank, or a new practice test),
 * and `save`, the page's own write for them, which tags each with `tag` when
 * it is not null.
 */
export interface GenerateContext {
  testId?: string;
  passages: readonly AiPassage[];
  types?: readonly QuestionType[];
  destination:
    { kind: "section"; title: string } | { kind: "bank" } | { kind: "practice" };
  save: (drafts: readonly GeneratedDraft[], tag: string | null) => Promise<void>;
}

interface GenerateValues {
  source: string;
  types: readonly string[];
  count: string;
  level: string;
  explanations: boolean;
}

interface GenerateItem extends GeneratedDraft {
  explanations: boolean;
}

const TYPES: readonly QuestionType[] = [
  "single_choice",
  "multiple_choice",
  "true_false",
  "fill_blank",
  "short_answer",
];
const DEFAULT_TYPES: readonly QuestionType[] = ["single_choice", "true_false"];
const COUNTS = ["3", "5", "8"] as const;
const LEVELS = ["b1", "b2", "c1"] as const;
const SOURCE_MIN = 40;
const SOURCE_MAX = 20000;

const PRIMARY_ICONS = { section: Plus, bank: Library, practice: FilePlus } as const;

function isLevel(value: string): value is AiLevel {
  return (LEVELS as readonly string[]).includes(value);
}

function initialTypes(preset: readonly QuestionType[] | undefined) {
  const types = TYPES.filter((type) => preset?.includes(type));
  return types.length > 0 ? types : DEFAULT_TYPES;
}

function request(
  values: GenerateValues,
  context: GenerateContext,
  language: AiGenerateRequest["language"],
): AiGenerateRequest {
  const passage = context.passages.find((item) => item.text === values.source);
  const level = isLevel(values.level) ? values.level : "b2";
  const settings = {
    types: TYPES.filter((type) => values.types.includes(type)),
    count: Number(values.count) as AiGenerateRequest["count"],
    level,
    explanations: values.explanations,
    language,
  };
  return passage && context.testId
    ? { ...settings, testId: context.testId, stimulusId: passage.id }
    : { ...settings, source: values.source.slice(0, SOURCE_MAX) };
}

function optionText(question: AiGeneratedQuestion, text: string, t: TFunction) {
  const key =
    question.type === "true_false" ? trueFalseLabelKey("true_false", text, null) : null;
  return key ? t(key) : text;
}

function answerOf(question: AiGeneratedQuestion): string | undefined {
  if (question.type === "fill_blank") return question.answerKey.join(" / ");
  if (question.type === "short_answer") return question.answerKey[0];
  return undefined;
}

function card(item: GenerateItem, t: TFunction, tagDrafts: boolean): AiCard {
  const { question } = item;
  const keys = new Set(question.answerKey);
  const answer = answerOf(question);
  return {
    selectable: true,
    tags: [
      { label: t(`questionEditor.type.${question.type}`), tone: "muted" },
      { label: t(`bank.level.${item.level}`), tone: "muted" },
      ...(tagDrafts ? [{ label: t("ai.draftTag"), tone: "accent" as const }] : []),
    ],
    title: question.prompt,
    ...(question.options
      ? {
          options: question.options.map((text) => ({
            text: optionText(question, text, t),
            correct: keys.has(text),
          })),
        }
      : {}),
    ...(answer === undefined ? {} : { answer }),
    ...(item.explanations && question.explanation.trim() !== ""
      ? { note: { label: t("ai.generate.explanation"), text: question.explanation } }
      : {}),
  };
}

/**
 * generateKind is "Generate questions" (V-14a): from a passage of the open
 * test, pasted text or a topic, in the five stored types (TFNG waits for R6,
 * DG-202). Its primary saves the kept questions through the page's write:
 * into the open question's section in the builder, or into the bank.
 */
export const generateKind: AiKind<GenerateValues, GenerateItem, GenerateContext> = {
  icon: Sparkles,
  titleKey: "ai.generate.title",
  subtitleKey: "ai.generate.subtitle",
  runLabelKey: "ai.generate.run",
  stepKeys: ["ai.generate.step1", "ai.generate.step2", "ai.generate.step3"],
  emptyKey: "ai.generate.empty",
  initial: (context) => ({
    source: context.passages[0]?.text ?? "",
    types: initialTypes(context.types),
    count: "5",
    level: "b2",
    explanations: true,
  }),
  fields: (values, set, context, t) => [
    {
      kind: "source",
      label: t("ai.generate.source"),
      value: values.source,
      placeholder: t("ai.generate.sourcePlaceholder"),
      rows: 8,
      sources:
        context.passages.length === 0
          ? []
          : [
              ...context.passages.map((passage, index) => ({
                key: passage.id,
                label: t("ai.generate.passage", { n: index + 1, title: passage.title }),
                icon: BookOpenText,
                active: values.source === passage.text,
                pick: () => set({ source: passage.text }),
              })),
              {
                key: "empty",
                label: t("ai.generate.startEmpty"),
                icon: Eraser,
                active: values.source === "",
                pick: () => set({ source: "" }),
              },
            ],
      onChange: (source) => set({ source }),
    },
    {
      kind: "chips",
      label: t("ai.generate.types"),
      options: TYPES.map((type) => ({
        value: type,
        label: t(`questionEditor.type.${type}`),
      })),
      value: values.types,
      onChange: (types) => set({ types }),
    },
    {
      kind: "seg",
      label: t("ai.generate.count"),
      options: COUNTS.map((count) => ({ value: count, label: count })),
      value: values.count,
      onChange: (count) => set({ count }),
    },
    {
      kind: "seg",
      label: t("ai.generate.level"),
      options: LEVELS.map((level) => ({
        value: level,
        label: t(`bank.level.${level}`),
      })),
      value: values.level,
      onChange: (level) => set({ level }),
    },
    {
      kind: "toggle",
      text: t("ai.generate.explanations"),
      sub: t("ai.generate.explanationsSub"),
      value: values.explanations,
      onChange: (explanations) => set({ explanations }),
    },
  ],
  problem: (values) => {
    if (values.source.trim().length < SOURCE_MIN) return "ai.generate.needSource";
    if (values.types.length === 0) return "ai.generate.needType";
    return null;
  },
  run: async (values, env, signal) => {
    const body = request(values, env.context, env.language);
    const result = await generateAiQuestions(body, signal);
    return {
      items: result.items.map((question) => ({
        question,
        level: body.level,
        explanations: body.explanations,
      })),
      usage: result.usage,
    };
  },
  card: (item, env) => card(item, env.t, env.tagDrafts),
  summary: (items, selected, t) =>
    t("ai.generate.summary", { n: selected, total: items.length }),
  primary: {
    icon: ({ context }) => PRIMARY_ICONS[context.destination.kind],
    label: (selected, { t, context }) => {
      const { destination } = context;
      if (destination.kind === "practice") return t("ai.generate.saveAsPractice");
      if (destination.kind === "bank")
        return t("ai.generate.saveToBank", { count: selected.length });
      return t("ai.generate.addTo", {
        count: selected.length,
        section: destination.title,
      });
    },
    apply: async (selected, { t, context, tagDrafts }) => {
      await context.save(
        selected.map(({ question, level }) => ({ question, level })),
        tagDrafts ? t("ai.draftTag") : null,
      );
      const { destination } = context;
      if (destination.kind === "practice")
        return t("ai.generate.savedPractice", { count: selected.length });
      if (destination.kind === "bank")
        return t(tagDrafts ? "ai.generate.savedTagged" : "ai.generate.saved", {
          count: selected.length,
        });
      return t("ai.generate.added", {
        count: selected.length,
        section: destination.title,
      });
    },
  },
};
