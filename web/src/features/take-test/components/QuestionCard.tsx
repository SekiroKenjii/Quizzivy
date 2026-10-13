import { useTranslation } from "react-i18next";
import { Flag } from "lucide-react";
import { cn } from "@/lib/utils";
import { QuestionAudio } from "./QuestionAudio";
import { QuestionSheet } from "./QuestionSheet";
import { usePaper } from "../paper";
import type { StudentQuestion } from "../api";

/**
 * QuestionCard is the paper-connected renderer: the one place a question is
 * joined to the answer being written into it and to its flag. A locked paper
 * keeps the answers legible and refuses edits.
 */
export function QuestionCard({
  question,
  number,
  total,
  onAudioExpired,
}: Readonly<{
  question: StudentQuestion;
  number: number;
  total: number;
  onAudioExpired: () => void;
}>) {
  const { answers, setAnswer, lock } = usePaper();
  const locked = lock !== null;

  return (
    <QuestionSheet
      question={question}
      number={number}
      total={total}
      action={<FlagToggle questionId={question.id} disabled={locked} />}
      audio={<QuestionAudio question={question} onExpired={onAudioExpired} />}
      answer={answers[question.id]}
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
  const { flags, toggleFlag } = usePaper();
  const flagged = flags.has(questionId);
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
