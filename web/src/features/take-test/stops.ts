import type { TFunction } from "i18next";
import { tfngLabelKey, trueFalseLabelKey } from "@/components/shared/content/trueFalse";
import { nfc } from "@/lib/nfc";
import { answered } from "./answered";
import { isPlainParagraph } from "./sentenceGap";
import type { Answer, StudentGroup, StudentQuestion } from "./api";

type Question = Pick<StudentQuestion, "id" | "type" | "prompt" | "promptContent"> &
  Readonly<{ media?: StudentQuestion["media"] }>;
type Group = Pick<StudentGroup, "questionIds" | "stimuli">;

/**
 * Cloze is one material's gaps that single-choice members of its group
 * answer: the positions of those members on the paper, ascending. Its size
 * is the cloze's number of gaps.
 */
export interface Cloze {
  positions: readonly number[];
}

/**
 * Stop is one screen of the engine: the positions of the questions it shows,
 * ascending, and the cloze they belong to, if any. A run is a stop of two or
 * more gaps of one cloze; every other stop holds one question.
 */
export interface Stop {
  positions: readonly number[];
  cloze: Cloze | null;
}

/**
 * paperStops divides a paper into stops, returning the stop of every
 * position. A run is two or more consecutive single-choice members of one
 * group, each bound by a question gap to a gap of the same material, none of
 * them with an image or audio of its own. A member with media ends the run
 * and is a stop of its own, as is every other question.
 */
export function paperStops(
  questions: readonly Question[],
  groups: readonly Group[],
): Stop[] {
  const clozes = clozesByPosition(questions, groups);
  const stops: Stop[] = [];
  let at = 0;
  while (at < questions.length) {
    const cloze = clozes.get(at) ?? null;
    let end = at;
    if (cloze !== null && !hasMedia(questions[at])) {
      while (
        end + 1 < questions.length &&
        clozes.get(end + 1) === cloze &&
        !hasMedia(questions[end + 1])
      )
        end++;
    }
    const stop: Stop = {
      positions: Array.from({ length: end - at + 1 }, (_, i) => at + i),
      cloze,
    };
    for (let p = at; p <= end; p++) stops[p] = stop;
    at = end + 1;
  }
  return stops;
}

/**
 * clozesByPosition maps the position of every single-choice member bound to
 * a gap to the cloze of that gap's material.
 */
function clozesByPosition(
  questions: readonly Question[],
  groups: readonly Group[],
): Map<number, Cloze> {
  const position = new Map(questions.map((question, index) => [question.id, index]));
  const clozes = new Map<number, Cloze>();
  for (const group of groups) {
    const members = new Set(group.questionIds);
    for (const material of group.stimuli) {
      const positions = material.gaps
        .flatMap((gap) => {
          if (gap.kind !== "question" || !members.has(gap.questionId)) return [];
          const at = position.get(gap.questionId);
          return at !== undefined && questions[at]?.type === "single_choice"
            ? [at]
            : [];
        })
        .sort((a, b) => a - b);
      if (positions.length === 0) continue;
      const cloze: Cloze = { positions };
      for (const at of positions) if (!clozes.has(at)) clozes.set(at, cloze);
    }
  }
  return clozes;
}

function hasMedia(question: Question | undefined): boolean {
  return question?.media != null;
}

/** isRun reports whether a stop shows two or more gaps of a cloze on one screen. */
export function isRun(stop: Stop | undefined): stop is Stop & { cloze: Cloze } {
  return stop !== undefined && stop.cloze !== null && stop.positions.length > 1;
}

/** stepBack is the first position of the stop before the one at `index`, or null on the first. */
export function stepBack(stops: readonly Stop[], index: number): number | null {
  const first = stops[index]?.positions[0] ?? index;
  if (first <= 0) return null;
  return stops[first - 1]?.positions[0] ?? first - 1;
}

/** stepOn is the first position of the stop after the one at `index`, or null on the last. */
export function stepOn(stops: readonly Stop[], index: number): number | null {
  const last = stops[index]?.positions.at(-1) ?? index;
  return last + 1 < stops.length ? last + 1 : null;
}

/**
 * nextEmptyGap is the position a run opens once the gap at `open` is
 * answered: the next gap with no answer, going round from `open`, or null
 * when every other gap is answered.
 */
export function nextEmptyGap(
  stop: Stop,
  open: number,
  questions: readonly StudentQuestion[],
  answers: Readonly<Record<string, Answer>>,
): number | null {
  const { positions } = stop;
  const from = positions.indexOf(open);
  for (let step = 1; step < positions.length; step++) {
    const at = positions[(from + step) % positions.length]!;
    const question = questions[at];
    if (question !== undefined && !answered(question, answers[question.id])) return at;
  }
  return null;
}

/**
 * sharedPrompt is the one prompt a run prints above its hint: the prompt of
 * its first member when every member has the same plain prompt (no rich
 * content, or rich content that is one paragraph of unmarked text projecting
 * to the prompt), and null otherwise.
 */
export function sharedPrompt<Q extends Question>(
  stop: Stop,
  questions: readonly Q[],
): Q | null {
  const members = stop.positions.flatMap((at) => {
    const question = questions[at];
    return question === undefined ? [] : [question];
  });
  const first = members[0];
  if (first === undefined) return null;
  const text = nfc(first.prompt).trim();
  const plain = members.every(
    (member) =>
      (member.promptContent == null ||
        isPlainParagraph(member.promptContent, member.prompt)) &&
      nfc(member.prompt).trim() === text,
  );
  return plain ? first : null;
}

/**
 * chosenWord is the plain text of what a choice answer chose, as a gap shows
 * it: the options' texts, joined, with a canonical true/false option named
 * as the engine names it; null when nothing is chosen.
 */
export function chosenWord(
  question: Pick<StudentQuestion, "type" | "options">,
  answer: Answer | undefined,
  t: TFunction,
): string | null {
  if (answer?.type !== "choice" || answer.optionIds.length === 0) return null;
  const chosen = new Set(answer.optionIds);
  const words = (question.options ?? []).flatMap((option, index) => {
    if (!chosen.has(option.id)) return [];
    const canonical =
      question.type === "tfng"
        ? tfngLabelKey(index)
        : trueFalseLabelKey(question.type, option.text, option.content);
    return [canonical === null ? nfc(option.text) : t(canonical)];
  });
  return words.length === 0 ? null : words.join(", ");
}

/**
 * openAfterChoice is the position to open once `answer` is chosen for the
 * question at `open`: on a run, the next gap with no answer, going round;
 * null when the stop is not a run or every other gap is answered.
 */
export function openAfterChoice(
  stops: readonly Stop[],
  open: number,
  questions: readonly StudentQuestion[],
  answers: Readonly<Record<string, Answer>>,
  answer: Answer,
): number | null {
  const stop = stops[open];
  const question = questions[open];
  if (!isRun(stop) || question === undefined) return null;
  return nextEmptyGap(stop, open, questions, { ...answers, [question.id]: answer });
}
