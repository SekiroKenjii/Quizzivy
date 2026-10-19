import type { TFunction } from "i18next";
import {
  BookOpenText,
  Eraser,
  FilePlus,
  Layers,
  Library,
  Plus,
  Sparkles,
} from "lucide-react";
import { trueFalseLabelKey } from "@/components/shared/content/trueFalse";
import {
  generateAiQuestions,
  type AiGeneratedQuestion,
  type AiGenerateRequest,
} from "../api";
import type { AiCard, AiField, AiKind } from "../kind";

/** AiPassageGroup is the shared-context group of the open test that holds a passage. */
export interface AiPassageGroup {
  id: string;
  title: string;
}

/**
 * AiPassage is a shared-content passage of the open test that a generation
 * can read. `group` is the group that holds it, given only by a host that
 * adds the kept questions to that group.
 */
export interface AiPassage {
  id: string;
  title: string;
  text: string;
  group?: AiPassageGroup;
}

/** AiSection is a section of the open test that a standalone run can read and fill. */
export interface AiSection {
  id: string;
  title: string;
}

/**
 * GenerateDestination is where the kept questions go: a section (by `id`
 * when the run read that section, else the one the page is on when they are
 * kept), the group that holds the passage they were generated from, the
 * bank, or a new practice test.
 */
export type GenerateDestination =
  | { kind: "section"; title: string; id?: string }
  | { kind: "group"; id: string; title: string }
  | { kind: "bank" }
  | { kind: "practice" };

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
 * passages it offers, the sections a standalone run can be fitted to and
 * `section`, the one it starts from (the one new questions join), the
 * question types to start from when not the default,
 * where the kept questions go (a section, the bank, or a new practice test),
 * and `save`, the page's own write for them into the run's destination, which
 * tags each with `tag` when it is not null. A section gives way to the group
 * that holds the passage the run read; any other destination is read from the
 * page when the questions are kept, not when they were made.
 */
export interface GenerateContext {
  testId?: string;
  passages: readonly AiPassage[];
  section?: AiSection;
  sections?: readonly AiSection[];
  types?: readonly QuestionType[];
  destination: Exclude<GenerateDestination, { kind: "group" }>;
  save: (
    drafts: readonly GeneratedDraft[],
    tag: string | null,
    destination: GenerateDestination,
  ) => Promise<void>;
}

type GenerateStyle = "standalone" | "passage";

interface GenerateValues {
  style: GenerateStyle;
  fitSection: boolean;
  sectionId: string;
  source: string;
  types: readonly string[];
  count: string;
  level: string;
  explanations: boolean;
}

