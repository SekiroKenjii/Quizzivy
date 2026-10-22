import { questionGaps, gapBindingsMatch } from "@/components/shared/content/gaps";
import {
  questionContentSchema,
  questionPromptContentSchema,
  isQuestionPromptContent,
} from "@/components/shared/content/questionContent";
import { optionContentSchema } from "@/components/shared/content/optionContent";
import { contentPlainText } from "@/components/shared/content/plainText";
import { z } from "zod";
import type { components } from "@/lib/api/schema";
import { comparePlaceholders, hasMismatch } from "./placeholders";

/** MAX_OPTIONS is the most options a choice question takes; the domain's cap. */
export const MAX_OPTIONS = 8;

/** The bounds R6 holds the new types to (76-r6.md, DG-113), as the server's domain does. */
export const MAX_MATCH_OPTIONS = 12;
export const MIN_MATCH_ITEMS = 2;
export const MAX_MATCH_ITEMS = 20;
export const MAX_MATCH_ITEM_LENGTH = 500;
export const MAX_RUBRIC_CRITERIA = 6;
export const MAX_RUBRIC_LABEL = 80;
export const MIN_SELECT_COUNT = 2;
export const MAX_SELECT_COUNT = 7;
export const MAX_WORDS_LIMIT = 100;

/** Form input validation for §7's question editor. */
const audioPolicySchema = z.object({
  maxPlays: z.number().int().min(1).nullable(),
  allowSeek: z.boolean(),
  showTranscriptAfterSubmit: z.boolean(),
  allowPause: z.boolean().exactOptional(),
});

const optionSchema = z
  .object({
    id: z.uuid().nullable(),
    text: z
      .string()
      .refine((text) => text.trim().length > 0, "questionEditor.errors.optionRequired"),
    isCorrect: z.boolean(),
    content: optionContentSchema.nullable().optional(),
  })
  .refine(
    (option) =>
      option.content == null || contentPlainText(option.content) === option.text,
    { message: "questionEditor.errors.optionContent", path: ["content"] },
  )
  .transform(({ content, ...option }) => ({
    ...option,
    ...(content === undefined ? {} : { content }),
  }));

const blankSchema = z
  .object({
    id: z.uuid().nullable(),
    gapId: z
      .string()
      .regex(/^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$/, "questionEditor.errors.gapMismatch")
      .nullable()
      .optional(),
    ordinal: z
      .number()
      .int()
      .min(1, "questionEditor.errors.blankOrdinal")
      .max(32767, "questionEditor.errors.blankOrdinal"),
    acceptedAnswers: z
      .array(
        z
          .string()
          .refine(
            (text) => text.trim().length > 0,
            "questionEditor.errors.answerRequired",
          ),
      )
      .min(1, "questionEditor.errors.answerRequired"),
    caseSensitive: z.boolean(),
  })
  .transform(({ gapId, ...blank }) => ({
    ...blank,
    ...(gapId === undefined ? {} : { gapId }),
  }));

const matchItemSchema = z.object({
  id: z.uuid().nullable(),
  text: z
    .string()
    .refine((text) => text.trim().length > 0, "questionEditor.errors.matchItemRequired")
    .refine(
      (text) => [...text].length <= MAX_MATCH_ITEM_LENGTH,
      "questionEditor.errors.matchItemTooLong",
    ),
  /** The 0-based index into `options`; -1 while the teacher has not chosen one. */
  correctOption: z.number().int(),
});

const rubricCriterionSchema = z.object({
  id: z.uuid().nullable(),
  label: z
    .string()
    .refine((text) => text.trim().length > 0, "questionEditor.errors.criterionRequired")
    .refine(
      (text) => [...text].length <= MAX_RUBRIC_LABEL,
      "questionEditor.errors.criterionTooLong",
    ),
});

