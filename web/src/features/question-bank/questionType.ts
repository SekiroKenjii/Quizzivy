import { questionGaps } from "@/components/shared/content/gaps";
import { plainOptionContent } from "@/components/shared/content/optionContent";
import { trueFalseLabelKey } from "@/components/shared/content/trueFalse";
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

/** TrueFalseAnswer is a true/false question's stored options read as an answer. */
export interface TrueFalseAnswer {
  trueIsCorrect: boolean;
  trueOption: Option | undefined;
  falseOption: Option | undefined;
}

/**
 * readTrueFalse reads a true/false question's answer by its texts, never by
 * position: the Word import keeps a question's own order, so "False" can come
 * first. The canonical texts decide first, by the test trueFalseLabelKey
 * applies: the option stored as exactly "True" is the true one, or else the
 * one beside an exact "False". Failing that, a truth word decides, the import's
 * set and their English and Vietnamese yes and no ("Đúng", "Sai", "T", "F",
 * "Yes", "No", "Có", "Không", in any case). Only when no text says which is
 * which is the first option taken as true. With no options, "True" is correct.
 */
export function readTrueFalse(options: readonly Option[]): TrueFalseAnswer {
  const canonical = options.map((option) => {
    const key = trueFalseLabelKey("true_false", option.text, option.content ?? null);
    return key === null ? null : key === "trueFalse.true";
  });
  const trueIndex =
    trueSide(canonical) ?? trueSide(options.map((option) => truthOf(option.text))) ?? 0;
  const trueOption = options[trueIndex];
  const falseOption = options.find((_, index) => index !== trueIndex);
  return { trueIsCorrect: trueOption?.isCorrect ?? true, trueOption, falseOption };
}

const TRUE_WORDS: ReadonlySet<string> = new Set([
  "true",
  "t",
  "đúng",
  "dung",
  "yes",
  "có",
]);
const FALSE_WORDS: ReadonlySet<string> = new Set(["false", "f", "sai", "no", "không"]);

function truthOf(text: string): boolean | null {
  const word = text.normalize("NFC").trim().replace(/\.+$/, "").trim().toLowerCase();
  if (TRUE_WORDS.has(word)) return true;
  if (FALSE_WORDS.has(word)) return false;
  return null;
}

function trueSide(sides: ReadonlyArray<boolean | null>): number | null {
  const named = sides.indexOf(true);
  if (named >= 0) return named;
  const other = sides.indexOf(false);
  if (other < 0) return null;
  return other === 0 ? 1 : 0;
}

/**
 * trueFalseOptions are a true/false question's two options in their stored,
 * canonical form: "True" first, "False" second, with `trueIsCorrect` saying
 * which one is ticked. Given the options they replace, each keeps the id of
 * the option it stands for, so the server matches it to its stored row, and
 * one that had content gets its canonical text as plain content.
 */
export function trueFalseOptions(
  trueIsCorrect: boolean,
  from?: Pick<TrueFalseAnswer, "trueOption" | "falseOption">,
): Option[] {
  return [
    canonicalOption("True", trueIsCorrect, from?.trueOption),
    canonicalOption("False", !trueIsCorrect, from?.falseOption),
  ];
}

function canonicalOption(
  text: string,
  isCorrect: boolean,
  source: Option | undefined,
): Option {
  return {
    id: source?.id ?? null,
    text,
    isCorrect,
    ...(source?.content == null ? {} : { content: plainOptionContent(text) }),
  };
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
    if (value.type !== "true_false") return trueFalseOptions(true);
    const answer = readTrueFalse(value.options);
    return trueFalseOptions(answer.trueIsCorrect, answer);
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
