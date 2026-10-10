import { questionGaps } from "@/components/shared/content/gaps";
import type { QuestionType, QuestionValues } from "./questionSchema";

type Option = QuestionValues["options"][number];

/** QUESTION_TYPES lists §7's five types in the order the editor offers them. */
export const QUESTION_TYPES: readonly QuestionType[] = [
  "single_choice",
  "multiple_choice",
  "true_false",
  "fill_blank",
  "short_answer",
];

/** optionLetter is the letter the deck names an option by: A for the first, H for the eighth. */
export function optionLetter(index: number): string {
  return String.fromCharCode(65 + index);
}

/** CHOICE_TYPES are the types whose answer is a set of marked options. */
export const CHOICE_TYPES: ReadonlySet<QuestionType> = new Set([
  "single_choice",
  "multiple_choice",
  "true_false",
]);

/**
 * typeLocked says whether the question must stay fill in the blank: its rich
 * prompt holds a gap, or a fill-in-the-blank rich prompt holds bound answers.
 * Leaving the type would orphan them, so the type controls disable the others.
 */
export function typeLocked(
  value: Pick<QuestionValues, "type" | "promptContent" | "blanks">,
): boolean {
  if (value.promptContent == null) return false;
  if (questionGaps(value.promptContent).length > 0) return true;
  return value.type === "fill_blank" && value.blanks.length > 0;
}

/**
 * trueFalseOptions are a true/false question's two options in their stored,
 * canonical form: "True" first, "False" second, with `trueIsCorrect` saying
 * which one is ticked.
 */
export function trueFalseOptions(trueIsCorrect: boolean): Option[] {
  return [
    { id: null, text: "True", isCorrect: trueIsCorrect },
    { id: null, text: "False", isCorrect: !trueIsCorrect },
  ];
}

/**
 * retype changes a question's type and keeps what still applies: the prompt,
 * the explanation, media, audio, points and tags always; the options between
 * single and multiple choice, where single keeps only the first tick; true /
 * false rebuilt as its two canonical options; the blanks only for fill in the
 * blank and the sample answer only for short answer.
 */
export function retype(value: QuestionValues, type: QuestionType): QuestionValues {
  return {
    ...value,
    type,
    options: optionsFor(value, type),
    blanks: type === "fill_blank" ? value.blanks : [],
    sampleAnswer: type === "short_answer" ? value.sampleAnswer : null,
  };
}

function optionsFor(value: QuestionValues, type: QuestionType): Option[] {
  const carried =
    value.type === "single_choice" || value.type === "multiple_choice"
      ? value.options
      : [];
  if (type === "true_false") {
    const first = value.type === "true_false" ? value.options[0] : undefined;
    return trueFalseOptions(first?.isCorrect ?? true);
  }
  if (type === "multiple_choice") return carried.length > 0 ? carried : blankOptions();
  if (type === "single_choice") {
    if (carried.length === 0) return blankOptions();
    const first = carried.findIndex((option) => option.isCorrect);
    return carried.map((option, index) => ({ ...option, isCorrect: index === first }));
  }
  return [];
}

function blankOptions(): Option[] {
  return [
    { id: null, text: "", isCorrect: true },
    { id: null, text: "", isCorrect: false },
  ];
}
