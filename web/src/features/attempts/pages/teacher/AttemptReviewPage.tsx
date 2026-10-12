import { ReviewGroup } from "@/features/media";
import { QuestionProse } from "@/components/shared/content/QuestionProse";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router";
import {
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
} from "@tanstack/react-query";
import type { TFunction } from "i18next";
import { Eye, Flag, FlagOff, Headphones, Rows3 } from "lucide-react";
import { Callout } from "@/components/shared/Callout";
import { EmptyState, LoadError } from "@/components/shared/ListState";
import { Avatar } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "@/components/ui/sonner";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Timeline } from "@/features/integrity/components/Timeline";
import { clockSpan } from "@/features/integrity/timeline";
import { FLAGGED } from "@/features/integrity/tones";
import { scoreText } from "@/features/assignments/studentTime";
import { PageHead } from "@/layouts/shell/PageHead";
import { useCrumbs } from "@/layouts/shell/crumbs";
import { ApiError, failureMessage } from "@/lib/api/errors";
import { useLocale } from "@/lib/i18n/useLocale";
import { formatTime, useDisplayTimeZone } from "@/lib/i18n/datetime";
import { cn } from "@/lib/utils";
import {
  finishGrading,
  flagAttempt,
  getAttemptForReview,
  gradeAttempt,
  type AdminQuestion,
  type AttemptReview,
  type ReviewAnswer,
} from "../../api";
import { monitorKey, reviewKey } from "../../keys";
import { AnswerReview } from "../../components/AnswerReview";
import { GradeByQuestion } from "../../components/GradeByQuestion";
import { GradingCard } from "../../components/GradingCard";
import { DOT, type Verdict } from "../../components/answerStyles";

type Tab = "paper" | "integrity";

/**
 * AttemptReviewPage is one student's paper in the teacher shell: the answers
 * with a rail of questions and the grading card, Finish grading, the flag, and
 * an Integrity tab with the full timeline and the private note.
 */
