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
import { failureMessage } from "@/lib/api/errors";
import {
  generateAiQuestions,
  type AiGeneratedQuestion,
  type AiGenerateRequest,
} from "../api";
import {
  AiPartialWriteError,
  AiWriteError,
  type AiCard,
  type AiEnv,
  type AiFailedPart,
  type AiField,
  type AiGroup,
  type AiKind,
  type AiPart,
  type AiPlanRow,
  type AiRunResult,
} from "../kind";
import { aiFailureOf } from "../outcome";

/** AiPassageGroup is the shared-context group of the open test that holds a passage. */
export interface AiPassageGroup {
  id: string;
  title: string;
}

/**
 * AiPassage is a shared-content passage of the open test that a generation
 * can read. `group` is the group that holds it, given only by a host that
 * adds the kept questions to that group, and `sectionId` the section that
 * holds the group.
 */
export interface AiPassage {
  id: string;
  title: string;
  text: string;
  group?: AiPassageGroup;
  sectionId?: string;
}

/**
 * AiSection is a saved section of the open test that a standalone run can
 * read and fill: whether it has instructions, how many questions it holds
 * (group members included), the types among them and the level they share,
 * when they share one.
 */
export interface AiSection {
  id: string;
  title: string;
  instructions?: boolean;
  questions?: number;
  types?: readonly QuestionType[];
  level?: AiLevel;
}

/**
 * GenerateDestination is where the kept questions go: a section (by `id`
 * when the run named it, else the one the page is on when they are kept),
 * the group that holds the passage they were generated from, the bank, or a
 * new practice test.
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
 * passages it offers, its saved sections, which a standalone run can be
 * fitted to and a whole-test plan fills, and `section`, the open one, which
 * new questions join and whose passages and types the panel starts from;
 * the question types to start from otherwise; where the kept questions go (a
 * section, the bank, or a new practice test); and `save`, the page's own
 * write for them into a destination, which tags each with `tag` when it is
 * not null. A run names its section or group when it starts; the bank's and
 * a practice test's destination is read when the questions are kept.
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

type GenerateScope = "section" | "test";
type GenerateStyle = "standalone" | "passage";
type PlanMode = "passage" | "standalone";

interface PlanEntry {
  count: string;
  mode: PlanMode;
  level: string;
  types: readonly string[];
}

interface GenerateValues {
  scope: GenerateScope;
  style: GenerateStyle;
  fitSection: boolean;
  sectionId: string;
  source: string;
  types: readonly string[];
  typesTouched: boolean;
  count: string;
  level: string;
  explanations: boolean;
  plan: Readonly<Record<string, PlanEntry>>;
  retarget: string;
}

interface GenerateItem extends GeneratedDraft {
  explanations: boolean;
  destination: GenerateDestination;
  group?: AiGroup;
}

interface Batch {
  key: string;
  section: AiSection;
  body: AiGenerateRequest;
  destination: GenerateDestination;
  sub?: AiGroup["sub"];
}

const TYPES: readonly QuestionType[] = [
  "single_choice",
  "multiple_choice",
  "true_false",
  "fill_blank",
  "short_answer",
];
const DEFAULT_TYPES: readonly QuestionType[] = ["single_choice", "true_false"];
const LEVELS = ["b1", "b2", "c1"] as const;
const SHARED = "shared";
const SOURCE_MIN = 40;
const TOPIC_MIN = 3;
const SOURCE_MAX = 20000;
const COUNT_MAX = 8;
const STYLES: readonly GenerateStyle[] = ["standalone", "passage"];
const SCOPES: readonly GenerateScope[] = ["section", "test"];

/** GENERATE_CEILING is the credits one Generate call reserves, the server's ceiling for the feature. */
export const GENERATE_CEILING = 200;
/** PLAN_QUESTIONS is the most questions one whole-test run asks for. */
export const PLAN_QUESTIONS = 40;
/** PLAN_CALLS is the most sections and passages one whole-test run writes for. */
export const PLAN_CALLS = 5;

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

