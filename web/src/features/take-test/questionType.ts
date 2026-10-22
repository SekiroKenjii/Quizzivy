import type { TFunction } from "i18next";
import type { StudentQuestion } from "./api";

/**
 * QuestionKind is how the engine answers a question. The four choice types
 * differ only in how many options may be chosen and how they are labelled,
 * so they share a kind. `unknown` is a type this build has no renderer for:
 * the paper says so and writes no answer for it.
 */
export type QuestionKind =
  "choice" | "fill_blank" | "short_answer" | "essay" | "matching" | "unknown";

type DrawnType = StudentQuestion["type"];

const KIND: Record<DrawnType, Exclude<QuestionKind, "unknown">> = {
  single_choice: "choice",
  multiple_choice: "choice",
  true_false: "choice",
  tfng: "choice",
  fill_blank: "fill_blank",
  short_answer: "short_answer",
  essay: "essay",
  matching: "matching",
};

const LABEL: Record<DrawnType, string> = {
  single_choice: "takeTest.type.single_choice",
  multiple_choice: "takeTest.type.multiple_choice",
  true_false: "takeTest.type.true_false",
  tfng: "takeTest.type.tfng",
  fill_blank: "takeTest.type.fill_blank",
  short_answer: "takeTest.type.short_answer",
  essay: "takeTest.type.essay",
  matching: "takeTest.type.matching",
};

/**
 * questionKind reads the kind from the question's type. A type the server
 * added after this page was loaded is `unknown`, never a choice.
 */
export function questionKind(question: Pick<StudentQuestion, "type">): QuestionKind {
  return isDrawn(question.type) ? KIND[question.type] : "unknown";
}

function isDrawn(type: string): type is DrawnType {
  return Object.hasOwn(KIND, type);
}

/**
 * questionLine is the line above a question, "Question 4 of 8 · Single
 * choice": the type is named as the builder's type menu names it.
 * An unknown type has no label, so its line stops at the position.
 */
export function questionLine(
  question: Pick<StudentQuestion, "type">,
  number: number,
  total: number,
  t: TFunction,
): string {
  if (!isDrawn(question.type)) {
    return t("takeTest.questionOf", { n: number, total });
  }
  return t("takeTest.questionOfType", {
    n: number,
    total,
    type: t(LABEL[question.type]),
  });
}

/**
 * clozeLine is the line above a stop that holds gaps of a cloze: "Questions
 * 23–27 of 34 · Cloze · 5 gaps" for the whole cloze, "… · 3 of 5 gaps" for
 * part of it, and "Question 26 of 34 · Cloze · 1 of 5 gaps" for a gap shown
 * on its own. `positions` are the stop's, `size` the cloze's number of gaps.
 */
export function clozeLine(
  positions: readonly number[],
  size: number,
  total: number,
  t: TFunction,
): string {
  const from = (positions[0] ?? 0) + 1;
  const to = (positions.at(-1) ?? 0) + 1;
  const shown = positions.length;
  return t("takeTest.cloze.line", {
    position:
      from === to
        ? t("takeTest.questionOf", { n: from, total })
        : t("takeTest.cloze.questions", { from, to, total }),
    type: t("takeTest.cloze.type"),
    gaps:
      shown === size
        ? t("takeTest.cloze.gaps", { count: size })
        : t("takeTest.cloze.gapsOf", { shown, count: size }),
  });
}
