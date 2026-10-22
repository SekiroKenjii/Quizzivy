import { ReviewGroup } from "@/features/media";
import { QuestionProse } from "@/components/shared/content/QuestionProse";
import { useEffect, useId, useRef, useState } from "react";
import { Trans, useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowRight, Check, Eye, Minus, Plus } from "lucide-react";
import { EmptyState, ListSkeleton, QueryStates } from "@/components/shared/ListState";
import { KeyGlyph } from "@/components/shared/KeyGlyph";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { commandKeyLabel } from "@/features/search/useCommandPalette";
import { PageHead } from "@/layouts/shell/PageHead";
import { ApiError } from "@/lib/api/errors";
import { nfc } from "@/lib/nfc";
import { cn } from "@/lib/utils";
import {
  answerRemovedRefusal,
  gradeAttempt,
  listAnswersForQuestion,
  type QuestionAnswerRow,
} from "../api";
import { answersKey, monitorKey, reviewKey } from "../keys";
import { gradeFieldMessage } from "../gradeErrors";
import {
  handMarked,
  markBand,
  markFields,
  markOf,
  markSignature,
  type EssayMark,
} from "../essayMark";
import { useConfirmMove, useUnsavedSection } from "@/features/settings/unsavedSection";
import { useLocale } from "@/lib/i18n/useLocale";
import { pointsSignature, useFocusWithin, useSavedSync } from "../savedSync";
import { EssayBands, GradedElsewhere } from "./BandScale";
import type { GradeMarks } from "./GradingCard";

/**
 * G-04: one question across every paper, rubric pinned on the left, names
 * hidden until the question is graded. Same grading write as G-03, one paper
 * per call; what changes is the order the teacher walks in. An answer the
 * retention policy removed shows the notice that says so and no points box.
 */
