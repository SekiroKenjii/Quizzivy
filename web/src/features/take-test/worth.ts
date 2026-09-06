import type { TFunction } from "i18next";
import type { StudentQuestion } from "./api";

/**
 * What the question is worth, and how it is divided (S-05).
 *
 * fill_blank names the per-blank share because that is how it is graded
 * (O-17) -- the line is a promise about scoring, so it has to match
 * grading.gradeFillBlank rather than merely sit near it.
 */
export function worth(question: StudentQuestion, t: TFunction): string {
  const parts = [t("takeTest.points", { points: decimal(question.points) })];

  const blanks = question.blanks?.length ?? 0;
  if (question.type === "fill_blank" && blanks > 0) {
    parts.push(t("takeTest.perBlank", { points: decimal(question.points / blanks) }));
  }
  if (question.type === "short_answer") {
    parts.push(t("takeTest.manualGrading"));
  }
  return parts.join(" · ");
}

/** Two decimals at most, and none when the number is whole: "1", not "1.00". */
export function decimal(value: number): string {
  return new Intl.NumberFormat("vi-VN", { maximumFractionDigits: 2 }).format(value);
}
