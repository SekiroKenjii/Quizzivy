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

/** MAX_OPTIONS is the most options a choice question takes, as the contract's maxItems. */
export const MAX_OPTIONS = 8;

/** Form input validation for §7's question editor. */
const audioPolicySchema = z.object({
  maxPlays: z.number().int().min(1).nullable(),
  allowSeek: z.boolean(),
  showTranscriptAfterSubmit: z.boolean(),
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

export const questionSchema = z
  .object({
    type: z.enum([
      "single_choice",
      "multiple_choice",
      "true_false",
      "fill_blank",
      "short_answer",
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
    audio: audioPolicySchema.nullable(),
    transcript: z.string().nullable(),
    options: z.array(optionSchema).max(MAX_OPTIONS, "questionEditor.errors.maxOptions"),
    blanks: z.array(blankSchema),
    points: z
      .number()
      .gt(0, "questionEditor.pointsError")
      .max(999999.99, "questionEditor.errors.pointsTooLarge")
      .multipleOf(0.01, "questionEditor.errors.pointPrecision"),
    explanation: z.string().nullable(),
    sampleAnswer: z.string().nullable(),
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

export type QuestionType = QuestionValues["type"];

function validateStructure(
  value: Pick<QuestionValues, "type" | "options" | "blanks" | "prompt"> & {
    promptContent?: QuestionValues["promptContent"];
  },
  context: z.RefinementCtx,
) {
  const issue = (path: string, key: string) =>
    context.addIssue({
      code: "custom",
      path: [path],
      message: `questionEditor.errors.${key}`,
    });
  if (value.promptContent != null && !isQuestionPromptContent(value.promptContent))
    return;
  const choice = ["single_choice", "multiple_choice", "true_false"].includes(
    value.type,
  );
  if (choice) {
    validateChoice(value, issue);
  } else if (value.options.length) issue("options", "unexpectedOptions");
  if (value.type !== "fill_blank") {
    if (value.blanks.length) issue("blanks", "unexpectedBlanks");
    if (value.promptContent && questionGaps(value.promptContent).length)
      issue("promptContent", "questionContent");
    return;
  }
  validateBlankStructure(value, issue);
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