export function GradeByQuestion({
  assignmentId,
  testTitle,
  initialQuestionId,
  onExit,
}: Readonly<{
  assignmentId: string;
  testTitle: string;
  initialQuestionId: string;
  onExit: () => void;
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const queryClient = useQueryClient();
  const [questionId, setQuestionId] = useState(initialQuestionId);
  const [showNames, setShowNames] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);
  const [fieldFailure, setFieldFailure] = useState<{
    attemptId: string;
    message: string;
  } | null>(null);
  const inputs = useRef(new Map<string, HTMLElement>());
  // Next question and Grade per paper drop unsaved marks, so they ask first.
  const confirmMove = useConfirmMove();

  const answers = useQuery({
    queryKey: answersKey(assignmentId, questionId),
    queryFn: ({ signal }) => listAnswersForQuestion(assignmentId, questionId, signal),
    // Another tab or teacher may grade these papers meanwhile.
    refetchOnWindowFocus: "always",
  });

  const grade = useMutation({
    mutationFn: ({ row, marks }: { row: QuestionAnswerRow; marks: GradeMarks }) =>
      gradeAttempt(row.attemptId, [
        { questionId, ...marks, comment: row.graderComment ?? null },
      ]),
    onMutate: () => setFieldFailure(null),
    onSuccess: async (_, { row }) => {
      setFailure(null);
      await queryClient.invalidateQueries({
        queryKey: answersKey(assignmentId, questionId),
      });
      await queryClient.invalidateQueries({ queryKey: reviewKey(row.attemptId) });
      await queryClient.invalidateQueries({ queryKey: monitorKey(assignmentId) });
      await queryClient.invalidateQueries({ queryKey: ["teacher-dashboard"] });
      await queryClient.invalidateQueries({ queryKey: ["teacher-attempts"] });
    },
    onError: (cause, { row }) => {
      const field = gradeFieldMessage(cause, questionId, t);
      if (field !== null) setFieldFailure({ attemptId: row.attemptId, message: field });
      else
        setFailure(cause instanceof ApiError ? cause.message : t("review.saveFailed"));
      if (answerRemovedRefusal(cause))
        void queryClient.invalidateQueries({
          queryKey: answersKey(assignmentId, questionId),
        });
    },
  });

  const data = answers.data;
  const manual = data?.manualQuestionIds ?? [];
  const at = manual.indexOf(questionId);
  const nextId = at >= 0 ? manual[at + 1] : undefined;
  const items = data?.items ?? [];
  const gradable = items.filter(
    (row) => row.answer !== null && row.answerRemoved !== true,
  );
  const done = gradable.filter((row) => row.manualScore != null).length;
  // The board's fairness rule: names come back once nothing is left to judge.
  const revealed = showNames || (gradable.length > 0 && done === gradable.length);

  const focusRow = (index: number) => {
    const row = items[index];
    if (row) inputs.current.get(row.attemptId)?.focus();
  };
  // Save and next moves on only if no other row holds unsaved marks.
  const nextAfterSave = (row: QuestionAnswerRow) =>
    confirmMove(goNext, (section) => section.startsWith(`g04:${row.attemptId}`));
  const saveRow =
    (row: QuestionAnswerRow, index: number) => (marks: GradeMarks, andNext: boolean) =>
      grade.mutate(
        { row, marks },
        { onSuccess: () => (andNext ? nextAfterSave(row) : focusRow(index + 1)) },
      );
  const goNext = () => {
    if (nextId === undefined) return;
    setQuestionId(nextId);
    setShowNames(false);
  };

  return (
    <div className="flex min-w-0 flex-col gap-4">
      <PageHead
        title={t("byQuestion.title")}
        back={{
          to: `/teacher/assignments/${assignmentId}`,
          label: t("review.backToAssignment"),
        }}
        actions={
          <>
            {data ? (
              <>
                <span className="text-muted-foreground self-center text-xs tabular-nums">
                  {t("byQuestion.graded", { done, total: gradable.length })}
                </span>
                <span
                  className="bg-secondary block h-1.5 w-32 self-center overflow-hidden rounded-full"
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
              aria-disabled={nextId === undefined || undefined}
              onClick={() => {
                if (nextId !== undefined) confirmMove(goNext);
              }}
            >
              {t("byQuestion.next")}
              <ArrowRight aria-hidden="true" />
            </Button>
            <Button variant="ghost" size="sm" onClick={() => confirmMove(onExit)}>
              {t("byQuestion.exit")}
            </Button>
          </>
        }
      >
        {data ? (
          <p className="text-muted-fg text-sm [overflow-wrap:anywhere]">
            {testTitle} ·{" "}
            {t("byQuestion.questionOf", {
              n: data.questionNumber,
              count: data.questionCount,
            })}
          </p>
        ) : null}
      </PageHead>

      <QueryStates
        query={answers}
        skeleton={<ListSkeleton rows={6} />}
        failed={t("byQuestion.loadFailed")}
      >
        {(data) =>
          !handMarked(data.question.type) ? (
            <EmptyState
              action={
                nextId === undefined ? undefined : (
                  <Button size="sm" onClick={() => confirmMove(goNext)}>
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
                    count: data.question.points,
                    points: new Intl.NumberFormat(locale, {
                      maximumFractionDigits: 2,
                    }).format(data.question.points),
                  })}
                </p>
                {data.sharedContext?.groups.map((group) => (
                  <ReviewGroup
                    key={group.id}
                    group={group}
                    numbers={
                      new Map(
                        group.questionIds.map((id, index) => [
                          id,
                          data.questionNumber -
                            group.questionIds.indexOf(data.question.id) +
                            index,
                        ]),
                      )
                    }
                    transcripts={data.sharedContext!.transcripts}
                    onRetry={() => void answers.refetch()}
                  />
                ))}
                <QuestionProse
                  className="text-sm"
                  text={data.question.prompt}
                  content={data.question.promptContent}
                />
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
                      <div className="text-muted-foreground mt-2 text-xs">
                        <p>{t("byQuestion.rubric")}</p>
                        <QuestionProse
                          text={data.question.explanation}
                          content={data.question.explanationContent}
                        />
                      </div>
                    )}
                  </div>
                )}
                <p className="text-muted-foreground rounded-md border p-3 text-xs leading-relaxed">
                  {t("byQuestion.namesHidden")}{" "}
                  <button
                    type="button"
                    className="text-foreground rounded-sm underline underline-offset-4"
                    aria-pressed={showNames}
                    onClick={() => setShowNames((value) => !value)}
                  >
                    {showNames ? t("byQuestion.hideNames") : t("byQuestion.showNames")}
                  </button>
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
                      rubric={
                        data.question.type === "essay"
                          ? (data.question.rubric ?? [])
                          : null
                      }
                      error={
                        fieldFailure?.attemptId === row.attemptId
                          ? fieldFailure.message
                          : null
                      }
                      pending={grade.isPending}
                      inputRef={(el) => {
                        if (el) inputs.current.set(row.attemptId, el);
                        else inputs.current.delete(row.attemptId);
                      }}
                      onSave={saveRow(row, index)}
                      onMove={(delta) => focusRow(index + delta)}
                    />
                  ))
                )}
                {data.question.type === "essay" ? (
                  <p className="text-muted-foreground hidden text-xs min-[768px]:block">
                    {t("grading.bandKeys")}
                  </p>
                ) : (
                  <p className="text-muted-foreground text-xs">
                    <Trans
                      i18nKey="byQuestion.keys"
                      values={{
                        max: data.question.points,
                        modifier: commandKeyLabel(),
                      }}
                      components={{ down: <KeyGlyph name="down" /> }}
                    />
                  </p>
                )}
              </div>
            </div>
          )
        }
      </QueryStates>
    </div>
  );
}

