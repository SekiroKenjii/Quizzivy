import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Flag } from "lucide-react";
import { cn } from "@/lib/utils";
import { QuestionAudio } from "./QuestionAudio";
import { QuestionSheet } from "./QuestionSheet";
import { useTakeTestStore } from "../store";
import type { StudentQuestion } from "../api";

/**
 * QuestionCard is the store-connected renderer: the one place a question is
 * joined to the answer being written into it and to its flag. A locked paper
 * keeps the answers legible and refuses edits.
 */
export function QuestionCard({
  question,
  number,
  total,
  lead,
  onAudioExpired,
}: Readonly<{
  question: StudentQuestion;
  number: number;
  total: number;
  lead?: ReactNode;
  onAudioExpired: () => void;
}>) {
  const answer = useTakeTestStore((s) => s.answers[question.id]);
  const setAnswer = useTakeTestStore((s) => s.setAnswer);
  const locked = useTakeTestStore((s) => s.lock !== null);

  return (
    <QuestionSheet
      question={question}
      number={number}
      total={total}
      action={<FlagToggle questionId={question.id} disabled={locked} />}
      lead={lead}
      audio={<QuestionAudio question={question} onExpired={onAudioExpired} />}
      answer={answer}
      onAnswer={(next) => setAnswer(question.id, next)}
      disabled={locked}
      onRetryMedia={onAudioExpired}
    />
  );
}

function FlagToggle({
  questionId,
  disabled,
}: Readonly<{ questionId: string; disabled: boolean }>) {
  const { t } = useTranslation();
  const flagged = useTakeTestStore((s) => s.flags.has(questionId));
  const toggleFlag = useTakeTestStore((s) => s.toggleFlag);
  return (
    <button
      type="button"
      aria-pressed={flagged}
      disabled={disabled}
      className={cn(
        "text-meta inline-flex h-8 min-h-0 min-w-0 flex-none items-center gap-1.5 rounded-md border px-2.5 leading-none font-medium whitespace-nowrap disabled:opacity-60",
        flagged
          ? "border-warning bg-warning-soft text-warning-ink"
          : "border-border bg-card text-fg",
      )}
      onClick={() => toggleFlag(questionId)}
    >
      <Flag className="size-3.5" aria-hidden="true" />
      {t(flagged ? "takeTest.flagOn" : "takeTest.flagOff")}
    </button>
  );
}
