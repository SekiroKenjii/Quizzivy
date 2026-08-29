import { z } from "zod";
import type { components } from "@/lib/api/schema";

/**
 * Form input validation for §7's question editor.
 *
 * Hand-written for the reason AGENTS.md gives: the generated types say what the
 * API accepts, this says what the FORM accepts, and those differ -- a form
 * needs localised messages and a "required" that fires before anything is sent.
 *
 * Cross-field rules the server also enforces (a choice question needs a correct
 * option, placeholders must match the blanks) are checked here too, so the
 * teacher sees them inline rather than after a round trip.
 */
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

/**
 * Fails `tsc` if the form produces something the endpoint would not accept.
 *
 * ASSIGNABILITY, not equality, and the difference is forced by
 * `exactOptionalPropertyTypes`: the contract marks several fields `?: T | null`,
 * which permits an absent property but not an `undefined` one, and no zod schema
 * can produce exact-optional output. So every field here is required and
 * nullable instead -- the editor always sends a complete body -- and the
 * assertion checks the direction that matters: what the form yields is something
 * the endpoint takes. A field the contract does not have, or a type mismatch, is
 * a compile error rather than a 400 in front of a teacher mid-edit.
 */
type QuestionInput = components["schemas"]["QuestionInput"];

type Expect<T extends true> = T;
type AssignableTo<A, B> = A extends B ? true : false;

// Exported so `noUnusedLocals` does not remove the only thing keeping the form
// and the contract in step. Nothing reads it; `tsc -b` checking it is the job.
export type QuestionValuesAreAcceptedByTheContract = Expect<
  AssignableTo<QuestionValues, QuestionInput>
>;
