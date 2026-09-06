import { useTranslation } from "react-i18next";
import { Info } from "lucide-react";
import { Note } from "./Note";
import { QuestionAudio } from "./QuestionAudio";
import { QuestionBody } from "./QuestionBody";
import { useTakeTestStore } from "../store";
import { worth } from "../worth";
import type { StudentQuestion } from "../api";

/**
 * The store-connected renderer: the one place a question is joined to the
 * answer being written into it. Under the answer, what it is worth (S-05) --
 * except for a short answer, whose body says so beside the word count, and
 * except from 1024px, where the meta line above the stem says it (S-08). A
 * fill-blank closes with the rule it is matched by.
 */
export function QuestionCard({
  question,
  onAudioExpired,
}: Readonly<{
  question: StudentQuestion;
  /** Refetches the attempt when a signed URL has expired (§11.2). */
  onAudioExpired: () => void;
}>) {
  const { t } = useTranslation();
  const answer = useTakeTestStore((s) => s.answers[question.id]);
  const setAnswer = useTakeTestStore((s) => s.setAnswer);
  const locked = useTakeTestStore((s) => s.lock !== null);

  return (
    <div className="space-y-4">
      <QuestionAudio question={question} onExpired={onAudioExpired} />
      <QuestionBody
        question={question}
        answer={answer}
        onAnswer={(next) => setAnswer(question.id, next)}
        disabled={locked}
      />
      {question.type !== "short_answer" && (
        <p className="text-muted-foreground text-xs lg:hidden">{worth(question, t)}</p>
      )}
      {question.type === "fill_blank" && (
        <Note icon={Info}>
          {t(
            (question.blanks ?? []).some((b) => b.caseSensitive)
              ? "takeTest.blankRuleSensitive"
              : "takeTest.blankRuleInsensitive",
          )}
        </Note>
      )}
    </div>
  );
}