export const questionSchema = z
  .object({
    type: z.enum([
      "single_choice",
      "multiple_choice",
      "true_false",
      "fill_blank",
      "short_answer",
      "tfng",
      "matching",
      "essay",
    ]),
    level: z
      .enum(["pre_a1", "a1", "a2", "b1", "b2", "c1", "c2"])
      .nullable()
      .default(null),
    skill: z
      .enum(["grammar", "vocabulary", "reading", "listening", "writing", "speaking"])
      .nullable()
      .default(null),
    promptContent: questionPromptContentSchema.nullable().optional(),
    explanationContent: questionContentSchema.nullable().optional(),
    prompt: z
      .string()
      .refine((text) => text.trim().length > 0, "questionEditor.errors.promptRequired"),
    mediaAssetId: z.uuid().nullable(),
    mediaAlt: z.string().nullable().default(null),
    audio: audioPolicySchema.nullable(),
    transcript: z.string().nullable(),
    options: z
      .array(optionSchema)
      .max(MAX_MATCH_OPTIONS, "questionEditor.errors.maxMatchOptions"),
    blanks: z.array(blankSchema),
    points: z
      .number()
      .gt(0, "questionEditor.pointsError")
      .max(999999.99, "questionEditor.errors.pointsTooLarge")
      .multipleOf(0.01, "questionEditor.errors.pointPrecision"),
    explanation: z.string().nullable(),
    sampleAnswer: z.string().nullable(),
    selectCount: z.number().int().nullable().exactOptional(),
    maxWords: z.number().int().nullable().exactOptional(),
    matchAllowReuse: z.boolean().nullable().exactOptional(),
    matchItems: z.array(matchItemSchema).exactOptional(),
    rubric: z.array(rubricCriterionSchema).exactOptional(),
    tags: z.array(z.string().min(1)),
  })
  .superRefine((value, context) => {
    validateStructure(value, context);
    for (const [field, text] of [
      ["promptContent", value.prompt],
      ["explanationContent", value.explanation],
    ] as const) {
      const document = value[field];
      if (document != null && (text == null || contentPlainText(document) !== text))
        context.addIssue({
          code: "custom",
          path: [field],
          message: "questionEditor.errors.questionContent",
        });
    }
  })
  .transform(({ promptContent, explanationContent, ...value }) => ({
    ...value,
    ...(promptContent === undefined ? {} : { promptContent }),
    ...(explanationContent === undefined ? {} : { explanationContent }),
  }));

export type QuestionValues = z.infer<typeof questionSchema>;

/** issueKey is the translation key of a failed parse's first issue, or fallback when that issue carries zod's own untranslated text. */
export function issueKey(error: z.ZodError, fallback: string): string {
  const message = error.issues[0]?.message;
  return message?.startsWith("questionEditor.") ? message : fallback;
}

const CHOICE_TYPES = new Set<string>([
  "single_choice",
  "multiple_choice",
  "true_false",
  "tfng",
]);

function deckBlocker(values: QuestionValues): string | null {
  const errors = "questionEditor.errors";
  if (values.prompt.trim() === "") return `${errors}.promptRequired`;
  if (!Number.isFinite(values.points) || values.points <= 0)
    return "questionEditor.pointsError";
  if (values.type === "fill_blank") {
    if (values.blanks.length === 0) return `${errors}.blankRequired`;
    const unanswered = values.blanks.some(
      (blank) => !blank.acceptedAnswers.some((answer) => answer.trim() !== ""),
    );
    if (unanswered) return `${errors}.answerRequired`;
  }
  if (CHOICE_TYPES.has(values.type)) return choiceBlocker(values);
  if (values.type === "matching") return matchingBlocker(values);
  return null;
}

function choiceBlocker(values: QuestionValues): string | null {
  const errors = "questionEditor.errors";
  if (values.options.length < 2) return `${errors}.twoOptions`;
  if (values.options.some((option) => option.text.trim() === ""))
    return `${errors}.optionRequired`;
  if (!values.options.some((option) => option.isCorrect))
    return `${errors}.correctRequired`;
  if (values.type === "multiple_choice" && values.selectCount != null) {
    const ticked = values.options.filter((option) => option.isCorrect).length;
    if (ticked !== values.selectCount) return `${errors}.selectCountTicks`;
  }
  return null;
}

