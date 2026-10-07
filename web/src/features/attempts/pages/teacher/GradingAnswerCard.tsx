import { Avatar } from "@/components/ui/avatar";
import { givenName } from "@/features/assignments/studentTime";
import { useState, type MouseEvent } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { AnswerReview } from "../../components/AnswerReview";
import { ReviewGroup } from "@/features/media/components/ReviewGroup";
import type { GradingQueueItem } from "../../api";
import type { GradingDraft } from "./useGradingQueue";
import { scoreOptions } from "./gradingRecovery";

/** GradingAnswerCard preserves the frozen answer and local comment while each chosen score saves immediately. */
export function GradingAnswerCard({
  item,
  draft,
  busy,
  finishReady,
  position,
  total,
  onScore,
  onComment,
  onPrevious,
  onNext,
  onRetryMaterial,
  onReview,
}: Readonly<{
  item: GradingQueueItem;
  draft: GradingDraft | undefined;
  busy: boolean;
  finishReady: boolean;
  position: number;
  total: number;
  onScore: (points: number) => void;
  onComment: (comment: string) => void;
  onPrevious: () => void;
  onNext: () => void;
  onRetryMaterial: () => void;
  onReview: (event: MouseEvent<HTMLAnchorElement>) => void;
}>) {
  const { t } = useTranslation();
  const [number, setNumber] = useState(String(draft?.points ?? item.score ?? ""));
  const points = draft?.points ?? item.score;
  const comment = draft?.comment ?? item.comment ?? "";
  const choices = scoreOptions(item.points);
  const group = item.sharedContext?.groups.find((entry) =>
    entry.questionIds.includes(item.questionId),
  );
  const snippets = item.points === 1 ? "short" : "essay";
  return (
    <section
      className="bg-card flex min-w-0 flex-1 flex-col overflow-hidden rounded-xl border shadow-sm"
      aria-label={t("grading.answerCard")}
    >
      <div className="space-y-4.5 px-5 py-4.5">
        <div className="flex flex-wrap items-center justify-between gap-2.5">
          <div className="flex min-w-0 items-center gap-2.5">
            <Avatar name={item.studentName} className="size-8.5" />
            <div className="min-w-0">
              <p className="text-sm font-semibold">{item.studentName}</p>
              <p className="text-muted-foreground text-[12.5px]">
                {item.assignmentTitle}
              </p>
            </div>
          </div>
          <span className="text-muted-foreground text-[12.5px]">
            <span className="mr-2 rounded-full border px-2 py-0.5">
              {t(`questionEditor.type.${item.type}`, { defaultValue: item.type })}
            </span>
            {t("grading.questionPoints", { n: item.questionNumber, max: item.points })}
          </span>
        </div>
        {group && item.sharedContext && (
          <ReviewGroup
            group={group}
            transcripts={item.sharedContext.transcripts}
            {...(item.sharedContext.audioPlays
              ? { plays: item.sharedContext.audioPlays }
              : {})}
            numbers={new Map([[item.questionId, item.questionNumber]])}
            onRetry={onRetryMaterial}
          />
        )}
        <div
          className={
            item.type === "short_answer"
              ? "[&>div>div:last-child]:bg-muted [&>div>div:last-child]:rounded-[10px] [&>div>div:last-child]:border-0"
              : undefined
          }
        >
          <AnswerReview
            question={item}
            answer={{ answer: item.answer, manualScore: points }}
          />
        </div>
        {item.blanks && item.blanks.length > 0 && (
          <div className="flex flex-wrap gap-1.5">
            <span className="text-muted-foreground text-xs">
              {t("grading.accepted")}
            </span>
            {item.blanks.flatMap((blank) =>
              blank.acceptedAnswers.map((answer) => (
                <span
                  key={`${blank.id}:${answer}`}
                  className="bg-success-soft text-success-ink rounded-md px-2 py-0.5 text-xs"
                >
                  {answer}
                </span>
              )),
            )}
          </div>
        )}
        {item.sampleAnswer && (
          <div className="bg-info-soft text-info-ink rounded-lg p-3">
            <h3 className="text-xs font-medium">{t("review.sampleAnswer")}</h3>
            <p className="mt-1 text-sm whitespace-pre-wrap">{item.sampleAnswer}</p>
          </div>
        )}
        <fieldset disabled={busy} className="space-y-2">
          <legend className="text-muted-foreground mb-2 text-xs font-medium">
            {t("review.points")}
          </legend>
          {choices.length > 0 ? (
            <div className="flex flex-wrap gap-2">
              {choices.map((value, index) => (
                <Button
                  key={value}
                  type="button"
                  variant={points === value ? "default" : "outline"}
                  aria-pressed={points === value}
                  aria-label={t("grading.score", { points: value })}
                  className="h-11 min-w-16 gap-2 rounded-[9px] px-3.5 text-[15px] font-semibold tabular-nums"
                  onClick={() => onScore(value)}
                >
                  {value}
                  <kbd
                    aria-hidden="true"
                    className="rounded border border-current px-1 text-[10.5px] font-medium opacity-60"
                  >
                    {index + 1}
                  </kbd>
                </Button>
              ))}
            </div>
          ) : (
            <label className="block space-y-2 text-sm">
              <span>{t("grading.numberScore", { max: item.points })}</span>
              <Input
                type="number"
                min={0}
                max={item.points}
                step={0.5}
                value={number}
                onChange={(event) => setNumber(event.target.value)}
                onBlur={() => {
                  if (
                    number.trim() &&
                    Number(number) >= 0 &&
                    Number(number) <= item.points
                  )
                    onScore(Number(number));
                }}
                aria-invalid={
                  number.trim() !== "" &&
                  (!Number.isFinite(Number(number)) ||
                    Number(number) < 0 ||
                    Number(number) > item.points)
                }
              />
            </label>
          )}
        </fieldset>
        <fieldset disabled={busy} className="space-y-2">
          <label
            htmlFor="grading-comment"
            className="text-muted-foreground text-xs font-medium"
          >
            {t("grading.comment")}
          </label>
          <Textarea
            id="grading-comment"
            rows={2}
            value={comment}
            placeholder={t("grading.commentPlaceholder")}
            onChange={(event) => onComment(event.target.value)}
            className="bg-background resize-y rounded-[9px] px-3 py-2.5 text-sm"
          />
          <div className="flex flex-wrap gap-1.5">
            {[0, 1, 2, 3].map((index) => {
              const text = t(`grading.snippets.${snippets}.${index}`);
              return (
                <Button
                  key={index}
                  type="button"
                  variant="outline"
                  size="sm"
                  className="text-muted-foreground h-auto rounded-full px-2.5 py-1 text-xs"
                  onClick={() => onComment(`${comment.trim()} ${text}.`.trim())}
                >
                  {text}
                </Button>
              );
            })}
          </div>
        </fieldset>
      </div>
      <div className="mt-auto flex flex-wrap items-center justify-between gap-2.5 border-t px-5 py-3">
        <span className="text-muted-foreground text-[12.5px]">
          {t("grading.position", {
            i: position,
            n: total,
            name: givenName(item.studentName),
          })}
          <span className="ml-2.5 hidden min-[768px]:inline">
            {t("grading.moveKeys")}
          </span>
        </span>
        <div className="flex flex-wrap gap-2">
          <Button asChild variant="ghost">
            <Link
              to={`/teacher/attempts/${item.attemptId}`}
              aria-disabled={busy}
              onClick={onReview}
            >
              {t("grading.openReview")}
            </Link>
          </Button>
          <Button
            variant="outline"
            disabled={busy || position <= 1}
            onClick={onPrevious}
          >
            {t("common.previous")}
          </Button>
          <Button disabled={busy} onClick={onNext}>
            {t(finishReady ? "review.finish" : "grading.saveNext")}
          </Button>
        </div>
      </div>
    </section>
  );
}
