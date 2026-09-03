import type { Answer } from "./api";

/**
 * Whether an answer says anything. S-06's dots and the review's counts both
 * read this, so "đã trả lời" means one thing everywhere: a choice with a
 * selection, a blank with any value, text with anything but whitespace, and
 * a true/false that was actually picked.
 */
export function answered(answer: Answer | undefined): boolean {
  if (answer === undefined) return false;
  switch (answer.type) {
    case "choice":
      return answer.optionIds.length > 0;
    case "true_false":
      return typeof answer.value === "boolean";
    case "fill_blank":
      return Object.values(answer.values).some((v) => v.trim() !== "");
    case "text":
      return answer.value.trim() !== "";
  }
}