function matchingBlocker(values: QuestionValues): string | null {
  const errors = "questionEditor.errors";
  const items = values.matchItems ?? [];
  if (items.length < MIN_MATCH_ITEMS) return `${errors}.matchTwoItems`;
  if (items.some((item) => item.text.trim() === ""))
    return `${errors}.matchItemRequired`;
  if (values.options.length < 2) return `${errors}.twoOptions`;
  if (values.options.some((option) => option.text.trim() === ""))
    return `${errors}.optionRequired`;
  if (items.some((item) => !inRange(item.correctOption, values.options.length)))
    return `${errors}.matchItemUnset`;
  return null;
}

function inRange(index: number, length: number): boolean {
  return Number.isInteger(index) && index >= 0 && index < length;
}

/**
 * blockingIssue is the translation key of what stops `values` from saving, or
 * null when they parse. The deck's seven checks come first, in its order: the
 * question, the points, a blank, an answer per blank, two options, every
 * option filled, a correct answer. Any other failed rule follows as the
 * parse's first translated issue, else as fallback.
 */
export function blockingIssue(values: QuestionValues, fallback: string): string | null {
  const parsed = questionSchema.safeParse(values);
  if (parsed.success) return null;
  return deckBlocker(values) ?? issueKey(parsed.error, fallback);
}

export type QuestionType = QuestionValues["type"];

type StructureInput = Pick<
  QuestionValues,
  | "type"
  | "options"
  | "blanks"
  | "prompt"
  | "selectCount"
  | "maxWords"
  | "matchAllowReuse"
  | "matchItems"
  | "rubric"
> & {
  promptContent?: QuestionValues["promptContent"];
};

function validateStructure(value: StructureInput, context: z.RefinementCtx) {
  const issue = (path: string, key: string) =>
    context.addIssue({
      code: "custom",
      path: [path],
      message: `questionEditor.errors.${key}`,
    });
  if (value.promptContent != null && !isQuestionPromptContent(value.promptContent))
    return;
  if (["single_choice", "multiple_choice", "true_false"].includes(value.type)) {
    validateChoice(value, issue);
  } else if (value.type === "tfng") {
    validateTfng(value, issue);
  } else if (value.type === "matching") {
    validateMatching(value, issue);
  } else if (value.options.length) issue("options", "unexpectedOptions");
  validateTypeFields(value, issue);
  if (value.type !== "fill_blank") {
    if (value.blanks.length) issue("blanks", "unexpectedBlanks");
    if (value.promptContent && questionGaps(value.promptContent).length)
      issue("promptContent", "questionContent");
    return;
  }
  validateBlankStructure(value, issue);
}

/**
 * validateTypeFields holds each R6 setting to the one type it belongs to and
 * to the bounds the server's domain holds it to: "choose exactly N" on a
 * multiple choice, the word limit on a short answer, the reuse switch on a
 * matching question (which needs it) and the rubric on an essay.
 */
function validateTypeFields(
  value: StructureInput,
  issue: (path: string, key: string) => void,
) {
  if (value.selectCount != null) {
    if (value.type !== "multiple_choice") issue("selectCount", "unexpectedTypeField");
    else validateSelectCount(value.selectCount, value.options, issue);
  }
  if (value.maxWords != null) validateMaxWords(value.type, value.maxWords, issue);
  if (value.type === "matching" && value.matchAllowReuse == null)
    issue("matchAllowReuse", "matchReuseRequired");
  if (value.type !== "matching") {
    if (value.matchAllowReuse != null) issue("matchAllowReuse", "unexpectedTypeField");
    if (value.matchItems?.length) issue("matchItems", "unexpectedTypeField");
  }
  if (value.type === "essay") validateRubric(value.rubric ?? [], issue);
  else if (value.rubric?.length) issue("rubric", "unexpectedTypeField");
}

