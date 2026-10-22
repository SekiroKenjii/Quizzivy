import type { TFunction } from "i18next";
import { ApiError } from "@/lib/api/errors";
import "./gradingMessages";

const SHAPE = new Set([
  "GRADE_NEEDS_BAND",
  "GRADE_CRITERIA_INCOMPLETE",
  "GRADE_NEEDS_POINTS",
  "GRADE_BAND_NOT_ALLOWED",
]);

/**
 * gradeFieldMessage is the message a refused grade shows beside the marks of
 * the question it names, or null when the refusal is not about that question's
 * marks: a 422 grade-shape refusal (`details.questionIds`), in the reader's
 * language, or a 400 whose `details` holds a message keyed by the question's
 * id. Every other failure stays the page's.
 */
export function gradeFieldMessage(
  cause: unknown,
  questionId: string,
  t: TFunction,
): string | null {
  if (!(cause instanceof ApiError)) return null;
  if (cause.status === 422 && SHAPE.has(cause.code)) {
    const ids = cause.details?.["questionIds"];
    if (Array.isArray(ids) && ids.length > 0 && !ids.includes(questionId)) return null;
    return t(`grading.gradeError.${cause.code}`);
  }
  const keyed = cause.status === 400 ? cause.details?.[questionId] : undefined;
  return typeof keyed === "string" && keyed.trim() !== "" ? keyed : null;
}