export default function AttemptReviewPage() {
  useDisplayTimeZone();
  const { t } = useTranslation();
  const { id = "" } = useParams<{ id: string }>();
  const queryClient = useQueryClient();
  const locale = useLocale();
  const [tab, setTab] = useState<Tab>("paper");
  const [byQuestion, setByQuestion] = useState(false);
  const [picked, setCurrent] = useState<number | null>(null);
  const [pointsFor, setPointsFor] = useState<string | null>(null);
  const [failure, setFailure] = useState<string | null>(null);

  const review = useQuery({
    queryKey: reviewKey(id),
    queryFn: ({ signal }) => getAttemptForReview(id, signal),
  });

  const invalidate = () =>
    invalidateReview(queryClient, id, review.data?.attempt.assignmentId);
  const grade = useMutation({
    mutationFn: ({
      questionId,
      points,
      comment,
    }: {
      questionId: string;
      points: number;
      comment: string | null;
    }) => gradeAttempt(id, [{ questionId, points, comment }]),
    onSuccess: async () => {
      setFailure(null);
      await invalidate();
    },
    onError: (cause) => setFailure(failureMessage(cause, t("review.saveFailed"))),
  });
  const flag = useMutation({
    mutationFn: (flagged: boolean) => flagAttempt(id, { flagged }),
    onSuccess: async (_, flagged) => {
      toast(t(flagToastKey(flagged)));
      await invalidate();
      await queryClient.invalidateQueries({ queryKey: ["admin-attempts"] });
    },
    onError: (cause) => setFailure(failureMessage(cause, t("review.flagFailed"))),
  });
  const finish = useMutation({
    mutationFn: () => finishGrading(id),
    onSuccess: async () => {
      toast(t("review.finished"));
      await invalidate();
    },
    onError: (cause) => setFailure(failureMessage(cause, t("review.finishFailed"))),
  });

  const data = review.data;
  const questions = data?.questions ?? [];
  const pendingIndexes = pendingIndexesOf(questions, data?.answers ?? {});
  const current = picked ?? firstIndex(pendingIndexes, questions.length);
  useCrumbs(
    data === undefined
      ? null
      : [
          {
            label: data.testTitle,
            to: `/teacher/assignments/${data.attempt.assignmentId}`,
          },
          { label: data.student.fullName },
        ],
  );

  if (review.isPending) return <ReviewSkeleton />;
  if (review.isError || data === undefined)
    return (
      <ReviewUnavailable error={review.error} onRetry={() => void review.refetch()} />
    );

  const { attempt, student } = data;
  const pending = pendingIndexes.length;
  const score = attempt.score;
  const live = attempt.status === "in_progress";
  const gradable = !live && attempt.status !== "voided";
  const verdicts = questions.map((q) => verdictOf(q, data.answers[q.id]));
  const { question, answer } = at(questions, data.answers, current);
  const group = data.sharedContext?.groups.find((item) =>
    item.questionIds.includes(question?.id ?? ""),
  );
  const numbers = new Map(questions.map((item, index) => [item.id, index + 1]));

  const next = () => {
    if (current === null) return;
    const to = nextIndex(current, pendingIndexes, questions.length);
    setCurrent(to);
    setPointsFor(questions[to]?.id ?? null);
  };
  const pick = (index: number) => {
    setCurrent(index);
    setPointsFor(null);
  };

  const manualIds = questions.filter((q) => q.type === "short_answer").map((q) => q.id);
  if (byQuestion && manualIds.length > 0) {
    const from = startManualId(question, manualIds);
    return (
      <GradeByQuestion
        assignmentId={attempt.assignmentId}
        testTitle={data.testTitle}
        initialQuestionId={from}
        onExit={() => setByQuestion(false)}
      />
    );
  }

  const flagged = attempt.integrity?.flagged === true;
  return (
    <div className="flex min-w-0 flex-col gap-4">
      <PageHead
        title={student.fullName}
        back={{
          to: `/teacher/assignments/${attempt.assignmentId}`,
          label: t("review.backToAssignment"),
        }}
        status={
          <>
            {attempt.status === "graded" && (
              <Badge variant="success">{t("status.attempt.graded")}</Badge>
            )}
            {flagged && <FlaggedPill />}
          </>
        }
        actions={
          <>
            {tab === "paper" && score && (
              <span className="text-ui self-center tabular-nums">
                <span className="font-semibold">
                  {scoreText(score.earned, score.total, locale, t).split("/")[0]}
                </span>
                <span className="text-muted-fg">/{score.total}</span>
              </span>
            )}
            {tab === "paper" && pending > 0 && gradable && (
              <Badge variant="outline" className="self-center">
                {t("review.pendingBadge", { count: pending })}
              </Badge>
            )}
            <Button
              variant={flagged ? "ghost" : "outline"}
              size="sm"
              aria-label={t(flagged ? "review.unflag" : "review.flag")}
              disabled={attempt.status === "voided"}
              aria-disabled={flag.isPending || undefined}
              className={cn(flag.isPending && "opacity-50")}
              onClick={() => {
                if (!flag.isPending) flag.mutate(!flagged);
              }}
            >
              {flagged ? <FlagOff aria-hidden="true" /> : <Flag aria-hidden="true" />}
              <span className="hidden lg:inline">
                {t(flagged ? "review.unflag" : "review.flag")}
              </span>
            </Button>
            {tab === "paper" && (
              <Button
                size="sm"
                disabled={!gradable || pending > 0}
                aria-disabled={finish.isPending || undefined}
                className={cn(finish.isPending && "opacity-50")}
                onClick={() => {
                  if (!finish.isPending) finish.mutate();
                }}
              >
                {t("review.finish")}
              </Button>
            )}
          </>
        }
      >
        <p className="text-muted-fg flex min-w-0 items-center gap-2 text-sm">
          <Avatar name={student.fullName} className="text-2xs size-6 flex-none" />
          <span className="min-w-0 [overflow-wrap:anywhere]">
            {headerMeta(data, t)}
          </span>
        </p>
      </PageHead>

      <Tabs value={tab} onValueChange={(value) => setTab(value as Tab)}>
        <TabsList aria-label={t("review.tabsLabel")}>
          <TabsTrigger value="paper">{t("review.tabs.paper")}</TabsTrigger>
          <TabsTrigger value="integrity">{t("review.tabs.integrity")}</TabsTrigger>
        </TabsList>
        <TabsContent value="integrity" className="pt-4.5">
          <Timeline
            attemptId={attempt.id}
            questions={questions}
            live={live}
            note={data.teacherNote}
            onViewPaper={() => setTab("paper")}
          />
        </TabsContent>
        <TabsContent
          value="paper"
          className="grid gap-4 pt-4.5 lg:grid-cols-[16.25rem_minmax(0,1fr)] lg:items-start"
        >
          <aside
            aria-label={t("review.rail")}
            className="bg-card shadow-card flex min-w-0 flex-col gap-4 rounded-xl border p-4"
          >
            <div>
              <p className="text-muted-fg mb-2 text-xs font-medium">
                {t("review.questions")}
              </p>
              <div className="grid grid-cols-[repeat(auto-fill,minmax(2.25rem,1fr))] gap-1.5">
                {questions.map((q, i) => (
                  <button
                    key={q.id}
                    type="button"
                    aria-current={i === current ? "true" : undefined}
                    aria-label={dotLabel(i, verdicts[i] ?? "unanswered", t)}
                    onClick={() => pick(i)}
                    className={cn(
                      DOT.base,
                      "h-9",
                      verdicts[i] === "correct" && DOT.correct,
                      verdicts[i] === "wrong" && DOT.wrong,
                      i === current && DOT.current,
                    )}
                  >
                    {i + 1}
                  </button>
                ))}
              </div>
              {pending > 0 && (
                <p className="text-muted-fg mt-2 text-xs">
                  {t("review.pendingNote", { count: pending })}
                </p>
              )}
              {manualIds.length > 0 && gradable && (
                <Button
                  variant="outline"
                  size="sm"
                  className="mt-3 w-full"
                  onClick={() => setByQuestion(true)}
                >
                  <Rows3 aria-hidden="true" />
                  {t("byQuestion.title")}
                </Button>
              )}
            </div>
            <div className="border-t pt-4">
              <RailStats data={data} />
            </div>
          </aside>

          <div className="min-w-0 space-y-4">
            {failure !== null && (
              <Callout tone="danger" announce>
                {failure}
              </Callout>
            )}

            {question === null ? (
              <EmptyState>{t("review.noQuestions")}</EmptyState>
            ) : (
              <>
                <div className="flex flex-wrap items-center gap-2">
                  <span className="text-muted-fg text-xs">
                    {t("review.questionMeta", {
                      n: (current ?? 0) + 1,
                      type: t(`questionEditor.type.${question.type}`, {
                        defaultValue: question.type,
                      }),
                      points: question.points,
                    })}
                  </span>
                  <VerdictBadge verdict={verdicts[current ?? 0] ?? "unanswered"} />
                </div>

                {group && data.sharedContext && (
                  <ReviewGroup
                    key={group.id}
                    group={group}
                    numbers={numbers}
                    transcripts={data.sharedContext.transcripts}
                    plays={data.sharedContext.audioPlays}
                    onRetry={() => void review.refetch()}
                    onQuestion={(questionId) => {
                      const number = numbers.get(questionId);
                      if (number !== undefined) pick(number - 1);
                      document.getElementById("review-answer")?.focus();
                    }}
                  />
                )}
                <Card id="review-answer" tabIndex={-1}>
                  <CardContent className="space-y-4">
                    <AnswerReview question={question} answer={answer} />
                    {question.type === "short_answer" &&
                      question.sampleAnswer != null && (
                        <details open className="bg-muted rounded-lg border p-4">
                          <summary className="text-muted-fg flex w-fit cursor-pointer items-center gap-1.5 rounded-sm text-xs">
                            <Eye className="size-3.5" aria-hidden="true" />
                            {t("review.sampleAnswer")}
                          </summary>
                          <p className="mt-2.5 text-sm leading-relaxed [overflow-wrap:anywhere] whitespace-pre-wrap">
                            {question.sampleAnswer}
                          </p>
                          {question.explanation != null && (
                            <QuestionProse
                              className="text-muted-fg mt-2 text-xs"
                              text={question.explanation}
                              content={question.explanationContent}
                            />
                          )}
                        </details>
                      )}
                    {question.audio && (
                      <AudioNote
                        question={question}
                        plays={data.audioPlays[question.id] ?? 0}
                      />
                    )}
                  </CardContent>
                </Card>

                {question.type === "short_answer" && gradable && (
                  <GradingCard
                    key={question.id}
                    question={question}
                    answer={answer}
                    pending={grade.isPending}
                    error={null}
                    focusPoints={pointsFor === question.id}
                    onPointsFocused={() => setPointsFor(null)}
                    onSave={(points, comment) =>
                      grade.mutate(
                        { questionId: question.id, points, comment },
                        { onSuccess: next },
                      )
                    }
                    onSkip={next}
                  />
                )}
                {question.type === "short_answer" && !gradable && (
                  <p className="text-muted-fg text-sm">
                    {t(live ? "review.notYetSubmitted" : "review.voided")}
                  </p>
                )}
              </>
            )}
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}

function ReviewUnavailable({
  error,
  onRetry,
}: Readonly<{ error: Error | null; onRetry: () => void }>) {
  const { t } = useTranslation();
  return (
    <div className="flex min-w-0 flex-col gap-4">
      <PageHead
        title={t("review.title")}
        back={{
          to: "/teacher/assignments",
          label: t("teacherShell.nav.assignments"),
        }}
      />
      {error instanceof ApiError && error.status === 404 ? (
        <EmptyState>{t("review.notFound")}</EmptyState>
      ) : (
        <div className="space-y-1">
          <LoadError error={error} onRetry={onRetry}>
            {t("review.loadFailed")}
          </LoadError>
          <p className="text-muted-fg text-xs">{t("review.loadFailedHint")}</p>
        </div>
      )}
    </div>
  );
}

function FlaggedPill() {
  const { t } = useTranslation();
  return (
    <span
      className={cn(
        FLAGGED.soft,
        FLAGGED.ink,
        "inline-flex h-5.5 items-center gap-1 rounded-full px-2 text-xs font-medium whitespace-nowrap",
      )}
    >
      <Flag aria-hidden="true" className="size-3" />
      {t("review.flaggedBadge")}
    </span>
  );
}

function headerMeta(data: AttemptReview, t: TFunction): string {
  const { attempt } = data;
  const parts = [
    data.testTitle,
    t("review.attemptOf", { n: attempt.attemptNo, total: data.maxAttempts }),
  ];
  if (attempt.submittedAt)
    parts.push(t("review.submittedAt", { time: formatTime(attempt.submittedAt) }));
  else if (attempt.status === "in_progress")
    parts.push(t("status.attempt.in_progress").toLocaleLowerCase());
  return parts.join(" · ");
}

function RailStats({ data }: Readonly<{ data: AttemptReview }>) {
  const { t } = useTranslation();
  const { attempt, questions, answers, integrity } = data;
  let autoEarned = 0;
  let autoTotal = 0;
  let manualEarned = 0;
  let manualTotal = 0;
  for (const q of questions) {
    const a = answers[q.id];
    if (q.type === "short_answer") {
      manualTotal += q.points;
      manualEarned += a?.manualScore ?? 0;
    } else {
      autoTotal += q.points;
      autoEarned += a?.manualScore ?? a?.autoScore ?? 0;
    }
  }
  const spent =
    attempt.submittedAt == null
      ? null
      : Math.max(
          1,
          Math.round(
            (new Date(attempt.submittedAt).getTime() -
              new Date(attempt.startedAt).getTime()) /
              60_000,
          ),
        );
  return (
    <div className="space-y-2 text-sm">
      <Line
        label={t("review.stats.auto")}
        value={`${trim(autoEarned)} / ${trim(autoTotal)}`}
      />
      <Line
        label={t("review.stats.manual")}
        value={`${trim(manualEarned)} / ${trim(manualTotal)}`}
      />
      <Line
        label={t("review.stats.duration")}
        value={spent === null ? "—" : t("assignments.minutes", { count: spent })}
      />
      <Line
        label={t("review.stats.focusLoss")}
        value={
          integrity.awayEpisodes === 0
            ? "0"
            : `${integrity.awayEpisodes} · ${clockSpan(integrity.totalAwayMs)}`
        }
      />
    </div>
  );
}

function trim(n: number): string {
  return String(Math.round(n * 100) / 100);
}

function Line({ label, value }: Readonly<{ label: string; value: string }>) {
  return (
    <div className="flex justify-between gap-2">
      <span className="text-muted-fg">{label}</span>
      <span className="tabular-nums">{value}</span>
    </div>
  );
}

function AudioNote({
  question,
  plays,
}: Readonly<{ question: AdminQuestion; plays: number }>) {
  const { t } = useTranslation();
  const max = question.audio?.maxPlays ?? null;
  const over = max !== null && plays > max;
  return (
    <div className="flex flex-wrap items-center gap-2">
      <Badge variant="outline">
        <Headphones aria-hidden="true" />
        {max === null
          ? t("review.playsUnlimited", { plays })
          : t("review.playsOf", { plays, max })}
      </Badge>
      {over && (
        <p className="text-muted-fg text-xs leading-relaxed">
          {t("review.overLimitNote")}
        </p>
      )}
    </div>
  );
}

function VerdictBadge({ verdict }: Readonly<{ verdict: Verdict }>) {
  const { t } = useTranslation();
  const variant = VERDICT_VARIANT[verdict] ?? "outline";
  return (
    <Badge variant={variant} className="ml-auto">
      {t(`review.verdict.${verdict}`)}
    </Badge>
  );
}

function isPending(answer: ReviewAnswer | undefined): boolean {
  return answer?.requiresManual === true && answer.manualScore == null;
}

function verdictOf(question: AdminQuestion, answer: ReviewAnswer | undefined): Verdict {
  if (answer === undefined || answer.answer === null) return "unanswered";
  if (isPending(answer)) return "pending";
  const earned = answer.manualScore ?? answer.autoScore ?? 0;
  if (earned >= question.points) return "correct";
  if (earned > 0) return "partial";
  return "wrong";
}

const VERDICT_VARIANT: Partial<Record<Verdict, "success" | "danger" | "warning">> = {
  correct: "success",
  wrong: "danger",
  partial: "warning",
};

function dotLabel(index: number, verdict: Verdict, t: TFunction): string {
  const verdictLabel = t(`review.verdict.${verdict}`);
  return `${t("takeTest.dotLabel", { n: index + 1 })}, ${verdictLabel}`;
}

function ReviewSkeleton() {
  const { t } = useTranslation();
  return (
    <div
      role="status"
      aria-live="polite"
      aria-label={t("common.loading")}
      className="space-y-4"
    >
      <div className="flex items-center gap-2">
        <Skeleton className="h-4 w-40" />
        <Skeleton className="ml-auto h-5 w-16 rounded-full" />
      </div>
      <Card>
        <CardContent className="space-y-4">
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-56" />
          <div className="space-y-2 rounded-md border p-4">
            <Skeleton className="h-3 w-28" />
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-40" />
          </div>
        </CardContent>
      </Card>
      <Card>
        <CardContent className="space-y-3">
          <Skeleton className="h-9 w-40" />
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-9 w-40" />
        </CardContent>
      </Card>
    </div>
  );
}

function pendingIndexesOf(
  questions: readonly { readonly id: string }[],
  answers: Readonly<Record<string, Parameters<typeof isPending>[0]>>,
): number[] {
  return questions
    .map((q, i) => (isPending(answers[q.id]) ? i : -1))
    .filter((i) => i >= 0);
}

/** Until the teacher picks, the first thing that needs a person, else the first question. */
function firstIndex(pendingIndexes: readonly number[], total: number): number | null {
  return pendingIndexes[0] ?? (total > 0 ? 0 : null);
}

/** The next unmarked answer after this one, wrapping, else simply the next question. */
function nextIndex(
  current: number,
  pendingIndexes: readonly number[],
  total: number,
): number {
  const after = pendingIndexes.find((i) => i > current) ?? pendingIndexes[0];
  return after ?? Math.min(current + 1, total - 1);
}

/** A change to this paper is also a change to its assignment's monitor and the dashboard. */
async function invalidateReview(
  queryClient: QueryClient,
  id: string,
  assignmentId: string | undefined,
) {
  await queryClient.invalidateQueries({ queryKey: reviewKey(id) });
  if (assignmentId !== undefined) {
    await queryClient.invalidateQueries({ queryKey: monitorKey(assignmentId) });
  }
  await queryClient.invalidateQueries({ queryKey: ["admin-dashboard"] });
}

function flagToastKey(flagged: boolean): string {
  return flagged ? "review.flaggedToast" : "review.unflaggedToast";
}

function at<Q extends { readonly id: string }, A>(
  questions: readonly Q[],
  answers: Readonly<Record<string, A>>,
  index: number | null,
): { question: Q | null; answer: A | undefined } {
  const question = index === null ? null : (questions[index] ?? null);
  return { question, answer: question === null ? undefined : answers[question.id] };
}

/** G-04 opens on this paper's essay when the teacher was already looking at one. */
function startManualId(
  question: { readonly id: string; readonly type: string } | null,
  manualIds: readonly string[],
): string {
  return question?.type === "short_answer" ? question.id : manualIds[0]!;
}