function validateMaxWords(
  type: QuestionType,
  maxWords: number,
  issue: (path: string, key: string) => void,
) {
  if (type !== "short_answer") issue("maxWords", "unexpectedTypeField");
  else if (!Number.isInteger(maxWords) || maxWords < 1 || maxWords > MAX_WORDS_LIMIT)
    issue("maxWords", "maxWordsRange");
}

function validateSelectCount(
  count: number,
  options: QuestionValues["options"],
  issue: (path: string, key: string) => void,
) {
  if (
    !Number.isInteger(count) ||
    count < MIN_SELECT_COUNT ||
    count > MAX_SELECT_COUNT ||
    count > options.length - 1
  ) {
    issue("selectCount", "selectCountRange");
    return;
  }
  if (options.filter((option) => option.isCorrect).length !== count)
    issue("options", "selectCountTicks");
}

function validateTfng(
  value: Pick<QuestionValues, "options">,
  issue: (path: string, key: string) => void,
) {
  if (value.options.length !== 3) issue("options", "tfngCount");
  const correct = value.options.filter((option) => option.isCorrect).length;
  if (correct === 0) issue("options", "correctRequired");
  else if (correct > 1) issue("options", "oneCorrect");
}

/** MatchingProblems names what is wrong with a matching question's items and options. */
export interface MatchingProblems {
  /** The indexes of the items with no option chosen, or one that is gone. */
  unset: ReadonlySet<number>;
  /** Without reuse, the indexes of the items whose option an earlier item already took. */
  taken: ReadonlySet<number>;
  /** Without reuse, more items than options: some item cannot have its own. */
  moreItemsThanOptions: boolean;
}

/**
 * matchingProblems is the one reading of a matching question's keys that the
 * schema and the matching editor share.
 */
export function matchingProblems(
  value: Pick<QuestionValues, "options" | "matchItems" | "matchAllowReuse">,
): MatchingProblems {
  const items = value.matchItems ?? [];
  const reuse = value.matchAllowReuse === true;
  const unset = new Set<number>();
  const taken = new Set<number>();
  const seen = new Set<number>();
  items.forEach((item, index) => {
    if (!inRange(item.correctOption, value.options.length)) {
      unset.add(index);
      return;
    }
    if (!reuse && seen.has(item.correctOption)) taken.add(index);
    seen.add(item.correctOption);
  });
  return {
    unset,
    taken,
    moreItemsThanOptions: !reuse && items.length > value.options.length,
  };
}

function validateMatching(
  value: StructureInput,
  issue: (path: string, key: string) => void,
) {
  const items = value.matchItems ?? [];
  if (value.options.length < 2) issue("options", "twoOptions");
  if (value.options.some((option) => option.isCorrect))
    issue("options", "matchOptionCorrect");
  if (items.length < MIN_MATCH_ITEMS) issue("matchItems", "matchTwoItems");
  if (items.length > MAX_MATCH_ITEMS) issue("matchItems", "maxMatchItems");
  const problems = matchingProblems(value);
  if (problems.moreItemsThanOptions) issue("matchItems", "matchMoreItemsThanOptions");
  if (problems.unset.size) issue("matchItems", "matchItemUnset");
  if (problems.taken.size) issue("matchItems", "matchOptionTaken");
}

function validateRubric(
  rubric: NonNullable<QuestionValues["rubric"]>,
  issue: (path: string, key: string) => void,
) {
  if (rubric.length > MAX_RUBRIC_CRITERIA) issue("rubric", "maxCriteria");
  const labels = rubric
    .map((criterion) => criterion.label.trim().toLowerCase())
    .filter((label) => label !== "");
  if (new Set(labels).size !== labels.length) issue("rubric", "duplicateCriterion");
}

