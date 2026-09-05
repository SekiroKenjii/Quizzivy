import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowRight, Check, Eye, EyeOff, Minus, Plus } from "lucide-react";
import { EmptyState, ListSkeleton, LoadError } from "@/components/shared/ListState";
import { Markdown } from "@/components/shared/Markdown";
import { PageHeader } from "@/components/shared/PageHeader";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { ApiError } from "@/lib/api/errors";
import { cn } from "@/lib/utils";
import { gradeAttempt, listAnswersForQuestion, type QuestionAnswerRow } from "../api";
import { answersKey, monitorKey, reviewKey } from "../keys";

/**
 * G-04: one question across every paper, rubric pinned on the left, names
 * hidden until the question is graded. Same grading write as G-03, one paper
 * per call; what changes is the order the teacher walks in.
 */
export function GradeByQuestion({
  assignmentId,
  testTitle,
  initialQuestionId,
  onExit,
}: {
  assignmentId: string;
  testTitle: string;
  initialQuestionId: string;
  onExit: () => void;
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [questionId, setQuestionId] = useState(initialQuestionId);
  const [showNames, setShowNames] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);
  const inputs = useRef(new Map<string, HTMLInputElement>());

  const answers = useQuery({
    queryKey: answersKey(assignmentId, questionId),
    queryFn: ({ signal }) => listAnswersForQuestion(assignmentId, questionId, signal),
  });

  const grade = useMutation({
    mutationFn: ({ row, points }: { row: QuestionAnswerRow; points: number }) =>
      gradeAttempt(row.attemptId, [
        { questionId, points, comment: row.graderComment ?? null },
      ]),
    onSuccess: async (_, { row }) => {
      setFailure(null);
      await queryClient.invalidateQueries({
        queryKey: answersKey(assignmentId, questionId),
      });
      await queryClient.invalidateQueries({ queryKey: reviewKey(row.attemptId) });
      await queryClient.invalidateQueries({ queryKey: monitorKey(assignmentId) });
      await queryClient.invalidateQueries({ queryKey: ["admin-dashboard"] });
      await queryClient.invalidateQueries({ queryKey: ["admin-attempts"] });
    },
    onError: (cause) =>
      setFailure(cause instanceof ApiError ? cause.message : t("review.saveFailed")),
  });

  const data = answers.data;
  const manual = data?.manualQuestionIds ?? [];
  const at = manual.indexOf(questionId);
  const nextId = at >= 0 ? manual[at + 1] : undefined;
  const items = data?.items ?? [];
  const gradable = items.filter((row) => row.answer !== null);
  const done = gradable.filter((row) => row.manualScore != null).length;
  // The board's fairness rule: names come back once nothing is left to judge.
  const revealed = showNames || (gradable.length > 0 && done === gradable.length);

  const focusRow = (index: number) => {
    const row = items[index];
    if (row) inputs.current.get(row.attemptId)?.focus();
  };
  const goNext = () => {
    if (nextId === undefined) return;
    setQuestionId(nextId);
    setShowNames(false);
  };

  return (
    <>
      <PageHeader
        title={t("byQuestion.title")}
        backTo={`/admin/assignments/${assignmentId}`}
        meta={
          data ? (
            <span className="text-muted-foreground text-xs">
              {testTitle} ·{" "}
              {t("byQuestion.questionOf", {
                n: data.questionNumber,
                count: data.questionCount,
              })}
            </span>
          ) : null
        }
        actions={
          <>
            {data ? (
              <>
                <span className="text-muted-foreground text-xs tabular-nums">
                  {t("byQuestion.graded", { done, total: gradable.length })}
                </span>
                <span
                  className="bg-secondary block h-1.5 w-32 overflow-hidden rounded-full"
                  role="img"
                  aria-label={t("byQuestion.graded", { done, total: gradable.length })}
                >
                  <span
                    className="bg-foreground block h-full rounded-full"
                    style={{
                      width: `${gradable.length === 0 ? 0 : Math.round((done / gradable.length) * 100)}%`,
                    }}
                  />
                </span>
              </>
            ) : null}
            <Button
              variant="outline"
              size="sm"
              disabled={nextId === undefined}
              onClick={goNext}
            >
              {t("byQuestion.next")}
              <ArrowRight aria-hidden="true" />
            </Button>
            <Button variant="ghost" size="sm" onClick={onExit}>
              {t("byQuestion.exit")}
            </Button>
          </>
        }
      />

      {answers.isPending ? (
        <ListSkeleton rows={6} />
      ) : answers.isError || data === undefined ? (
        <LoadError error={answers.error} onRetry={() => void answers.refetch()}>
          {t("byQuestion.loadFailed")}
        </LoadError>
      ) : data.question.type !== "short_answer" ? (
        <EmptyState
          action={
            nextId === undefined ? undefined : (
              <Button size="sm" onClick={goNext}>
                {t("byQuestion.next")}
              </Button>
            )
          }
        >
          {t("byQuestion.notManual")}
        </EmptyState>
      ) : (
        <div className="grid gap-5 lg:grid-cols-3">
          <div className="space-y-4 lg:sticky lg:top-4 lg:self-start">
            <p className="text-muted-foreground text-xs">
              {t("byQuestion.meta", {
                n: data.questionNumber,
                type: t(`questionEditor.type.${data.question.type}`, {
                  defaultValue: data.question.type,
                }),
                points: data.question.points,
              })}
            </p>
            <Markdown className="text-sm">{data.question.prompt}</Markdown>
            {data.question.sampleAnswer != null && (
              <div className="bg-muted/30 rounded-md border p-4">
                <p className="text-muted-foreground flex items-center gap-1.5 text-xs">
                  <Eye className="size-3.5" aria-hidden="true" />
                  {t("review.sampleAnswer")}
                </p>
                <p className="mt-2 text-sm leading-relaxed whitespace-pre-wrap">
                  {data.question.sampleAnswer}
                </p>
                {data.question.explanation != null && (
                  <p className="text-muted-foreground mt-2 text-xs">
                    {t("byQuestion.rubric")}: {data.question.explanation}
                  </p>
                )}
              </div>
            )}
            <div className="flex items-start justify-between gap-3 rounded-md border p-3">
              <p className="text-muted-foreground text-xs leading-relaxed">
                {t("byQuestion.namesHidden")}
              </p>
              <Button
                variant="outline"
                size="xs"
                className="shrink-0"
                aria-pressed={showNames}
                onClick={() => setShowNames((value) => !value)}
              >
                {showNames ? <EyeOff aria-hidden="true" /> : <Eye aria-hidden="true" />}
                {showNames ? t("byQuestion.hideNames") : t("byQuestion.showNames")}
              </Button>
            </div>
            <p className="text-muted-foreground text-xs">
              {t("byQuestion.keys", { max: data.question.points })}
            </p>
          </div>

          <div className="space-y-3 lg:col-span-2">
            {failure !== null && (
              <p role="alert" className="text-destructive text-sm">
                {failure}
              </p>
            )}
            {items.length === 0 ? (
              <EmptyState>{t("byQuestion.empty")}</EmptyState>
            ) : (
              items.map((row, index) => (
                <AnswerRow
                  key={row.attemptId}
                  row={row}
                  label={
                    revealed
                      ? row.studentName
                      : t("byQuestion.student", {
                          n: String(index + 1).padStart(2, "0"),
                        })
                  }
                  max={data.question.points}
                  pending={grade.isPending}
                  inputRef={(el) => {
                    if (el) inputs.current.set(row.attemptId, el);
                    else inputs.current.delete(row.attemptId);
                  }}
                  onSave={(points, andNext) =>
                    grade.mutate(
                      { row, points },
                      { onSuccess: () => (andNext ? goNext() : focusRow(index + 1)) },
                    )
                  }
                  onMove={(delta) => focusRow(index + delta)}
                />
              ))
            )}
          </div>
        </div>
      )}
    </>
  );
}