type Rubric = Parameters<typeof markBand>[0];

function AnswerRow({
  row,
  label,
  max,
  rubric,
  error,
  pending,
  inputRef,
  onSave,
  onMove,
}: Readonly<{
  row: QuestionAnswerRow;
  label: string;
  max: number;
  rubric: Rubric | null;
  error: string | null;
  pending: boolean;
  inputRef: (el: HTMLElement | null) => void;
  onSave: (marks: GradeMarks, andNext: boolean) => void;
  onMove: (delta: -1 | 1) => void;
}>) {
  const { t } = useTranslation();
  const labelId = useId();
  const savedPoints = row.manualScore == null ? "" : String(row.manualScore);
  const [points, setPoints] = useState(savedPoints);
  const [holding, hold] = useFocusWithin();
  const sync = useSavedSync(
    savedPoints,
    rubric === null ? pointsSignature(points) : savedPoints,
    () => setPoints(savedPoints),
    holding,
  );
  useUnsavedSection(`g04:${row.attemptId}`, sync.dirty);
  const box = useRef<HTMLInputElement | null>(null);
  const errorId = useId();
  useEffect(() => {
    if (error === null || rubric !== null) return;
    const lost =
      document.activeElement === null || document.activeElement === document.body;
    if (lost) box.current?.focus();
  }, [error, rubric]);
  const value = Number(points);
  const valid =
    points.trim() !== "" && Number.isFinite(value) && value >= 0 && value <= max;
  const removed = row.answerRemoved === true;
  const blank = removed || row.answer === null;
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
          <span id={labelId} className="text-sm font-medium">
            {label}
          </span>
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
        {removed && (
          <div className="flex items-baseline gap-3">
            <p
              data-slot="answer-removed"
              className="text-muted-foreground min-w-0 flex-1 text-sm italic"
            >
              {t("review.answerRemoved")}
            </p>
            {row.manualScore != null && (
              <span className="text-sm tabular-nums">
                {row.manualScore}/{max}
              </span>
            )}
          </div>
        )}
        {blank && !removed && (
          <p className="text-muted-foreground text-sm">{t("byQuestion.blank")}</p>
        )}
        {!blank && (
          <p className="text-sm leading-relaxed whitespace-pre-wrap">
            {nfc(text ?? "")}
          </p>
        )}
        {!blank && rubric !== null && (
          <EssayRowMarks
            row={row}
            owner={labelId}
            rubric={rubric}
            error={error}
            pending={pending}
            inputRef={inputRef}
            onSave={(marks) => onSave(marks, false)}
          />
        )}
        {!blank && rubric === null && sync.elsewhere && (
          <GradedElsewhere onShow={() => setPoints(savedPoints)} />
        )}
        {!blank && rubric === null && (
          <div className="flex items-center gap-2" {...hold}>
            <Button
              variant="outline"
              size="icon-xs"
              aria-label={t("review.decrease")}
              onClick={() => step(-1)}
            >
              <Minus aria-hidden="true" />
            </Button>
            <Input
              ref={(el) => {
                box.current = el;
                inputRef(el);
              }}
              type="number"
              inputMode="decimal"
              min={0}
              max={max}
              step={0.5}
              className="w-20 text-center tabular-nums"
              aria-label={t("byQuestion.pointsFor", { who: label })}
              aria-invalid={error !== null || undefined}
              aria-describedby={error === null ? undefined : errorId}
              value={points}
              onChange={(event) => setPoints(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "ArrowDown" || event.key === "ArrowUp") {
                  event.preventDefault();
                  onMove(event.key === "ArrowDown" ? 1 : -1);
                } else if (event.key === "Enter" && valid) {
                  event.preventDefault();
                  onSave({ points: value }, event.metaKey || event.ctrlKey);
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
              onClick={() => onSave({ points: value }, false)}
            >
              <Check aria-hidden="true" />
              {t("byQuestion.save")}
            </Button>
          </div>
        )}
        {!blank && rubric === null && error !== null && (
          <p id={errorId} role="alert" className="text-destructive text-sm">
            {error}
          </p>
        )}
      </CardContent>
    </Card>
  );
}

/**
 * EssayRowMarks marks one paper's essay in G-04: its band, or a band per
 * rubric criterion with the derived band, and Save once the mark is complete.
 * Before that Save looks paused (R6-00) and, if pressed, says what is missing.
 * Its scales are named after the row (`owner`) as well as the criterion.
 */
function EssayRowMarks({
  row,
  owner,
  rubric,
  error,
  pending,
  inputRef,
  onSave,
}: Readonly<{
  row: QuestionAnswerRow;
  owner: string;
  rubric: Rubric;
  error: string | null;
  pending: boolean;
  inputRef: (el: HTMLElement | null) => void;
  onSave: (marks: GradeMarks) => void;
}>) {
  const { t } = useTranslation();
  const savedMark = markOf(row);
  const [mark, setMark] = useState<EssayMark>(savedMark);
  const [missing, setMissing] = useState<string | null>(null);
  const complete = markBand(rubric, mark) !== null;
  const reset = () => {
    setMark(savedMark);
    setMissing(null);
  };
  const [holding, hold] = useFocusWithin();
  const { dirty, elsewhere } = useSavedSync(
    markSignature(rubric, savedMark),
    markSignature(rubric, mark),
    reset,
    holding,
  );
  useUnsavedSection(`g04:${row.attemptId}:marks`, dirty);
  return (
    <div
      className="space-y-3"
      {...hold}
      ref={(el) => inputRef(el?.querySelector<HTMLElement>("[data-score]") ?? null)}
    >
      <EssayBands
        rubric={rubric}
        mark={mark}
        error={error ?? missing}
        owner={owner}
        keysHint={false}
        onChange={(next) => {
          setMissing(null);
          setMark(next);
        }}
      />
      {elsewhere && <GradedElsewhere onShow={reset} />}
      <div className="flex justify-end">
        <Button
          size="sm"
          disabled={pending}
          aria-disabled={!complete}
          onClick={() => {
            if (complete) onSave(markFields(rubric, mark));
            else
              setMissing(
                t(
                  rubric.length ? "grading.pickCriteriaFirst" : "grading.pickBandFirst",
                ),
              );
          }}
        >
          <Check aria-hidden="true" />
          {t("byQuestion.save")}
        </Button>
      </div>
    </div>
  );
}
