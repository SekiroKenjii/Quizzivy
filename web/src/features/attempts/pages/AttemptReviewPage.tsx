import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { TFunction } from "i18next";
import { Eye, Flag, FlagOff, Headphones, Rows3 } from "lucide-react";
import { EmptyState, LoadError } from "@/components/shared/ListState";
import { PageAside } from "@/components/shared/PageAside";
import { PageHeader } from "@/components/shared/PageHeader";
import { Avatar } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "@/components/ui/sonner";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Timeline } from "@/features/integrity/components/Timeline";
import { clockSpan } from "@/features/integrity/timeline";
import { scoreText } from "@/features/assignments/studentTime";
import { ApiError } from "@/lib/api/errors";
import { useLocale } from "@/lib/i18n/useLocale";
import { formatTime } from "@/lib/i18n/datetime";
import { cn } from "@/lib/utils";
import {
  finishGrading,
  flagAttempt,
  getAttemptForReview,
  gradeAttempt,
  type AdminQuestion,
  type AttemptReview,
  type ReviewAnswer,
} from "../api";
import { monitorKey, reviewKey } from "../keys";
import { AnswerReview } from "../components/AnswerReview";
import { GradeByQuestion } from "../components/GradeByQuestion";
import { GradingCard } from "../components/GradingCard";
import { DOT, type Verdict } from "../components/answerStyles";

type Tab = "paper" | "integrity";

