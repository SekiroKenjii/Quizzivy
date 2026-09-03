import { z } from "zod";
import type { components } from "@/lib/api/schema";

/** Form input validation for §7's question editor. */
const audioPolicySchema = z.object({
  maxPlays: z.number().int().min(1).nullable(),
  allowSeek: z.boolean(),
  showTranscriptAfterSubmit: z.boolean(),
});

const optionSchema = z.object({
  id: z.string().uuid().nullable(),
  text: z.string().min(1, "questionEditor.errors.optionRequired"),
  isCorrect: z.boolean(),
});

const blankSchema = z.object({
  id: z.string().uuid().nullable(),
  ordinal: z.number().int().min(1),
  acceptedAnswers: z.array(z.string().min(1)).min(1),
  caseSensitive: z.boolean(),
});

export const questionSchema = z.object({
  type: z.enum([
    "single_choice",
    "multiple_choice",
    "true_false",
    "fill_blank",
    "short_answer",
  ]),
  prompt: z.string().min(1, "questionEditor.errors.promptRequired"),
  mediaAssetId: z.string().uuid().nullable(),
  audio: audioPolicySchema.nullable(),
  transcript: z.string().nullable(),
  options: z.array(optionSchema),
  blanks: z.array(blankSchema),
  points: z.number().gt(0, "questionEditor.pointsError").max(999999.99),
  explanation: z.string().nullable(),
  sampleAnswer: z.string().nullable(),
  tags: z.array(z.string().min(1)),
});

export type QuestionValues = z.infer<typeof questionSchema>;

export type QuestionType = QuestionValues["type"];

/** A blank single-choice question -- what /admin/question-bank/new starts from. */
export function emptyQuestion(): QuestionValues {
  return {
    type: "single_choice",
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
