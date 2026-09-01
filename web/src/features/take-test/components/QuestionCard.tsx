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
      <p className="text-muted-foreground text-xs">
        {t("builder.points", { points: question.points })}
        {question.type === "short_answer" ? ` · ${t("takeTest.manualGrading")}` : ""}
      </p>
    </div>
  );
}
