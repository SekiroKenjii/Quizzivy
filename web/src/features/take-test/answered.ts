import type { Answer, StudentQuestion } from "./api";
import type { DotState } from "./components/Navigator";

/**
 * Whether an answer says anything. S-06's dots and the review's counts both
 * read this, so "đã trả lời" means one thing everywhere.
 *
 * A fill_blank needs EVERY blank filled, because grading is per blank (O-17),
 * and a matching every item paired. "Choose N" needs exactly N picks (T-R6.6),
 * as the student card's count does on the server. It takes the question
 * because `values` and `pairs` carry only what was typed or paired.
 */
export function answered(
  question: StudentQuestion,
  answer: Answer | undefined,
): boolean {
  if (answer === undefined) return false;
  switch (answer.type) {
    case "choice":
      return question.type === "multiple_choice" && question.selectCount != null
        ? answer.optionIds.length === question.selectCount
        : answer.optionIds.length > 0;
    case "true_false":
      return typeof answer.value === "boolean";
    case "fill_blank": {
      const blanks = question.blanks ?? [];
      return (
        blanks.length > 0 &&
        blanks.every((blank) => (answer.values[blank.id] ?? "").trim() !== "")
      );
    }
    case "text":
      return answer.value.trim() !== "";
    case "matching": {
      const items = question.matchItems ?? [];
      return (
        items.length > 0 &&
        items.every((item) => typeof answer.pairs[item.id] === "string")
      );
    }
  }
}

/** Progress is how far a question is answered: not at all, partly, or fully. */
export type Progress = "none" | "partly" | "full";

/**
 * progress says how far an answer goes, for the navigator's squares. Full is
 * exactly `answered`. Partly is a fill-in with at least one blank typed and
 * not all of them, a matching with at least one item paired and not all of
 * them, or a "Choose N" with some picks and not exactly N; every other answer
 * is full or none.
 */
export function progress(
  question: StudentQuestion,
  answer: Answer | undefined,
): Progress {
  if (answered(question, answer)) return "full";
  switch (answer?.type) {
    case "fill_blank":
      return (question.blanks ?? []).some(
        (blank) => (answer.values[blank.id] ?? "").trim() !== "",
      )
        ? "partly"
        : "none";
    case "matching":
      return (question.matchItems ?? []).some(
        (item) => typeof answer.pairs[item.id] === "string",
      )
        ? "partly"
        : "none";
    case "choice":
      return question.type === "multiple_choice" &&
        question.selectCount != null &&
        answer.optionIds.length > 0
        ? "partly"
        : "none";
    default:
      return "none";
  }
}

/**
 * dotsOf gives each question of the paper its square's state: answered,
 * partly answered, flagged.
 */
export function dotsOf(
  questions: StudentQuestion[],
  answers: Readonly<Record<string, Answer>>,
  flags: ReadonlySet<string>,
): DotState[] {
  return questions.map((q) => {
    const done = progress(q, answers[q.id]);
    return {
      id: q.id,
      answered: done === "full",
      partly: done === "partly",
      flagged: flags.has(q.id),
    };
  });
}
