import type { Answer, StudentQuestion } from "./api";

/**
 * chooseOption is the answer after a student picks `optionId`, by tap or by
 * key. A multiple choice toggles the option. "Choose N" (`selectCount`) also
 * toggles, and the (N+1)th pick replaces the oldest: `optionIds` keeps the
 * order of the picks, so the oldest is still known after a reload. Every other
 * choice type holds one option.
 */
export function chooseOption(
  question: Pick<StudentQuestion, "type" | "selectCount">,
  current: Answer | undefined,
  optionId: string,
): Answer {
  if (question.type !== "multiple_choice")
    return { type: "choice", optionIds: [optionId] };
  const picks = current?.type === "choice" ? current.optionIds : [];
  if (picks.includes(optionId))
    return { type: "choice", optionIds: picks.filter((id) => id !== optionId) };
  const limit = question.selectCount;
  const kept =
    limit == null ? picks : picks.slice(Math.max(0, picks.length - limit + 1));
  return { type: "choice", optionIds: [...kept, optionId] };
}