interface GenerateItem extends GeneratedDraft {
  explanations: boolean;
  destination: GenerateDestination;
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
const TOPIC_MIN = 3;
const STYLES: readonly GenerateStyle[] = ["standalone", "passage"];
const SOURCE_MAX = 20000;

const PRIMARY_ICONS = {
  section: Plus,
  group: Layers,
  bank: Library,
  practice: FilePlus,
} as const;

function isLevel(value: string): value is AiLevel {
  return (LEVELS as readonly string[]).includes(value);
}

function initialTypes(preset: readonly QuestionType[] | undefined) {
  const types = TYPES.filter((type) => preset?.includes(type));
  return types.length > 0 ? types : DEFAULT_TYPES;
}

function chosenPassage(values: GenerateValues, context: GenerateContext) {
  return values.style === "passage"
    ? context.passages.find((item) => item.text === values.source)
    : undefined;
}

function offeredSections(context: GenerateContext): readonly AiSection[] {
  if (!context.testId || context.destination.kind !== "section") return [];
  if (context.sections && context.sections.length > 0) return context.sections;
  return context.section ? [context.section] : [];
}

function chosenSection(values: GenerateValues, context: GenerateContext) {
  const offered = offeredSections(context);
  return offered.find((section) => section.id === values.sectionId) ?? offered[0];
}

function fittedSection(values: GenerateValues, context: GenerateContext) {
  return values.style === "standalone" && values.fitSection
    ? chosenSection(values, context)
    : undefined;
}

function destinationFor(
  values: GenerateValues,
  context: GenerateContext,
): GenerateDestination {
  const section = fittedSection(values, context);
  if (section) return { kind: "section", id: section.id, title: section.title };
  const passage = chosenPassage(values, context);
  return passage?.group && context.destination.kind === "section"
    ? { kind: "group", id: passage.group.id, title: passage.group.title }
    : context.destination;
}

function withStyle(
  style: string,
  values: GenerateValues,
  context: GenerateContext,
): Partial<GenerateValues> {
  if (style === "standalone") {
    const fromPassage = context.passages.some((item) => item.text === values.source);
    return { style, ...(fromPassage ? { source: "" } : {}) };
  }
  const first = context.passages[0];
  return {
    style: "passage",
    ...(values.source.trim() === "" && first ? { source: first.text } : {}),
  };
}

function topicMissing(values: GenerateValues, context: GenerateContext) {
  const topic = [...values.source.trim()].length;
  if (topic > 0) return topic < TOPIC_MIN;
  return !fittedSection(values, context);
}

function destinationOf(
  items: readonly GenerateItem[],
  context: GenerateContext,
): GenerateDestination {
  const kept = items[0]?.destination;
  return kept?.kind === "group" || (kept?.kind === "section" && kept.id)
    ? kept
    : context.destination;
}

function request(
  values: GenerateValues,
  context: GenerateContext,
  language: AiGenerateRequest["language"],
): AiGenerateRequest {
  const passage = chosenPassage(values, context);
  const level = isLevel(values.level) ? values.level : "b2";
  const settings = {
    types: TYPES.filter((type) => values.types.includes(type)),
    count: Number(values.count) as AiGenerateRequest["count"],
    level,
    explanations: values.explanations,
    language,
  };
  if (values.style === "standalone") {
    const topic = values.source.trim().slice(0, SOURCE_MAX);
    const section = fittedSection(values, context);
    return {
      ...settings,
      standalone: true,
      ...(topic === "" ? {} : { source: topic }),
      ...(section && context.testId
        ? { testId: context.testId, sectionId: section.id }
        : {}),
    };
  }
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

function sourceFields(
  values: GenerateValues,
  set: (patch: Partial<GenerateValues>) => void,
  context: GenerateContext,
  t: TFunction,
): AiField[] {
  if (values.style === "passage")
    return [
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
                  label: t("ai.generate.passage", {
                    n: index + 1,
                    title: passage.title,
                  }),
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
    ];
  const offered = offeredSections(context);
  const section = chosenSection(values, context);
  const topic: AiField = {
    kind: "source",
    label: t("ai.generate.topic"),
    value: values.source,
    placeholder: t("ai.generate.topicPlaceholder"),
    rows: 3,
    sources: [],
    onChange: (source) => set({ source }),
  };
  if (!section) return [topic];
  return [
    topic,
    {
      kind: "toggle",
      text: t("ai.generate.fitSection", { section: section.title }),
      sub: t("ai.generate.fitSectionSub"),
      value: values.fitSection,
      onChange: (fitSection) => set({ fitSection }),
    },
    ...(values.fitSection && offered.length > 1
      ? [
          {
            kind: "seg" as const,
            label: t("ai.generate.section"),
            options: offered.map((item) => ({ value: item.id, label: item.title })),
            value: section.id,
            onChange: (sectionId: string) => set({ sectionId }),
          },
        ]
      : []),
  ];
}

/**
 * generateKind is "Generate questions" (V-14a): standalone questions about a
 * topic or fitted to the open section, or questions from a passage of the
 * open test or pasted text, in the five stored types (TFNG waits for R6,
 * DG-202). It starts from a passage where the page offers one and is
 * standalone otherwise. Its primary saves the kept questions through the
 * page's write: in the builder into the section a fitted run read, the group
 * that holds the chosen passage, or else the open question's section; into
 * the bank; or into a new practice test.
 */
export const generateKind: AiKind<GenerateValues, GenerateItem, GenerateContext> = {
  icon: Sparkles,
  titleKey: "ai.generate.title",
  subtitleKey: "ai.generate.subtitle",
  runLabelKey: "ai.generate.run",
  stepKeys: ["ai.generate.step1", "ai.generate.step2", "ai.generate.step3"],
  emptyKey: "ai.generate.empty",
  initial: (context) => ({
    style: context.passages.length > 0 ? "passage" : "standalone",
    fitSection: true,
    sectionId: context.section?.id ?? "",
    source: context.passages[0]?.text ?? "",
    types: initialTypes(context.types),
    count: "5",
    level: "b2",
    explanations: true,
  }),
  fields: (values, set, context, t) => [
    {
      kind: "seg",
      label: t("ai.generate.style"),
      options: STYLES.map((style) => ({
        value: style,
        label: t(`ai.generate.styles.${style}`),
      })),
      value: values.style,
      onChange: (style) => set(withStyle(style, values, context)),
    },
    ...sourceFields(values, set, context, t),
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
  problem: (values, context) => {
    if (values.style === "standalone") {
      if (topicMissing(values, context)) return "ai.generate.needTopic";
    } else if (values.source.trim().length < SOURCE_MIN)
      return "ai.generate.needSource";
    if (values.types.length === 0) return "ai.generate.needType";
    return null;
  },
  run: async (values, env, signal) => {
    const body = request(values, env.context, env.language);
    const destination = destinationFor(values, env.context);
    const result = await generateAiQuestions(body, signal);
    return {
      items: result.items.map((question) => ({
        question,
        level: body.level,
        explanations: body.explanations,
        destination,
      })),
      usage: result.usage,
    };
  },
  card: (item, env) => card(item, env.t, env.tagDrafts),
  summary: (items, selected, t) =>
    t("ai.generate.summary", { n: selected, total: items.length }),
  primary: {
    icon: ({ context }, items) => PRIMARY_ICONS[destinationOf(items, context).kind],
    label: (selected, { t, context }, items) => {
      const destination = destinationOf(items, context);
      const count = selected.length;
      switch (destination.kind) {
        case "practice":
          return t("ai.generate.saveAsPractice");
        case "bank":
          return t("ai.generate.saveToBank", { count });
        case "group":
          return t("ai.generate.addToGroup", { count, group: destination.title });
        case "section":
          return t("ai.generate.addTo", { count, section: destination.title });
      }
    },
    apply: async (selected, { t, context, tagDrafts }) => {
      const destination = destinationOf(selected, context);
      const count = selected.length;
      await context.save(
        selected.map(({ question, level }) => ({ question, level })),
        tagDrafts ? t("ai.draftTag") : null,
        destination,
      );
      switch (destination.kind) {
        case "practice":
          return t("ai.generate.savedPractice", { count });
        case "bank":
          return t(tagDrafts ? "ai.generate.savedTagged" : "ai.generate.saved", {
            count,
          });
        case "group":
          return t("ai.generate.addedToGroup", { count, group: destination.title });
        case "section":
          return t("ai.generate.added", { count, section: destination.title });
      }
    },
  },
};
