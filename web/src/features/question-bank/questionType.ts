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

/** editorType is the entry of QUESTION_TYPES a question of `type` shows as. */
export function editorType(type: QuestionType): QuestionType {
  return type === "tfng" ? "true_false" : type;
}

/** ESSAY_POINTS is an essay's default points: a band of 3 to 9 is then its score. */
export const ESSAY_POINTS = 9;

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
  const word = trimSpacesAndDots(text.normalize("NFC")).toLowerCase();
  if (TRUE_WORDS.has(word)) return true;
  if (FALSE_WORDS.has(word)) return false;
  return null;
}

function trimSpacesAndDots(text: string): string {
  let start = 0;
  let end = text.length;
  while (start < end && (text[start] === " " || text[start] === ".")) start++;
  while (end > start && (text[end - 1] === " " || text[end - 1] === ".")) end--;
  return text.slice(start, end);
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

/** TfngAnswer is which of a tfng question's three fixed options is correct. */
export type TfngAnswer = "true" | "false" | "not_given";

const TFNG_ORDER: readonly TfngAnswer[] = ["true", "false", "not_given"];

/** TFNG_TEXTS are a tfng question's option texts in their fixed order, as the server writes them. */
export const TFNG_TEXTS: Readonly<Record<TfngAnswer, string>> = {
  true: "True",
  false: "False",
  not_given: "Not given",
};

/**
 * readTfng reads a tfng question's answer by position: the server stores its
 * three options in fixed order (True, False, Not given). With none ticked
 * the answer is True, as a new true/false question's is.
 */
export function readTfng(options: readonly Option[]): TfngAnswer {
  const index = options.findIndex((option) => option.isCorrect);
  return TFNG_ORDER[index] ?? "true";
}

/**
 * tfngOptions are a tfng question's three options in their fixed order with
 * `answer` ticked, each keeping the id of the option it replaces at its
 * position, so the server matches it to its stored row.
 */
export function tfngOptions(
  answer: TfngAnswer,
  from: readonly Option[] = [],
): Option[] {
  return TFNG_ORDER.map((key, index) =>
    canonicalOption(TFNG_TEXTS[key], key === answer, from[index]),
  );
}

/**
 * withNotGiven turns a true/false question into a tfng one, or back, keeping
 * its answer and the ids of its True and False options. Turning "Not given"
 * off while it is the answer makes the answer False, as the deck draws it.
 */
export function withNotGiven(value: QuestionValues, on: boolean): QuestionValues {
  if (on === (value.type === "tfng")) return value;
  if (on) {
    const answer = readTrueFalse(value.options);
    return {
      ...value,
      type: "tfng",
      options: tfngOptions(answer.trueIsCorrect ? "true" : "false", [
        ...(answer.trueOption ? [answer.trueOption] : []),
        ...(answer.falseOption ? [answer.falseOption] : []),
      ]),
    };
  }
  const answer = readTfng(value.options);
  return {
    ...value,
    type: "true_false",
    options: trueFalseOptions(answer === "true", {
      trueOption: value.options[0],
      falseOption: value.options[1],
    }),
  };
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
 * the explanation, media, audio, points and tags always; the options among
 * single choice, multiple choice and matching, where single keeps only the
 * first tick and matching none; true / false rebuilt as its two canonical
 * options (a tfng question keeps its answer unless it was "Not given"); the
 * blanks only for fill in the blank; the sample answer between short answer
 * and essay. Each R6 setting stays only on its own type. Points the teacher
 * left at the old type's default take the new type's (nine for an essay, one
 * otherwise); points they set are kept. A value equal to the default cannot
 * be told from it, so nine points set on a choice question come back as one
 * after a trip through essay (QC-R6A-4, accepted: the field shows the change).
 */
export function retype(value: QuestionValues, type: QuestionType): QuestionValues {
  const options = optionsFor(value, type);
  return withTypeFields(
    {
      ...value,
      type,
      options,
      blanks: type === "fill_blank" ? value.blanks : [],
      sampleAnswer:
        WRITTEN_TYPES.has(type) && WRITTEN_TYPES.has(value.type)
          ? value.sampleAnswer
          : null,
      points:
        value.points === defaultPoints(value.type) ? defaultPoints(type) : value.points,
    },
    value,
  );
}

/** defaultPoints is what a new question of `type` is worth: nine for an essay, else one. */
export function defaultPoints(type: QuestionType): number {
  return type === "essay" ? ESSAY_POINTS : 1;
}

const WRITTEN_TYPES: ReadonlySet<QuestionType> = new Set(["short_answer", "essay"]);

function withTypeFields(next: QuestionValues, from: QuestionValues): QuestionValues {
  const rest = { ...next };
  delete rest.selectCount;
  delete rest.maxWords;
  delete rest.matchAllowReuse;
  delete rest.matchItems;
  delete rest.rubric;
  const same = next.type === from.type;
  switch (next.type) {
    case "multiple_choice":
      return same && from.selectCount != null
        ? { ...rest, selectCount: from.selectCount }
        : rest;
    case "short_answer":
      return same && from.maxWords != null
        ? { ...rest, maxWords: from.maxWords }
        : rest;
    case "essay":
      return { ...rest, rubric: same ? (from.rubric ?? []) : [] };
    case "matching":
      return {
        ...rest,
        matchAllowReuse: same ? (from.matchAllowReuse ?? false) : false,
        matchItems: same ? (from.matchItems ?? []) : blankItems(),
      };
    default:
      return rest;
  }
}

const LISTED_TYPES: ReadonlySet<QuestionType> = new Set([
  "single_choice",
  "multiple_choice",
  "matching",
]);

function trueFalseFor(value: QuestionValues): Option[] {
  if (value.type === "tfng") return withNotGiven(value, false).options;
  if (value.type !== "true_false") return trueFalseOptions(true);
  const answer = readTrueFalse(value.options);
  return trueFalseOptions(answer.trueIsCorrect, answer);
}

function optionsFor(value: QuestionValues, type: QuestionType): Option[] {
  const carried = LISTED_TYPES.has(value.type) ? value.options : [];
  if (type === "true_false") return trueFalseFor(value);
  if (type === "tfng") {
    if (value.type === "tfng")
      return tfngOptions(readTfng(value.options), value.options);
    return withNotGiven({ ...value, options: trueFalseFor(value) }, true).options;
  }
  if (type === "matching") {
    if (carried.length === 0) return blankOptions(false);
    return carried.map((option) => ({ ...option, isCorrect: false }));
  }
  if (type === "multiple_choice") return carried.length > 0 ? carried : blankOptions();
  if (type === "single_choice") {
    if (carried.length === 0) return blankOptions();
    const first = carried.findIndex((option) => option.isCorrect);
    return carried.map((option, index) => ({ ...option, isCorrect: index === first }));
  }
  return [];
}

function blankOptions(firstCorrect = true): Option[] {
  return [
    { id: null, text: "", isCorrect: firstCorrect },
    { id: null, text: "", isCorrect: false },
  ];
}

function blankItems(): NonNullable<QuestionValues["matchItems"]> {
  return [
    { id: null, text: "", correctOption: -1 },
    { id: null, text: "", correctOption: -1 },
  ];
}
