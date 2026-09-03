import type { Answer, StudentQuestion } from "./api";

/**
 * Whether an answer says anything. S-06's dots and the review's counts both
 * read this, so "đã trả lời" means one thing everywhere.
 *
 * A fill_blank needs EVERY blank filled, because grading is per blank (O-17).
 * It takes the question because `values` carries only the blanks typed into.
 */
export function answered(
  question: StudentQuestion,
  answer: Answer | undefined,
): boolean {
  if (answer === undefined) return false;
  switch (answer.type) {
    case "choice":
      return answer.optionIds.length > 0;
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
  }
}
