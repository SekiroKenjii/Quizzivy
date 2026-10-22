import { contentPlainText } from "@/components/shared/content/plainText";
import {
  isQuestionContent,
  type QuestionContent,
} from "@/components/shared/content/questionContent";
import type { AiFinding, AiVariantChange } from "@/features/ai/api";
import type { GeneratedDraft } from "@/features/ai/kinds/generate";
import { MAX_OPTIONS, type QuestionValues } from "./questionSchema";
import { tfngOptions, trueFalseOptions, type TfngAnswer } from "./questionType";

const GAP = /_{3,}/;

function blankPrompt(prompt: string): string {
  return GAP.test(prompt) ? prompt.replace(GAP, "{{1}}") : `${prompt} {{1}}`;
}

function isTrue(text: string | undefined): boolean {
  return text?.trim().toLowerCase() !== "false";
}

function tfngAnswer(text: string | undefined): TfngAnswer {
  const key = text?.trim().toLowerCase();
  if (key === "false") return "false";
  return key === "not given" ? "not_given" : "true";
}

/**
 * draftQuestion turns a question the assistant drafted into the values a bank
 * write takes: a plain prompt, one point, the level it was written for, and
 * `tag` when it is not null. A choice question's options are ticked where the
 * key names them; a true/false question takes the two canonical options, and
 * a tfng question its three in their fixed order; a blank's first `___` becomes its one gap, and a short answer's key its
 * sample answer.
 */
export function draftQuestion(
  draft: GeneratedDraft,
  tag: string | null,
): QuestionValues {
  const { question } = draft;
  const keys = new Set(question.answerKey);
  const base: QuestionValues = {
    type: question.type,
    level: draft.level,
    skill: null,
    prompt: question.prompt,
    promptContent: null,
    mediaAssetId: null,
    mediaAlt: null,
    audio: null,
    transcript: null,
    options: [],
    blanks: [],
    points: 1,
    explanation: question.explanation.trim() === "" ? null : question.explanation,
    sampleAnswer: null,
    tags: tag === null ? [] : [tag],
  };
  switch (question.type) {
    case "single_choice":
    case "multiple_choice":
      return {
        ...base,
        options: (question.options ?? []).map((text) => ({
          id: null,
          text,
          isCorrect: keys.has(text),
        })),
      };
    case "true_false":
      return { ...base, options: trueFalseOptions(isTrue(question.answerKey[0])) };
    case "tfng":
      return { ...base, options: tfngOptions(tfngAnswer(question.answerKey[0])) };
    case "fill_blank":
      return {
        ...base,
        prompt: blankPrompt(question.prompt),
        blanks: [
          {
            id: null,
            ordinal: 1,
            acceptedAnswers: [...question.answerKey],
            caseSensitive: false,
          },
        ],
      };
    case "short_answer":
      return { ...base, sampleAnswer: question.answerKey[0] ?? null };
  }
}

/**
 * withFix applies a check's one-click fix to a question's values: an option's
 * new text (dropping rich content, which the text must match), an extra
 * accepted answer for a blank, a new plain explanation, a tfng question's new
 * key (the option it names becomes the one correct option) or a short
 * answer's word limit. A fix that names nothing the question holds, or is
 * advice only, leaves the values as they are.
 */
export function withFix(values: QuestionValues, finding: AiFinding): QuestionValues {
  const { kind, payload } = finding.fix;
  if (payload === undefined) return values;
  const { optionId, blankId, maxWords, text } = payload;
  switch (kind) {
    case "option_text":
      return {
        ...values,
        options: values.options.map((option) =>
          optionId !== undefined && option.id === optionId
            ? { id: option.id, text, isCorrect: option.isCorrect }
            : option,
        ),
      };
    case "accepted_answer":
      return {
        ...values,
        blanks: values.blanks.map((blank) =>
          blankId !== undefined &&
          blank.id === blankId &&
          !blank.acceptedAnswers.includes(text)
            ? { ...blank, acceptedAnswers: [...blank.acceptedAnswers, text] }
            : blank,
        ),
      };
    case "explanation":
      return { ...values, explanation: text, explanationContent: null };
    case "tfng_answer": {
      const named = (id: string | null) => optionId !== undefined && id === optionId;
      return values.type !== "tfng" ||
        !values.options.some((option) => named(option.id))
        ? values
        : {
            ...values,
            options: values.options.map((option) => ({
              ...option,
              isCorrect: named(option.id),
            })),
          };
    }
    case "word_limit":
      return values.type !== "short_answer" || maxWords === undefined
        ? values
        : { ...values, maxWords };
    case "none":
      return values;
  }
}

