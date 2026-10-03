import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { ContentImage } from "@/components/shared/content/ContentImage";
import { PAPER_SURFACE } from "@/components/shared/content/paperSurface";
import { cn } from "@/lib/utils";
import { QuestionBody } from "./QuestionBody";
import { UnknownType } from "./UnknownType";
import { questionKind, questionLine } from "../questionType";
import { worth } from "../worth";
import type { Answer, StudentQuestion } from "../api";

/**
 * QuestionSheet is the question pane's column, as the deck draws it: the line
 * "Question 4 of 8 · Choose one" with `action` at its far end, then `lead`,
 * the question's recording and image, the prompt and the answer, and under
 * the answer what the question is worth. A short answer says its worth beside
 * the word count. A type this page has no renderer for gets UnknownType in
 * place of the prompt and the answer, and claims no worth. Images and rich
 * tables inside it keep the paper surface in dark mode. It holds no state:
 * the engine joins it to the store through QuestionCard, and the teacher's
 * preview draws it read-only.
 */
export function QuestionSheet({
  question,
  number,
  total,
  headingId,
  action,
  lead,
  audio,
  answer,
  onAnswer,
  disabled = false,
  onRetryMedia,
}: Readonly<{
  question: StudentQuestion;
  number: number;
  total: number;
  headingId?: string | undefined;
  action?: ReactNode;
  lead?: ReactNode;
  audio?: ReactNode;
  answer: Answer | undefined;
  onAnswer: (answer: Answer) => void;
  disabled?: boolean;
  onRetryMedia?: (() => void) | undefined;
}>) {
  const { t } = useTranslation();
  const kind = questionKind(question);
  return (
    <div className={cn("flex flex-col gap-4.5", PAPER_SURFACE)}>
      <div className="flex items-center justify-between gap-2.5">
        <p
          id={headingId}
          tabIndex={headingId === undefined ? undefined : -1}
          className="text-muted-fg min-w-0 scroll-mt-6 rounded-sm text-sm font-medium"
        >
          {questionLine(question, number, total, t)}
        </p>
        {action}
      </div>
      {lead}
      {audio}
      {question.media?.kind === "image" && question.media.url ? (
        <ContentImage
          src={question.media.url}
          alt={t("takeTest.imageLabel")}
          onRetry={onRetryMedia}
        />
      ) : null}
      {kind === "unknown" ? (
        <UnknownType number={number} />
      ) : (
        <QuestionBody
          question={question}
          answer={answer}
          onAnswer={onAnswer}
          disabled={disabled}
        />
      )}
      {(kind === "choice" || kind === "fill_blank") && (
        <p className="text-muted-fg text-meta -mt-3 leading-normal">
          {worth(question, t)}
        </p>
      )}
    </div>
  );
}
