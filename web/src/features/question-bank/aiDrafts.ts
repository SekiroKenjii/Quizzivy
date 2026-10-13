import type { AiFinding } from "@/features/ai/api";
import type { GeneratedDraft } from "@/features/ai/kinds/generate";
import type { QuestionValues } from "./questionSchema";
import { trueFalseOptions } from "./questionType";

const GAP = /_{3,}/;

function blankPrompt(prompt: string): string {
  return GAP.test(prompt) ? prompt.replace(GAP, "{{1}}") : `${prompt} {{1}}`;
}

function isTrue(text: string | undefined): boolean {
  return text?.trim().toLowerCase() !== "false";
}

/**
 * draftQuestion turns a question the assistant drafted into the values a bank
 * write takes: a plain prompt, one point, the level it was written for, and
 * `tag` when it is not null. A choice question's options are ticked where the
 * key names them; a true/false question takes the two canonical options; a
 * blank's first `___` becomes its one gap, and a short answer's key its
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
 * accepted answer for a blank, or a new plain explanation. A fix that names
 * nothing the question holds, or is advice only, leaves the values as they
 * are.
 */
export function withFix(values: QuestionValues, finding: AiFinding): QuestionValues {
  const { kind, payload } = finding.fix;
  if (payload === undefined) return values;
  const { optionId, blankId, text } = payload;
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
    case "none":
      return values;
  }
}
