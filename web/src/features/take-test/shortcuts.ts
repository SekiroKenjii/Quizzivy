import type { Answer, StudentQuestion } from "./api";
import { questionShowing } from "./panes";
import { questionKind } from "./questionType";
import { openAfterChoice, stepBack, stepOn, type Stop } from "./stops";

const PICK_KEYS = "abcde";

/**
 * PaperKey is what a key pressed over the paper asks for: open the stop at
 * `index` (null at either end), flag the question (null when the key repeats
 * or the question is out of sight), or give the question `answer` and open
 * `open` after it. Each one is the engine's to act on and to prevent the
 * key's default for.
 */
export type PaperKey =
  | { kind: "open"; index: number | null }
  | { kind: "flag"; questionId: string | null }
  | { kind: "answer"; questionId: string; answer: Answer; open: number | null };

/**
 * paperKey reads the engine's keys, the deck's: the arrows move by stop and
 * stop at either end, A to E choose and F flags, on a run of a cloze for the
 * open gap, and a choice opens the run's next gap with no answer. It answers
 * null for a key with a modifier, one composing text, one already handled,
 * one typed into a field, and any other key. Whether a dialog is open or the
 * paper is locked is the caller's to check first.
 */
export function paperKey(
  event: KeyboardEvent,
  paper: Readonly<{
    stops: Stop[];
    index: number;
    question: StudentQuestion;
    questions: StudentQuestion[];
    answers: Readonly<Record<string, Answer>>;
  }>,
): PaperKey | null {
  const { stops, index, question, questions, answers } = paper;
  if (
    event.defaultPrevented ||
    event.isComposing ||
    event.metaKey ||
    event.ctrlKey ||
    event.altKey ||
    typingIn(event.target)
  )
    return null;
  switch (event.key) {
    case "ArrowRight":
      return { kind: "open", index: stepOn(stops, index) };
    case "ArrowLeft":
      return { kind: "open", index: stepBack(stops, index) };
    case "f":
    case "F":
      return {
        kind: "flag",
        questionId: !event.repeat && questionShowing(question.id) ? question.id : null,
      };
  }
  const pick = PICK_KEYS.indexOf(event.key.toLowerCase());
  const option = question.options?.[pick];
  if (
    pick >= 0 &&
    option !== undefined &&
    !event.repeat &&
    questionKind(question) === "choice" &&
    questionShowing(question.id)
  ) {
    const answer = chooseOption(question, answers[question.id], option.id);
    return {
      kind: "answer",
      questionId: question.id,
      answer,
      open: openAfterChoice(stops, index, questions, answers, answer),
    };
  }
  return null;
}

function typingIn(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  return (
    target.isContentEditable ||
    (target instanceof HTMLInputElement &&
      target.type !== "radio" &&
      target.type !== "checkbox") ||
    target.tagName === "TEXTAREA" ||
    target.tagName === "SELECT" ||
    target.closest(
      '[role="slider"], [role="menu"], [role="listbox"], [role="combobox"]',
    ) !== null
  );
}

function chooseOption(
  question: StudentQuestion,
  current: Answer | undefined,
  optionId: string,
): Answer {
  if (question.type === "multiple_choice") {
    const chosen = new Set(current?.type === "choice" ? current.optionIds : []);
    if (!chosen.delete(optionId)) chosen.add(optionId);
    return { type: "choice", optionIds: [...chosen] };
  }
  return { type: "choice", optionIds: [optionId] };
}