function inBuilder(context: GenerateContext) {
  return (
    context.testId !== undefined &&
    context.destination.kind === "section" &&
    (context.sections?.length ?? 0) > 0
  );
}

function offeredSections(context: GenerateContext): readonly AiSection[] {
  return inBuilder(context) ? (context.sections ?? []) : [];
}

function chosenSection(values: GenerateValues, context: GenerateContext) {
  const offered = offeredSections(context);
  return offered.find((section) => section.id === values.sectionId) ?? offered[0];
}

function readable(section: AiSection) {
  return (
    section.title.trim() !== "" ||
    section.instructions === true ||
    (section.questions ?? 0) > 0
  );
}

function titleOf(section: AiSection, t: TFunction) {
  return section.title.trim() || t("ai.generate.untitled");
}

function passagesOf(section: AiSection, context: GenerateContext) {
  return context.passages.filter((passage) => passage.sectionId === section.id);
}

function passageLabel(passage: AiPassage, context: GenerateContext, t: TFunction) {
  return t("ai.generate.passage", {
    n: context.passages.indexOf(passage) + 1,
    title: passage.title,
  });
}

function sectionMeta(section: AiSection, context: GenerateContext, t: TFunction) {
  const passages = passagesOf(section, context).length;
  return [
    section.questions
      ? t("ai.generate.questionCount", { count: section.questions })
      : t("ai.generate.noQuestions"),
    passages > 0 ? t("ai.generate.passageCount", { count: passages }) : null,
    section.instructions ? t("ai.generate.hasInstructions") : null,
  ]
    .filter((part) => part !== null)
    .join(" · ");
}

function levelOf(entry: PlanEntry, values: GenerateValues): AiLevel {
  if (isLevel(entry.level)) return entry.level;
  return isLevel(values.level) ? values.level : "b2";
}

function entryFor(section: AiSection, values: GenerateValues): PlanEntry {
  return (
    values.plan[section.id] ?? {
      count: "0",
      mode: "passage",
      level: section.level ?? SHARED,
      types: initialTypes(section.types),
    }
  );
}

function planRows(values: GenerateValues, context: GenerateContext) {
  return offeredSections(context).map((section) => {
    const entry = entryFor(section, values);
    const passages = passagesOf(section, context);
    const blocked = !readable(section);
    const perPassage = passages.length > 0 && entry.mode === "passage";
    const count = Number(entry.count) || 0;
    const each = perPassage ? passages.length : 1;
    const calls = blocked || count === 0 ? 0 : each;
    const questions = blocked ? 0 : count * each;
    return { section, entry, passages, blocked, perPassage, count, calls, questions };
  });
}

function planTotals(values: GenerateValues, context: GenerateContext) {
  const rows = planRows(values, context);
  return {
    rows,
    questions: rows.reduce((sum, row) => sum + row.questions, 0),
    calls: rows.reduce((sum, row) => sum + row.calls, 0),
    sections: rows.filter((row) => row.questions > 0).length,
  };
}

function chosenPassage(values: GenerateValues, context: GenerateContext) {
  return values.style === "passage"
    ? context.passages.find((item) => item.text === values.source)
    : undefined;
}

function fitted(values: GenerateValues, context: GenerateContext) {
  return values.style === "standalone" && values.fitSection
    ? chosenSection(values, context)
    : undefined;
}

function destinationFor(
  values: GenerateValues,
  context: GenerateContext,
): GenerateDestination {
  if (values.style === "standalone") {
    const section = chosenSection(values, context);
    if (section) return { kind: "section", id: section.id, title: section.title };
  }
  const passage = chosenPassage(values, context);
  return passage?.group && context.destination.kind === "section"
    ? { kind: "group", id: passage.group.id, title: passage.group.title }
    : context.destination;
}

function firstPassage(context: GenerateContext) {
  const open = context.section;
  return (open ? passagesOf(open, context)[0] : undefined) ?? context.passages[0];
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
  const first = firstPassage(context);
  return {
    style: "passage",
    ...(values.source.trim() === "" && first ? { source: first.text } : {}),
  };
}