/**
 * fixFits reports whether a check's fix still names something the question
 * holds. A fix names an option or a blank by id, and the server gives a
 * question's options new ids each time it is saved, so a finding kept across
 * a save no longer fits; a new tfng key fits while the question is still a
 * tfng holding that option, and a word limit while it is still a short
 * answer with no limit, so it never replaces one the teacher set; the explanation fix names nothing and always does.
 */
export function fixFits(values: QuestionValues, finding: AiFinding): boolean {
  const { kind, payload } = finding.fix;
  const optionId = payload?.optionId;
  const blankId = payload?.blankId;
  switch (kind) {
    case "option_text":
      return optionId !== undefined && values.options.some((o) => o.id === optionId);
    case "tfng_answer":
      return (
        values.type === "tfng" &&
        optionId !== undefined &&
        values.options.some((o) => o.id === optionId)
      );
    case "word_limit":
      return (
        values.type === "short_answer" &&
        values.maxWords == null &&
        payload?.maxWords !== undefined
      );
    case "accepted_answer":
      return blankId !== undefined && values.blanks.some((b) => b.id === blankId);
    case "explanation":
    case "none":
      return true;
  }
}

function reordered(
  options: QuestionValues["options"],
  order: readonly string[],
): QuestionValues["options"] {
  const rank = new Map(order.map((id, index) => [id, index]));
  const last = order.length;
  return [...options].sort(
    (a, b) => (rank.get(a.id ?? "") ?? last) - (rank.get(b.id ?? "") ?? last),
  );
}

function withChange(
  values: QuestionValues,
  change: AiVariantChange,
  id: (optionId: string) => string,
): QuestionValues {
  const { prompt, optionOrder, optionId, text } = change.payload;
  switch (change.kind) {
    case "reword":
      return prompt === undefined ? values : { ...values, prompt, promptContent: null };
    case "reorder":
      return optionOrder === undefined
        ? values
        : { ...values, options: reordered(values.options, optionOrder.map(id)) };
    case "distract":
      if (optionId === undefined || text === undefined) return values;
      return {
        ...values,
        options: values.options.map((option) =>
          option.id === id(optionId) && !option.isCorrect
            ? { id: option.id, text, isCorrect: false }
            : option,
        ),
      };
  }
}

/**
 * withVariant applies a variant's kept changes to a question's values: a
 * reworded prompt as plain text, the options in their new order, or a wrong
 * option's new text; a correct option is never rewritten. The changes name
 * the options of the question they were proposed for, and `ids` maps each of
 * those ids to the id of the matching option in `values`, for a copy whose
 * options have ids of their own.
 */
export function withVariant(
  values: QuestionValues,
  changes: readonly AiVariantChange[],
  ids: ReadonlyMap<string, string> = new Map(),
): QuestionValues {
  const id = (optionId: string) => ids.get(optionId) ?? optionId;
  return changes.reduce((next, change) => withChange(next, change, id), values);
}

function paragraphs(text: string): QuestionContent {
  return {
    format: "semantic_v1",
    blocks: text
      .trim()
      .split(/\n\s*\n/u)
      .map((paragraph) => ({
        type: "paragraph" as const,
        content: paragraph
          .split("\n")
          .flatMap((line, index) => [
            ...(index > 0 ? [{ type: "break" as const }] : []),
            ...(line === "" ? [] : [{ type: "text" as const, text: line, marks: [] }]),
          ]),
      })),
  };
}

/**
 * withExplanation puts an explanation the assistant wrote in place of the
 * question's own, in the form the field holds: rich content while the field is
 * rich (it has content, or no text), a plain string while it is Markdown.
 */
export function withExplanation(values: QuestionValues, text: string): QuestionValues {
  const rich =
    values.explanationContent != null || (values.explanation ?? "").trim() === "";
  const document = rich ? paragraphs(text) : null;
  if (document === null || !isQuestionContent(document))
    return { ...values, explanation: text, explanationContent: null };
  return {
    ...values,
    explanation: contentPlainText(document),
    explanationContent: document,
  };
}

/**
 * withDistractor adds a suggested wrong option to a choice question's options:
 * into the first empty option not marked correct, or as a new last option
 * while there are fewer than MAX_OPTIONS. It is null when every option is
 * written and there is no room for another.
 */
export function withDistractor(
  options: QuestionValues["options"],
  text: string,
): QuestionValues["options"] | null {
  const empty = options.findIndex(
    (option) => !option.isCorrect && option.text.trim() === "",
  );
  if (empty >= 0)
    return options.map((option, index) =>
      index === empty ? { id: option.id, text, isCorrect: false } : option,
    );
  if (options.length >= MAX_OPTIONS) return null;
  return [...options, { id: null, text, isCorrect: false }];
}
