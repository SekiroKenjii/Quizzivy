import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { QuestionBody } from "./QuestionBody";
import { useTakeTestStore } from "../store";
import type { StudentQuestion } from "../api";

/**
 * The store-connected renderer: the one place a question is joined to the
 * answer being written into it.
 *
 * QuestionBody itself takes props, so it can be tested one type at a time and
 * reused by the teacher's review screen in Phase 4, which renders the same
 * question against an answer nobody is editing.
 */
export function QuestionCard({ question }: { question: StudentQuestion }) {
  const { t } = useTranslation();
  const answer = useTakeTestStore((s) => s.answers[question.id]);
  const setAnswer = useTakeTestStore((s) => s.setAnswer);
  const locked = useTakeTestStore((s) => s.lock !== null);

  return (
    <div className="space-y-4">
      <QuestionBody
        question={question}
        answer={answer}
        onAnswer={(next) => setAnswer(question.id, next)}
        disabled={locked}
      />
      <p className="text-muted-foreground text-xs">{worth(question, t)}</p>
    </div>
  );
}

/**
 * What the question is worth, and how it is divided (S-05).
 *
 * fill_blank names the per-blank share because that is how it is graded
 * (O-17) -- the line is a promise about scoring, so it has to match
 * grading.gradeFillBlank rather than merely sit near it.
 */
function worth(question: StudentQuestion, t: TFunction): string {
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
function decimal(value: number): string {
  return new Intl.NumberFormat("vi-VN", { maximumFractionDigits: 2 }).format(value);
}