/** G-03: read, award, comment, next -- with the integrity tab beside it (G-05). */
export default function AttemptReviewPage() {
  const { t } = useTranslation();
  const { id = "" } = useParams<{ id: string }>();
  const queryClient = useQueryClient();
  const locale = useLocale();
  const [tab, setTab] = useState<Tab>("paper");
  const [byQuestion, setByQuestion] = useState(false);
  const [picked, setCurrent] = useState<number | null>(null);
  const [failure, setFailure] = useState<string | null>(null);

  const review = useQuery({
    queryKey: reviewKey(id),
    queryFn: ({ signal }) => getAttemptForReview(id, signal),
  });

  const invalidate = async () => {
    await queryClient.invalidateQueries({ queryKey: reviewKey(id) });
    if (review.data) {
      await queryClient.invalidateQueries({
        queryKey: monitorKey(review.data.attempt.assignmentId),
      });
    }
    await queryClient.invalidateQueries({ queryKey: ["admin-dashboard"] });
  };
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
    onError: (cause) =>
      setFailure(cause instanceof ApiError ? cause.message : t("review.saveFailed")),
  });
  const flag = useMutation({
    mutationFn: (flagged: boolean) => flagAttempt(id, { flagged }),
    onSuccess: async (_, flagged) => {
      toast(t(flagged ? "review.flaggedToast" : "review.unflaggedToast"));
      await invalidate();
      await queryClient.invalidateQueries({ queryKey: ["admin-attempts"] });
    },
    onError: (cause) =>
      setFailure(cause instanceof ApiError ? cause.message : t("review.flagFailed")),
  });
  const finish = useMutation({
    mutationFn: () => finishGrading(id),
    onSuccess: async () => {
      toast(t("review.finished"));
      await invalidate();
    },
    onError: (cause) =>
      setFailure(cause instanceof ApiError ? cause.message : t("review.finishFailed")),
  });

  const data = review.data;
  const questions = data?.questions ?? [];
  const pendingIndexes = questions
    .map((q, i) => (isPending(data?.answers[q.id]) ? i : -1))
    .filter((i) => i >= 0);
  // Until the teacher picks, the first thing that needs a person, else the first question.
  const current = picked ?? pendingIndexes[0] ?? (questions.length > 0 ? 0 : null);

  if (review.isPending) return <ReviewSkeleton />;
  if (review.isError || data === undefined) {
    return (
      <>
        <PageHeader title={t("review.title")} backTo="/admin/assignments" />
        <div className="space-y-1">
          <LoadError error={review.error} onRetry={() => void review.refetch()}>
            {t("review.loadFailed")}
          </LoadError>
          <p className="text-muted-foreground text-xs">{t("review.loadFailedHint")}</p>
        </div>
      </>
    );
  }

  const { attempt, student } = data;
  const pending = pendingIndexes.length;
  const score = attempt.score;
  const live = attempt.status === "in_progress";
  const gradable = !live && attempt.status !== "voided";
  const verdicts = questions.map((q) => verdictOf(q, data.answers[q.id]));
  const question = current === null ? null : (questions[current] ?? null);
  const answer = question === null ? undefined : data.answers[question.id];

  const next = () => {
    if (current === null) return;
    const after = pendingIndexes.find((i) => i > current) ?? pendingIndexes[0];
    setCurrent(after ?? Math.min(current + 1, questions.length - 1));
  };

  // G-04: the same route, a toggle; it starts on this paper's essay when there is one.
  const manualIds = questions.filter((q) => q.type === "short_answer").map((q) => q.id);
  if (byQuestion && manualIds.length > 0) {
    const from =
      question !== null && question.type === "short_answer"
        ? question.id
        : manualIds[0]!;
    return (
      <GradeByQuestion
        assignmentId={attempt.assignmentId}
        testTitle={data.testTitle}
        initialQuestionId={from}
        onExit={() => setByQuestion(false)}
      />
    );
  }

  return (
    <>
      <PageHeader
        title={student.fullName}
        backTo={`/admin/assignments/${attempt.assignmentId}`}
        leading={<Avatar name={student.fullName} size="sm" />}
        meta={
          <>
            {/* F-12: the bar keeps the title and the primary action legible at 768; the rest yields. */}
            <span className="text-muted-foreground hidden text-xs xl:inline">
              {headerMeta(data, t)}
            </span>
            <Tabs
              value={tab}
              onValueChange={(value) => setTab(value as Tab)}
              className="ml-4"
            >
              <TabsList>
                <TabsTrigger value="paper" className="whitespace-nowrap">
                  {t("review.tabs.paper")}
                </TabsTrigger>
                <TabsTrigger value="integrity" className="whitespace-nowrap">
                  {t("review.tabs.integrity")}
                </TabsTrigger>
              </TabsList>
            </Tabs>
          </>
        }
        actions={
          <>
            {score && (
              <span className="text-sm tabular-nums">
                <span className="font-semibold">
                  {scoreText(score.earned, score.total, locale, t).split("/")[0]}
                </span>
                <span className="text-muted-foreground">/{score.total}</span>
              </span>
            )}
            {pending > 0 && gradable && (
              <Badge variant="outline" className="hidden lg:inline-flex">
                {t("review.pendingBadge", { count: pending })}
              </Badge>
            )}
            {attempt.status === "graded" && (
              <Badge variant="success">{t("status.attempt.graded")}</Badge>
            )}
            {/* G-05: a mark to look again, set or cleared by hand; never a verdict. */}
            {attempt.integrity?.flagged ? (
              <>
                <Badge variant="warning">{t("review.flaggedBadge")}</Badge>
                <Button
                  variant="ghost"
                  size="sm"
                  aria-label={t("review.unflag")}
                  disabled={attempt.status === "voided" || flag.isPending}
                  onClick={() => flag.mutate(false)}
                >
                  <FlagOff aria-hidden="true" />
                  <span className="hidden lg:inline">{t("review.unflag")}</span>
                </Button>
              </>
            ) : (
              <Button
                variant="outline"
                size="sm"
                aria-label={t("review.flag")}
                disabled={attempt.status === "voided" || flag.isPending}
                onClick={() => flag.mutate(true)}
              >
                <Flag aria-hidden="true" />
                <span className="hidden lg:inline">{t("review.flag")}</span>
              </Button>
            )}
            <Button
              size="sm"
              disabled={!gradable || pending > 0 || finish.isPending}
              onClick={() => finish.mutate()}
            >
              {t("review.finish")}
            </Button>
          </>
        }
      />

      {tab === "integrity" ? (
        <Timeline
          attemptId={attempt.id}
          questions={questions}
          live={live}
          note={data.teacherNote}
          onViewPaper={() => setTab("paper")}
        />
      ) : (
        <>
          <PageAside label={t("review.rail")} side="left">
            <div>
              <p className="text-muted-foreground mb-2 text-xs font-medium tracking-wide uppercase">
                {t("review.questions")}
              </p>
              <div className="grid grid-cols-[repeat(auto-fill,minmax(2.25rem,1fr))] gap-1.5">
                {questions.map((q, i) => (
                  <button
                    key={q.id}
                    type="button"
                    aria-current={i === current ? "true" : undefined}
                    aria-label={dotLabel(i, verdicts[i] ?? "unanswered", t)}
                    onClick={() => setCurrent(i)}
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
                <p className="text-muted-foreground mt-2 text-xs">
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
            <Separator />
            <RailStats data={data} />
          </PageAside>

          {failure !== null && (
            <p role="alert" className="text-destructive mb-3 text-sm">
              {failure}
            </p>
          )}

          {question === null ? (
            <EmptyState>{t("review.noQuestions")}</EmptyState>
          ) : (
            <div className="space-y-4">
              <div className="flex items-center gap-2">
                <span className="text-muted-foreground text-xs">
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

              <Card>
                <CardContent className="space-y-4">
                  <AnswerReview question={question} answer={answer} />
                  {question.type === "short_answer" &&
                    question.sampleAnswer != null && (
                      <details open className="bg-muted/30 rounded-md border p-4">
                        <summary className="text-muted-foreground flex cursor-pointer items-center gap-1.5 text-xs">
                          <Eye className="size-3.5" aria-hidden="true" />
                          {t("review.sampleAnswer")}
                        </summary>
                        <p className="mt-2.5 text-sm leading-relaxed whitespace-pre-wrap">
                          {question.sampleAnswer}
                        </p>
                        {question.explanation != null && (
                          <p className="text-muted-foreground mt-2 text-xs">
                            {question.explanation}
                          </p>
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
                <p className="text-muted-foreground text-sm">
                  {t(live ? "review.notYetSubmitted" : "review.voided")}
                </p>
              )}
            </div>
          )}
        </>
      )}
    </>
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

function RailStats({ data }: { data: AttemptReview }) {
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

function Line({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between gap-2">
      <span className="text-muted-foreground">{label}</span>
      <span className="tabular-nums">{value}</span>
    </div>
  );
}

function AudioNote({ question, plays }: { question: AdminQuestion; plays: number }) {
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
        <p className="text-muted-foreground text-xs leading-relaxed">
          {t("review.overLimitNote")}
        </p>
      )}
    </div>
  );
}

function VerdictBadge({ verdict }: { verdict: Verdict }) {
  const { t } = useTranslation();
  const variant =
    verdict === "correct"
      ? "success"
      : verdict === "wrong"
        ? "danger"
        : verdict === "partial"
          ? "warning"
          : "outline";
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

function dotLabel(index: number, verdict: Verdict, t: TFunction): string {
  return `${t("takeTest.dotLabel", { n: index + 1 })}, ${t(`review.verdict.${verdict}`)}`;
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