function topicProblem(values: GenerateValues, context: GenerateContext) {
  const topic = [...values.source.trim()].length;
  if (topic >= TOPIC_MIN) return null;
  if (topic > 0) return "ai.generate.needTopic";
  const section = fitted(values, context);
  if (!section) return "ai.generate.needTopic";
  return readable(section) ? null : "ai.generate.needTopicEmptySection";
}

function goneSection(items: readonly GenerateItem[], context: GenerateContext) {
  const kept = items[0]?.destination;
  return kept?.kind === "section" &&
    kept.id !== undefined &&
    !(context.sections ?? []).some((section) => section.id === kept.id)
    ? kept
    : undefined;
}

function destinationOf(
  items: readonly GenerateItem[],
  context: GenerateContext,
  values: GenerateValues,
): GenerateDestination {
  const gone = goneSection(items, context);
  if (gone) {
    const instead = (context.sections ?? []).find(
      (section) => section.id === values.retarget,
    );
    return instead ? { kind: "section", id: instead.id, title: instead.title } : gone;
  }
  const kept = items[0]?.destination;
  return kept?.kind === "group" || (kept?.kind === "section" && kept.id)
    ? kept
    : context.destination;
}

function settings(
  values: GenerateValues,
  language: AiGenerateRequest["language"],
  types: readonly string[],
  count: number,
  level: AiLevel,
) {
  return {
    types: TYPES.filter((type) => types.includes(type)),
    count,
    level,
    explanations: values.explanations,
    language,
  };
}

function request(
  values: GenerateValues,
  context: GenerateContext,
  language: AiGenerateRequest["language"],
): AiGenerateRequest {
  const passage = chosenPassage(values, context);
  const base = settings(
    values,
    language,
    values.types,
    Number(values.count),
    isLevel(values.level) ? values.level : "b2",
  );
  if (values.style === "standalone") {
    const topic = values.source.trim().slice(0, SOURCE_MAX);
    const section = fitted(values, context);
    return {
      ...base,
      standalone: true,
      ...(topic === "" ? {} : { source: topic }),
      ...(section && context.testId
        ? { testId: context.testId, sectionId: section.id }
        : {}),
    };
  }
  return passage && context.testId
    ? { ...base, testId: context.testId, stimulusId: passage.id }
    : { ...base, source: values.source.slice(0, SOURCE_MAX) };
}

function batchesOf(
  values: GenerateValues,
  context: GenerateContext,
  language: AiGenerateRequest["language"],
  t: TFunction,
): Batch[] {
  const testId = context.testId ?? "";
  return planTotals(values, context).rows.flatMap((row) => {
    if (row.calls === 0) return [];
    const base = settings(
      values,
      language,
      row.entry.types,
      row.count,
      levelOf(row.entry, values),
    );
    if (row.perPassage)
      return row.passages.map((passage): Batch => ({
        key: `${row.section.id}:${passage.id}`,
        section: row.section,
        body: { ...base, testId, stimulusId: passage.id },
        destination: passage.group
          ? { kind: "group" as const, id: passage.group.id, title: passage.group.title }
          : { kind: "section" as const, id: row.section.id, title: row.section.title },
        sub: {
          key: passage.id,
          title: passageLabel(passage, context, t),
          note: t("ai.generate.intoGroup"),
        },
      }));
    const batch: Batch = {
      key: row.section.id,
      section: row.section,
      body: { ...base, standalone: true, testId, sectionId: row.section.id },
      destination: {
        kind: "section" as const,
        id: row.section.id,
        title: row.section.title,
      },
    };
    return [batch];
  });
}

function partMeta(batches: readonly Batch[], t: TFunction) {
  const first = batches[0];
  if (!first) return "";
  const level = t(`bank.level.${first.body.level}`);
  return first.body.standalone
    ? `${t("ai.generate.styles.standalone")} · ${level}`
    : `${t("ai.generate.passageCount", { count: batches.length })} · ${t(
        "ai.generate.perPassage",
      ).toLowerCase()} · ${level}`;
}