function AnswerRow({
  row,
  label,
  max,
  pending,
  inputRef,
  onSave,
  onMove,
}: {
  row: QuestionAnswerRow;
  label: string;
  max: number;
  pending: boolean;
  inputRef: (el: HTMLInputElement | null) => void;
  onSave: (points: number, andNext: boolean) => void;
  onMove: (delta: -1 | 1) => void;
}) {
  const { t } = useTranslation();
  const [points, setPoints] = useState(
    row.manualScore == null ? "" : String(row.manualScore),
  );
  const value = Number(points);
  const valid =
    points.trim() !== "" && Number.isFinite(value) && value >= 0 && value <= max;
  const blank = row.answer === null;
  const text = row.answer?.type === "text" ? row.answer.value : null;
  const step = (delta: number) => {
    const next = Math.min(
      max,
      Math.max(0, (Number.isFinite(value) ? value : 0) + delta),
    );
    setPoints(String(Math.round(next * 100) / 100));
  };

  return (
    <Card className={cn("gap-0", blank && "opacity-70")}>
      <CardContent className="space-y-3 py-4">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium">{label}</span>
          {row.attemptNo > 1 && (
            <span className="text-muted-foreground text-xs">
              {t("byQuestion.attemptNo", { n: row.attemptNo })}
            </span>
          )}
          {row.manualScore != null && (
            <Badge variant="success" className="ml-auto">
              {t("byQuestion.gradedBadge")}
            </Badge>
          )}
        </div>
        {blank ? (
          <p className="text-muted-foreground text-sm">{t("byQuestion.blank")}</p>
        ) : (
          <p className="text-sm leading-relaxed whitespace-pre-wrap">{text ?? ""}</p>
        )}
        {!blank && (
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="icon-xs"
              aria-label={t("review.decrease")}
              onClick={() => step(-1)}
            >
              <Minus aria-hidden="true" />
            </Button>
            <Input
              ref={inputRef}
              type="number"
              inputMode="decimal"
              min={0}
              max={max}
              step={0.5}
              className="w-20 text-center tabular-nums"
              aria-label={t("byQuestion.pointsFor", { who: label })}
              value={points}
              onChange={(event) => setPoints(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "ArrowDown" || event.key === "ArrowUp") {
                  event.preventDefault();
                  onMove(event.key === "ArrowDown" ? 1 : -1);
                } else if (event.key === "Enter" && valid) {
                  event.preventDefault();
                  onSave(value, event.metaKey || event.ctrlKey);
                }
              }}
            />
            <Button
              variant="outline"
              size="icon-xs"
              aria-label={t("review.increase")}
              onClick={() => step(1)}
            >
              <Plus aria-hidden="true" />
            </Button>
            <span className="text-muted-foreground text-sm tabular-nums">/{max}</span>
            <Button
              size="sm"
              className="ml-auto"
              disabled={!valid || pending}
              onClick={() => onSave(value, false)}
            >
              <Check aria-hidden="true" />
              {t("byQuestion.save")}
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