function validateBlankStructure(
  value: Pick<QuestionValues, "blanks" | "prompt"> & {
    promptContent?: QuestionValues["promptContent"];
  },
  issue: (path: string, key: string) => void,
) {
  if (!value.blanks.length) issue("blanks", "blankRequired");
  const ordinals = value.blanks.map((blank) => blank.ordinal);
  if (new Set(ordinals).size !== ordinals.length) issue("blanks", "duplicateBlank");
  if (value.promptContent != null) {
    if (!gapBindingsMatch(value.promptContent, value.blanks))
      issue("blanks", "gapMismatch");
    else if (gapProblems(value.promptContent, value.blanks).emptyGapLabels.length)
      issue("blanks", "answerRequired");
  } else if (
    value.blanks.some((blank) => blank.gapId != null) ||
    hasMismatch(comparePlaceholders(value.prompt, ordinals))
  )
    issue("blanks", "placeholderMismatch");
}

/** ChoiceProblems names what is wrong, or merely unusual, with a choice question's options. */
export interface ChoiceProblems {
  tooFew: boolean;
  tooMany: boolean;
  noneCorrect: boolean;
  /** A multiple-choice question with one tick: a warning, never a blocking rule. */
  oneCorrectOfMany: boolean;
  manyCorrectOfOne: boolean;
}

/**
 * choiceProblems is the one reading of a choice question's options that the
 * schema and the options editor share. Only `oneCorrectOfMany` does not block
 * saving: the server takes a multiple-choice question with one correct answer.
 */
export function choiceProblems(
  value: Pick<QuestionValues, "type" | "options">,
): ChoiceProblems {
  const correct = value.options.filter((option) => option.isCorrect).length;
  const multiple = value.type === "multiple_choice";
  return {
    tooFew: value.options.length < 2,
    tooMany: value.options.length > MAX_OPTIONS,
    noneCorrect: correct === 0,
    oneCorrectOfMany: multiple && correct === 1,
    manyCorrectOfOne: !multiple && correct > 1,
  };
}

/**
 * gapProblems lists, in prompt order, the labels of the gaps in a rich prompt
 * whose answer row has no accepted answer that says something.
 */
export function gapProblems(
  content: NonNullable<QuestionValues["promptContent"]>,
  blanks: ReadonlyArray<{ gapId?: string | null; acceptedAnswers: readonly string[] }>,
): { emptyGapLabels: string[] } {
  const answered = new Set(
    blanks
      .filter((blank) => blank.acceptedAnswers.some((answer) => answer.trim() !== ""))
      .map((blank) => blank.gapId),
  );
  return {
    emptyGapLabels: questionGaps(content)
      .filter((gap) => !answered.has(gap.id))
      .map((gap) => gap.label),
  };
}

function validateChoice(
  value: Pick<QuestionValues, "type" | "options">,
  issue: (path: string, key: string) => void,
) {
  const problems = choiceProblems(value);
  if (problems.tooFew) issue("options", "twoOptions");
  if (problems.tooMany) issue("options", "maxOptions");
  if (value.type === "true_false" && value.options.length !== 2)
    issue("options", "trueFalseCount");
  if (problems.noneCorrect) issue("options", "correctRequired");
  if (problems.manyCorrectOfOne) issue("options", "oneCorrect");
}

/** A blank single-choice question -- what /teacher/question-bank/new starts from. */
export function emptyQuestion(): QuestionValues {
  return {
    type: "single_choice",
    level: null,
    skill: null,
    prompt: "",
    mediaAssetId: null,
    mediaAlt: null,
    audio: null,
    transcript: null,
    options: [
      { id: null, text: "", isCorrect: true },
      { id: null, text: "", isCorrect: false },
    ],
    blanks: [],
    points: 1,
    explanation: null,
    sampleAnswer: null,
    tags: [],
  };
}

/** Fails `tsc` if the form produces something the endpoint would not accept. */
type QuestionInput = components["schemas"]["QuestionInput"];

type Expect<T extends true> = T;
type AssignableTo<A, B> = A extends B ? true : false;

// Exported so `noUnusedLocals` does not remove the only thing keeping the form
// and the contract in step. Nothing reads it; `tsc -b` checking it is the job.
export type QuestionValuesAreAcceptedByTheContract = Expect<
  AssignableTo<QuestionValues, QuestionInput>
>;