function stateOf(settled: readonly (AiPart["state"] | undefined)[]): AiPart["state"] {
  if (settled.includes("running")) return "running";
  return settled.includes("failed") ? "failed" : "done";
}

async function runPlan(
  batches: readonly Batch[],
  values: GenerateValues,
  env: AiEnv<GenerateContext>,
  signal: AbortSignal,
  report: (parts: readonly AiPart[]) => void,
): Promise<AiRunResult<GenerateItem>> {
  const sections = [
    ...new Map(batches.map((batch) => [batch.section.id, batch.section])).values(),
  ];
  const order = new Map(
    (env.context.sections ?? []).map((section, index) => [section.id, index]),
  );
  const groups: AiGroup[] = sections.map((section) => ({
    key: section.id,
    title: titleOf(section, env.t),
    order: order.get(section.id) ?? 0,
  }));
  const states = new Map<string, "running" | "done" | "failed">(
    batches.map((batch) => [batch.key, "running"]),
  );
  const counts = new Map<string, number>();
  const tell = () =>
    report(
      sections.map((section) => {
        const own = batches.filter((batch) => batch.section.id === section.id);
        const settled = own.map((batch) => states.get(batch.key));
        const state = stateOf(settled);
        return {
          key: section.id,
          title: titleOf(section, env.t),
          meta: partMeta(own, env.t),
          state,
          count: own.reduce((sum, batch) => sum + (counts.get(batch.key) ?? 0), 0),
        };
      }),
    );
  tell();
  const settled = await Promise.allSettled(
    batches.map(async (batch) => {
      try {
        const result = await generateAiQuestions(batch.body, signal);
        states.set(batch.key, "done");
        counts.set(batch.key, result.items.length);
        return result;
      } catch (cause) {
        states.set(batch.key, "failed");
        throw cause;
      } finally {
        if (!signal.aborted) tell();
      }
    }),
  );
  if (signal.aborted) throw new DOMException("Aborted", "AbortError");
  const items: GenerateItem[] = [];
  let credits = 0;
  let left: number | null = null;
  const failedBatches: { batch: Batch; cause: unknown }[] = [];
  settled.forEach((outcome, index) => {
    const batch = batches[index];
    if (!batch) return;
    if (outcome.status === "rejected") {
      failedBatches.push({ batch, cause: outcome.reason });
      return;
    }
    credits += outcome.value.usage.credits;
    left =
      left === null
        ? outcome.value.usage.left
        : Math.min(left, outcome.value.usage.left);
    const group = groups.find((entry) => entry.key === batch.section.id);
    for (const question of outcome.value.items)
      items.push({
        question,
        level: batch.body.level,
        explanations: batch.body.explanations,
        destination: batch.destination,
        ...(group ? { group: batch.sub ? { ...group, sub: batch.sub } : group } : {}),
      });
  });
  const first = failedBatches[0];
  if (first && failedBatches.length === batches.length) throw first.cause;
  const failed: AiFailedPart<GenerateItem>[] = sections.flatMap((section) => {
    const own = failedBatches.filter(({ batch }) => batch.section.id === section.id);
    const reason = own[0];
    if (!reason) return [];
    const failure = aiFailureOf(reason.cause, env.t("ai.failed"));
    return [
      {
        key: section.id,
        ...(failure.kind === "error" ? { message: failure.message } : {}),
        max: own.length * GENERATE_CEILING,
        retry: (again: AbortSignal) =>
          runPlan(
            own.map(({ batch }) => batch),
            values,
            env,
            again,
            () => undefined,
          ),
      },
    ];
  });
  const standalone = items.some((item) => item.destination.kind === "section");
  return {
    items,
    usage: { credits, left: left ?? 0 },
    groups,
    failed,
    ...(standalone ? { notice: env.t("ai.generate.standaloneNotice") } : {}),
  };
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

function styleField(
  values: GenerateValues,
  set: (patch: Partial<GenerateValues>) => void,
  context: GenerateContext,
  t: TFunction,
): AiField {
  return {
    kind: "radios",
    label: t("ai.generate.style"),
    options: STYLES.map((style) => ({
      value: style,
      label: t(`ai.generate.styles.${style}`),
      sub: t(`ai.generate.styleSubs.${style}`),
    })),
    value: values.style,
    onChange: (style) => set(withStyle(style, values, context)),
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
                ...context.passages.map((passage) => ({
                  key: passage.id,
                  label: passageLabel(passage, context, t),
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
  const section = chosenSection(values, context);
  const optional = section && values.fitSection && readable(section);
  const topic: AiField = {
    kind: "source",
    label: t("ai.generate.topic"),
    ...(optional ? { optional: t("ai.generate.optional") } : {}),
    value: values.source,
    placeholder: t("ai.generate.topicPlaceholder"),
    rows: 3,
    sources: [],
    onChange: (source) => set({ source }),
  };
  if (!section) return [topic];
  return [
    {
      kind: "destination",
      label: t("ai.generate.destination"),
      change: t("ai.generate.changeSection"),
      changeLabel: t("ai.generate.changeSectionLabel", {
        section: titleOf(section, t),
      }),
      options: offeredSections(context).map((item) => ({
        value: item.id,
        label: titleOf(item, t),
        italic: item.title.trim() === "",
        sub: sectionMeta(item, context, t),
        ...(item.id === context.section?.id ? { badge: t("ai.generate.openNow") } : {}),
      })),
      value: section.id,
      onChange: (sectionId) => {
        const next = offeredSections(context).find((item) => item.id === sectionId);
        set({
          sectionId,
          ...(values.typesTouched || !next ? {} : { types: initialTypes(next.types) }),
        });
      },
      toggle: {
        text: t("ai.generate.fit"),
        sub: t(values.fitSection ? "ai.generate.fitSub" : "ai.generate.fitOffSub"),
        value: values.fitSection,
        onChange: (fitSection) => set({ fitSection }),
      },
    },
    topic,
  ];
}

function typeOptions(t: TFunction) {
  return TYPES.map((type) => ({
    value: type,
    label: t(`questionEditor.type.${type}`),
  }));
}

function levelOptions(t: TFunction) {
  return LEVELS.map((level) => ({ value: level, label: t(`bank.level.${level}`) }));
}

function sectionFields(
  values: GenerateValues,
  set: (patch: Partial<GenerateValues>) => void,
  context: GenerateContext,
  t: TFunction,
): AiField[] {
  return [
    styleField(values, set, context, t),
    ...sourceFields(values, set, context, t),
    {
      kind: "chips",
      label: t("ai.generate.types"),
      options: typeOptions(t),
      value: values.types,
      onChange: (types) => set({ types, typesTouched: true }),
    },
    {
      kind: "stepper",
      label: t("ai.generate.count"),
      hint: t("ai.generate.countRange"),
      fewer: t("ai.generate.fewer"),
      more: t("ai.generate.more"),
      value: values.count,
      min: 1,
      max: COUNT_MAX,
      onChange: (count) => set({ count }),
    },
    {
      kind: "seg",
      label: t("ai.generate.level"),
      options: levelOptions(t),
      value: values.level,
      onChange: (level) => set({ level }),
    },
  ];
}

function planFields(
  values: GenerateValues,
  set: (patch: Partial<GenerateValues>) => void,
  context: GenerateContext,
  t: TFunction,
): AiField[] {
  const totals = planTotals(values, context);
  const patch = (id: string, entry: PlanEntry, change: Partial<PlanEntry>) =>
    set({ plan: { ...values.plan, [id]: { ...entry, ...change } } });
  const shared = isLevel(values.level) ? t(`bank.level.${values.level}`) : "";
  const rows: AiPlanRow[] = totals.rows.map(
    ({ section, entry, passages, blocked, perPassage, count, questions }) => ({
      key: section.id,
      title: titleOf(section, t),
      italic: section.title.trim() === "",
      meta: [
        section.questions
          ? t("ai.generate.questionCount", { count: section.questions })
          : t("ai.generate.noQuestions"),
        ...(passages.length > 0
          ? [t("ai.generate.passageCount", { count: passages.length })]
          : []),
      ].join(" · "),
      count: entry.count,
      onCount: (value) => patch(section.id, entry, { count: value }),
      ...(blocked ? { blocked: t("ai.generate.planNeedsContext") } : {}),
      ...(perPassage ? { unit: t("ai.generate.perPassageUnit") } : {}),
      ...(passages.length > 0
        ? {
            mode: {
              label: t("ai.generate.style"),
              options: [
                { value: "passage", label: t("ai.generate.perPassage") },
                { value: "standalone", label: t("ai.generate.styles.standalone") },
              ],
              value: entry.mode,
              onChange: (mode: string) =>
                patch(section.id, entry, {
                  mode: mode === "standalone" ? "standalone" : "passage",
                }),
              calc: perPassage
                ? t("ai.generate.planPerPassage", {
                    passages: passages.length,
                    count,
                    total: questions,
                  })
                : t("ai.generate.planStandalone", { count }),
            },
          }
        : {}),
      types: {
        label: t("ai.generate.types"),
        hint: section.types?.length ? ` · ${t("ai.generate.typesFromSection")}` : "",
        options: typeOptions(t),
        value: entry.types,
        onChange: (types) => patch(section.id, entry, { types }),
      },
      level: {
        label: t("ai.generate.level"),
        hint:
          section.level !== undefined && entry.level === section.level
            ? ` · ${t("ai.generate.levelFromSection")}`
            : "",
        options: [
          { value: SHARED, label: t("ai.generate.levelUseShared", { level: shared }) },
          ...levelOptions(t),
        ],
        value: entry.level,
        onChange: (level) => patch(section.id, entry, { level }),
      },
    }),
  );
  return [
    {
      kind: "plan",
      label: t("ai.generate.plan"),
      sub: t("ai.generate.planSub"),
      head: {
        section: t("ai.generate.planSection"),
        count: t("ai.generate.planCount"),
      },
      fewer: t("ai.generate.fewer"),
      more: t("ai.generate.more"),
      max: COUNT_MAX,
      rows,
      total: t("ai.generate.planTotal", {
        count: totals.questions,
        sections: totals.sections,
      }),
      over: [
        ...(totals.questions > PLAN_QUESTIONS
          ? [t("ai.generate.overCount", { count: totals.questions })]
          : []),
        ...(totals.calls > PLAN_CALLS
          ? [t("ai.generate.overBatches", { count: totals.calls })]
          : []),
      ],
    },
    {
      kind: "seg",
      label: t("ai.generate.levelShared"),
      sub: t("ai.generate.levelSharedSub"),
      options: levelOptions(t),
      value: values.level,
      onChange: (level) => set({ level }),
    },
  ];
}

function sectionsOf(items: readonly GenerateItem[]) {
  return new Set(items.map((item) => item.group?.key ?? ""));
}

function titleOfGroup(items: readonly GenerateItem[]) {
  return items[0]?.group?.title ?? "";
}

function saveKey(destination: GenerateDestination) {
  if (destination.kind === "group") return `group:${destination.id}`;
  return `section:${destination.kind === "section" ? (destination.id ?? "") : ""}`;
}

function refusal(
  cause: unknown,
  first: GenerateItem,
  saved: readonly GenerateItem[],
  t: TFunction,
) {
  if (saved.length === 0) return cause;
  return new AiPartialWriteError(
    t("ai.generate.saveRefused", {
      section: first.group?.title ?? "",
      reason:
        cause instanceof AiWriteError
          ? cause.message
          : failureMessage(cause, t("ai.applyFailed")),
    }),
    saved,
  );
}

async function savePlan(
  selected: readonly GenerateItem[],
  { t, context, tagDrafts }: AiEnv<GenerateContext>,
) {
  const batches = new Map<string, GenerateItem[]>();
  const ordered = [...selected].sort(
    (a, b) => (a.group?.order ?? 0) - (b.group?.order ?? 0),
  );
  for (const item of ordered) {
    const key = saveKey(item.destination);
    batches.set(key, [...(batches.get(key) ?? []), item]);
  }
  const saved: GenerateItem[] = [];
  for (const items of batches.values()) {
    const first = items[0];
    if (!first) continue;
    try {
      await context.save(
        items.map(({ question, level }) => ({ question, level })),
        tagDrafts ? t("ai.draftTag") : null,
        first.destination,
      );
    } catch (cause) {
      throw refusal(cause, first, saved, t);
    }
    saved.push(...items);
  }
}

/**
 * generateKind is "Generate questions" (V-14a). For one section it writes
 * standalone questions about a topic or fitted to the section the run names,
 * or questions from a passage of the open test or pasted text, 1 to 8 at a
 * time, in the five stored types (TFNG waits for R6, DG-202). It starts from
 * a passage when the open section holds one and is standalone otherwise. In
 * the builder it can also fill the whole test from a plan, one call for each
 * section, or each passage of a section written per passage, at most
 * PLAN_CALLS calls and PLAN_QUESTIONS questions, each call reserving its own
 * GENERATE_CEILING; a section that fails can be tried again alone. Its
 * primary saves the kept questions through the page's write: in the builder
 * into the section or group each run named; into the bank; or into a new
 * practice test.
 */
export const generateKind: AiKind<GenerateValues, GenerateItem, GenerateContext> = {
  icon: Sparkles,
  titleKey: "ai.generate.title",
  subtitleKey: "ai.generate.subtitle",
  runLabelKey: "ai.generate.run",
  stepKeys: ["ai.generate.step1", "ai.generate.step2", "ai.generate.step3"],
  emptyKey: "ai.generate.empty",
  initial: (context) => {
    const open = offeredSections(context).find(
      (section) => section.id === context.section?.id,
    );
    const passage = open
      ? passagesOf(open, context).length > 0
      : context.passages.length > 0;
    return {
      scope: "section",
      style: passage ? "passage" : "standalone",
      fitSection: true,
      sectionId: open?.id ?? context.section?.id ?? "",
      source: passage ? (firstPassage(context)?.text ?? "") : "",
      types: initialTypes(open?.types?.length ? open.types : context.types),
      typesTouched: false,
      count: "5",
      level: "b2",
      explanations: true,
      plan: {},
      retarget: "",
    };
  },
  fields: (values, set, context, t) => [
    ...(inBuilder(context)
      ? [
          {
            kind: "seg" as const,
            label: t("ai.generate.scope"),
            hideLabel: true,
            options: SCOPES.map((scope) => ({
              value: scope,
              label: t(`ai.generate.scopes.${scope}`),
            })),
            value: values.scope,
            onChange: (scope: string) =>
              set({ scope: scope === "test" ? "test" : "section" }),
          },
        ]
      : []),
    ...(values.scope === "test"
      ? planFields(values, set, context, t)
      : sectionFields(values, set, context, t)),
    {
      kind: "toggle",
      text: t("ai.generate.explanations"),
      sub: t("ai.generate.explanationsSub"),
      value: values.explanations,
      onChange: (explanations) => set({ explanations }),
    },
  ],
  problem: (values, context) => {
    if (values.scope === "test") {
      const totals = planTotals(values, context);
      if (totals.sections === 0) return "ai.generate.planEmpty";
      if (totals.questions > PLAN_QUESTIONS || totals.calls > PLAN_CALLS)
        return "ai.generate.overLimit";
      if (totals.rows.some((row) => row.calls > 0 && row.entry.types.length === 0))
        return "ai.generate.needType";
      return null;
    }
    if (values.style === "standalone") {
      const missing = topicProblem(values, context);
      if (missing) return missing;
    } else if (values.source.trim().length < SOURCE_MIN)
      return "ai.generate.needSource";
    if (values.count === "") return "ai.generate.needCount";
    if (values.types.length === 0) return "ai.generate.needType";
    return null;
  },
  steps: (values, context, t) => {
    if (values.style !== "standalone")
      return [t("ai.generate.step1"), t("ai.generate.step2"), t("ai.generate.step3")];
    const section = fitted(values, context);
    return [
      section
        ? t("ai.generate.stepSection", { section: titleOf(section, t) })
        : t("ai.generate.stepTopic"),
      t("ai.generate.step2"),
      t("ai.generate.stepRepeats"),
    ];
  },
  runLabel: (values, context, t) =>
    values.scope === "test"
      ? t("ai.generate.runCount", { count: planTotals(values, context).questions })
      : t("ai.generate.run"),
  empty: (values, t) => {
    if (values.scope === "test") return t("ai.generate.allFailed");
    return t(
      values.style === "standalone"
        ? "ai.generate.emptyStandalone"
        : "ai.generate.empty",
    );
  },
  ceiling: (values, context) =>
    values.scope === "test"
      ? {
          max: Math.max(1, planTotals(values, context).calls) * GENERATE_CEILING,
          plan: true,
        }
      : { max: GENERATE_CEILING, plan: false },
  run: async (values, env, signal, report) => {
    if (values.scope === "test")
      return runPlan(
        batchesOf(values, env.context, env.language, env.t),
        values,
        env,
        signal,
        report,
      );
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
      ...(values.style === "standalone"
        ? { notice: env.t("ai.generate.standaloneNotice") }
        : {}),
    };
  },
  card: (item, env) => card(item, env.t, env.tagDrafts),
  group: (item) => item.group ?? { key: "", title: "", order: 0 },
  resultFields: (values, set, items, context, t) => {
    if (values.scope === "test") return [];
    const gone = goneSection(items, context);
    if (!gone) return [];
    return [
      {
        kind: "alert",
        text: t("ai.generate.sectionGone", {
          section: gone.title || t("ai.generate.untitled"),
        }),
        choose: {
          label: t("ai.generate.chooseSection"),
          options: (context.sections ?? []).map((section) => ({
            value: section.id,
            label: titleOf(section, t),
          })),
          value: values.retarget,
          onChange: (retarget) => set({ retarget }),
        },
      },
    ];
  },
  summary: (items, selected, t, values) =>
    values.scope === "test"
      ? t("ai.generate.summaryTest", {
          n: selected,
          total: items.length,
          sections: sectionsOf(items).size,
        })
      : t("ai.generate.summary", { n: selected, total: items.length }),
  primary: {
    icon: ({ context }, items, values) =>
      values.scope === "test"
        ? Plus
        : PRIMARY_ICONS[destinationOf(items, context, values).kind],
    blocked: (items, { context }, values) =>
      values.scope !== "test" &&
      goneSection(items, context) !== undefined &&
      !(context.sections ?? []).some((section) => section.id === values.retarget),
    label: (selected, { t, context }, items, values) => {
      const count = selected.length;
      if (values.scope === "test") {
        const sections = sectionsOf(selected);
        return sections.size === 1
          ? t("ai.generate.addTo", { count, section: titleOfGroup(selected) })
          : t("ai.generate.addToSections", { count, sections: sections.size });
      }
      const destination = destinationOf(items, context, values);
      switch (destination.kind) {
        case "practice":
          return t("ai.generate.saveAsPractice");
        case "bank":
          return t("ai.generate.saveToBank", { count });
        case "group":
          return t("ai.generate.addToGroup", { count, group: destination.title });
        case "section":
          return t("ai.generate.addTo", {
            count,
            section: destination.title || t("ai.generate.untitled"),
          });
      }
    },
    apply: async (selected, env, values) => {
      const { t, context, tagDrafts } = env;
      const count = selected.length;
      if (values.scope === "test") {
        await savePlan(selected, env);
        const sections = sectionsOf(selected);
        return sections.size === 1
          ? t("ai.generate.added", { count, section: titleOfGroup(selected) })
          : t("ai.generate.addedToSections", { count, sections: sections.size });
      }
      const destination = destinationOf(selected, context, values);
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
          return t("ai.generate.added", {
            count,
            section: destination.title || t("ai.generate.untitled"),
          });
      }
    },
  },
};
