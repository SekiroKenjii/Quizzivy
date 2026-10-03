import type { TFunction } from "i18next";
import type { StudentQuestion } from "./api";

/**
 * QuestionKind is how the engine answers a question. The three choice types
 * differ only in how many options may be chosen, so they share a kind.
 * `unknown` is a type this build has no renderer for: the paper says so and
 * writes no answer for it.
 */
export type QuestionKind = "choice" | "fill_blank" | "short_answer" | "unknown";

const KIND: Record<StudentQuestion["type"], Exclude<QuestionKind, "unknown">> = {
  single_choice: "choice",
  multiple_choice: "choice",
  true_false: "choice",
  fill_blank: "fill_blank",
  short_answer: "short_answer",
};

const LABEL: Record<StudentQuestion["type"], string> = {
  single_choice: "takeTest.type.single_choice",
  multiple_choice: "takeTest.type.multiple_choice",
  true_false: "takeTest.type.true_false",
  fill_blank: "takeTest.type.fill_blank",
  short_answer: "takeTest.type.short_answer",
};

/**
 * questionKind reads the kind from the question's type. A type the server
 * added after this page was loaded is `unknown`, never a choice.
 */
export function questionKind(question: Pick<StudentQuestion, "type">): QuestionKind {
  return Object.hasOwn(KIND, question.type) ? KIND[question.type] : "unknown";
}

/**
 * questionLine is the line above a question, "Question 4 of 8 · Choose one".
 * An unknown type has no label, so its line stops at the position.
 */
export function questionLine(
  question: Pick<StudentQuestion, "type">,
  number: number,
  total: number,
  t: TFunction,
): string {
  if (!Object.hasOwn(LABEL, question.type)) {
    return t("takeTest.questionOf", { n: number, total });
  }
  return t("takeTest.questionOfType", {
    n: number,
    total,
    type: t(LABEL[question.type]),
  });
}
