import { RichBlankPrompt } from "@/components/shared/content/RichBlankPrompt";
import { QuestionProse } from "@/components/shared/content/QuestionProse";
import { OptionText } from "@/components/shared/content/OptionText";
import { useEffect, useReducer, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import {
  CircleAlert,
  Headphones,
  Info,
  Lock,
  Pencil,
  RefreshCw,
  ScrollText,
} from "lucide-react";
import { EmptyState } from "@/components/shared/ListState";
import { BackLink } from "@/components/shared/BackLink";
import { Markdown } from "@/components/shared/Markdown";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "@/components/ui/sonner";
import { DOT, OPTION, optionKey } from "@/features/attempts/components/answerStyles";
import { scoreText } from "@/features/assignments/studentTime";
import { ReviewGroup } from "@/features/media";
import { AudioPlayer } from "@/features/media/components/AudioPlayer";
import type { Answer } from "@/features/take-test/api";
import { blankInputs } from "@/features/take-test/components/blankInputs";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { useDetailShell } from "@/layouts/detailShell";
import { ApiError } from "@/lib/api/errors";
import { useLocale } from "@/lib/i18n/useLocale";
import { formatTime, shortDate } from "@/lib/i18n/datetime";
import { cn } from "@/lib/utils";
import { getAttemptResult, type AttemptResult, type ResultQuestion } from "../api";
import type { TFunction } from "i18next";
import type { Locale } from "@/lib/i18n";

type Chip = "all" | "wrong" | "pending";

/** ResultPage keeps the result summary and policy-aware answers in one reading flow. */
export default function ResultPage() {
  const { t } = useTranslation();
  const { attemptId = "" } = useParams<{ attemptId: string }>();
  const shell = useDetailShell();
  const wide = useMediaQuery("(min-width: 1024px)");
  const [chip, setChip] = useState<Chip>("all");
  const target = useRef<string | null>(null);
  const [focusRequest, requestFocus] = useReducer((n: number) => n + 1, 0);
  useEffect(() => {
    if (target.current === null) return;
    const element = document.getElementById(`result-question-${target.current}`);
    element?.focus({ preventScroll: true });
    element?.scrollIntoView?.({ block: "start" });
    target.current = null;
  }, [focusRequest]);

  const result = useQuery({
    queryKey: ["attempt-result", attemptId],
    queryFn: ({ signal }) => getAttemptResult(attemptId, signal),
    retry: (count, error) => !(error instanceof ApiError) && count < 2,
  });
  const title = result.data?.testTitle ?? null;
  useEffect(() => {
    shell.setTitle(title);
    return () => shell.setTitle(null);
  }, [shell, title]);

  if (result.isPending) return <ResultSkeleton />;
  if (result.isError) {
    const error = result.error;
    if (error instanceof ApiError && error.status === 409) {
      return (
        <Card>
          <CardContent className="text-center">
            <p className="text-sm leading-relaxed">{t("result.notReady")}</p>
            <p className="text-muted-foreground mt-1 text-sm">{error.message}</p>
            <Button className="mt-4 w-full" asChild>
              <Link to="/app">{t("takeTest.backHome")}</Link>
            </Button>
          </CardContent>
        </Card>
      );
    }
    return <ResultError error={error} onRetry={() => void result.refetch()} />;
  }

  const data = result.data;
  const { attempt, review, questions } = data;
  const wrong = questions.filter((q) => verdict(q, review) === "wrong").length;
  const pending = questions.filter((q) => q.pendingManual === true).length;
  const activeChip = chip === "wrong" && !review.showScore ? "all" : chip;
  const shown = questions.filter((q) => {
    if (activeChip === "wrong") return verdict(q, review) === "wrong";
    if (activeChip === "pending") return q.pendingManual === true;
    return true;
  });

  const numbers = new Map(questions.map((question, index) => [question.id, index + 1]));
  const shownIds = new Set(shown.map((question) => question.id));
  const contexts = new Map(
    data.sharedContext?.groups.map((group) => [
      group.questionIds.find((id) => shownIds.has(id)),
      group,
    ]),
  );
  const jump = (id: string) => {
    target.current = id;
    setChip("all");
    requestFocus();
  };

  const scoreBlock = review.showScore ? (
    <ScoreTile data={data} pending={pending} />
  ) : (
    <Card>
      <CardContent className="text-center">
        <Lock className="text-muted-foreground mx-auto size-6" aria-hidden="true" />
        <p className="mt-3 text-sm font-medium">{t("result.hiddenTitle")}</p>
        <p className="text-muted-foreground mt-1.5 text-xs leading-relaxed">
          {t("result.hiddenBody", {
            time: attempt.submittedAt ? formatTime(attempt.submittedAt) : "",
            date: attempt.submittedAt ? shortDate(attempt.submittedAt) : "",
          })}
        </p>
      </CardContent>
    </Card>
  );
  const noKey = !review.showCorrectAnswers && (
    <p className="text-muted-foreground flex items-start gap-1.5 text-xs leading-relaxed">
      <Info className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
      <span>{t("result.noKey")}</span>
    </p>
  );

  return (
    <div className="mx-auto w-full max-w-[720px] space-y-5">
      <div>
        {wide && <BackLink to="/app">{t("student.myAssignments")}</BackLink>}
        <h1 className="mt-3 text-xl leading-snug font-semibold tracking-tight">
          {data.testTitle}
        </h1>
        {attempt.submittedAt && (
          <p className="text-muted-foreground mt-1 text-xs">
            {t("result.submittedMeta", {
              time: formatTime(attempt.submittedAt),
              date: shortDate(attempt.submittedAt),
              n: attempt.attemptNo,
              total: data.maxAttempts,
            })}
          </p>
        )}
      </div>

      {scoreBlock}
      {pending > 0 && review.showScore && (
        <div className="bg-muted/40 flex items-start gap-2.5 rounded-md px-3 py-2.5">
          <Pencil
            className="text-muted-foreground mt-0.5 size-4 shrink-0"
            aria-hidden="true"
          />
          <div className="text-sm">
            <p className="font-medium">
              {t("result.pendingTitle", { count: pending })}
            </p>
            <p className="text-muted-foreground">{t("result.pendingBody")}</p>
          </div>
        </div>
      )}

      {!review.showScore && (
        <Card>
          <CardContent className="space-y-2">
            <p className="text-sm font-medium">{t("result.yourPaper")}</p>
            <p className="text-muted-foreground text-sm leading-relaxed">
              {t(
                review.showCorrectAnswers
                  ? "result.answeredWithKey"
                  : "result.answeredNoKey",
                {
                  answered: questions.filter((q) => q.answer !== null).length,
                  total: questions.length,
                },
              )}
            </p>
          </CardContent>
        </Card>
      )}

      <div
        className="flex flex-wrap items-center gap-1.5"
        role="group"
        aria-label={t("result.filter")}
      >
        {(review.showScore
          ? (["all", "wrong", "pending"] as Chip[])
          : (["all", "pending"] as Chip[])
        ).map((value) => (
          <Button
            key={value}
            size="xs"
            variant={activeChip === value ? "secondary" : "ghost"}
            className={cn(activeChip !== value && "text-muted-foreground")}
            aria-pressed={activeChip === value}
            onClick={() => setChip(value)}
          >
            {t(`result.chips.${value}`, {
              count: { all: questions.length, wrong, pending }[value],
            })}
          </Button>
        ))}
      </div>

      {noKey}
      {shown.length === 0 && (
        <EmptyState
          action={
            <Button variant="outline" onClick={() => setChip("all")}>
              {t("result.showAll")}
            </Button>
          }
        >
          {t(`result.empty.${activeChip}`)}
        </EmptyState>
      )}

      {shown.map((question) => {
        const group = contexts.get(question.id);
        return (
          <div key={question.id} className="flex min-w-0 flex-col gap-5">
            {group && data.sharedContext && (
              <ReviewGroup
                group={group}
                numbers={numbers}
                transcripts={data.sharedContext.transcripts}
                plays={data.sharedContext.audioPlays}
                onQuestion={jump}
                onRetry={() => void result.refetch()}
              />
            )}
            <div
              id={`result-question-${question.id}`}
              tabIndex={-1}
              className="outline-none"
            >
              <QuestionCard
                question={question}
                number={numbers.get(question.id) ?? 0}
                review={review}
              />
            </div>
          </div>
        );
      })}

      {!review.showExplanations && shown.length > 0 && (
        <p className="text-muted-foreground text-center text-xs leading-relaxed">
          {t("result.noExplanations")}
        </p>
      )}
    </div>
  );
}

function ScoreTile({
  data,
  pending,
}: Readonly<{ data: AttemptResult; pending: number }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const { attempt } = data;
  const score = attempt.score;
  if (!score) return null;
  const percent =
    score.total === 0 ? 0 : Math.round((score.earned / score.total) * 100);
  const pendingPoints = data.questions
    .filter((q) => q.pendingManual === true)
    .reduce((sum, q) => sum + q.points, 0);
  const [earned, total] = scoreText(score.earned, score.total, locale, t).split("/");
  return (
    <Card>
      <CardContent className="text-center">
        <p className="text-muted-foreground text-xs tracking-wide uppercase">
          {t(pending > 0 ? "result.provisional" : "result.yourScore")}
        </p>
        <p className="mt-2 text-4xl font-semibold tabular-nums">
          {earned}
          <span className="text-muted-foreground text-xl">/{total}</span>
        </p>
        {pending > 0 && (
          <p className="text-muted-foreground mt-2 text-xs">
            {t("result.excludesPending", { points: pendingPoints })}
          </p>
        )}
        <span
          className="bg-secondary mt-4 block h-1.5 overflow-hidden rounded-full"
          role="img"
          aria-label={t("result.percent", { percent })}
        >
          <span
            className={cn(
              "block h-full rounded-full",
              percent >= 80 ? "bg-success" : "bg-foreground",
            )}
            style={{ width: `${percent}%` }}
          />
        </span>
        <dl className="mt-4 grid grid-cols-3 gap-3 border-t pt-4 text-sm">
          {(["correct", "wrong", "pending"] as const).map((state) => (
            <div key={state}>
              <dt className="text-muted-foreground">{t(`result.rows.${state}`)}</dt>
              <dd className="mt-1 font-medium tabular-nums">
                {state === "pending"
                  ? pending
                  : data.questions.filter((q) => verdict(q, data.review) === state)
                      .length}
              </dd>
            </div>
          ))}
        </dl>
      </CardContent>
    </Card>
  );
}

type Verdict = "correct" | "wrong" | "partial" | "pending" | "unknown";

function verdict(q: ResultQuestion, review: AttemptResult["review"]): Verdict {
  if (q.pendingManual === true) return "pending";
  if (!review.showScore || q.earned == null) return "unknown";
  if (q.earned >= q.points) return "correct";
  if (q.earned > 0) return "partial";
  return "wrong";
}

function QuestionCard({
  question,
  number,
  review,
}: Readonly<{
  question: ResultQuestion;
  number: number;
  review: AttemptResult["review"];
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const v = verdict(question, review);
  const isAudio = question.media?.kind === "audio";
  const earnedText = earnedLabel(v, question, locale, t);
  return (
    <Card>
      <CardContent className="space-y-3">
        <div className="flex items-center gap-2">
          <span
            className={cn(
              DOT.base,
              "size-6 text-xs",
              v === "correct" && DOT.correct,
              v === "wrong" && DOT.wrong,
            )}
            aria-hidden="true"
          >
            {number}
          </span>
          {earnedText !== null && (
            <span className="text-muted-foreground text-xs">{earnedText}</span>
          )}
          <span className="ml-auto">
            <VerdictBadge
              v={v}
              isAudio={isAudio}
              graded={question.graderComment != null}
            />
          </span>
        </div>

        <Body question={question} review={review} />

        {isAudio && question.media && (
          <>
            <AudioPlayer
              src={question.media.url}
              label={t("takeTest.audioLabel")}
              durationMs={question.media.durationMs}
              allowSeek
              size="sm"
              preload="metadata"
              hint={t("result.replayFreely")}
            />
            {question.transcript != null ? (
              <details className="text-sm">
                <summary className="text-muted-foreground flex cursor-pointer items-center gap-1.5">
                  <ScrollText className="size-4" aria-hidden="true" />
                  {t("result.showTranscript")}
                </summary>
                <p className="text-muted-foreground mt-2 leading-relaxed whitespace-pre-wrap">
                  {question.transcript}
                </p>
              </details>
            ) : (
              <p className="text-muted-foreground flex items-center gap-1.5 text-xs">
                <ScrollText className="size-3.5 shrink-0" aria-hidden="true" />
                <span>{t("result.noTranscript")}</span>
              </p>
            )}
          </>
        )}

        {question.explanation != null && (
          <div className="bg-muted/40 flex items-start gap-2 rounded-md px-3 py-2.5">
            <Info
              className="text-muted-foreground mt-0.5 size-4 shrink-0"
              aria-hidden="true"
            />
            <QuestionProse
              className="min-w-0 flex-1 text-xs"
              text={question.explanation}
              content={question.explanationContent}
            />
          </div>
        )}

        {question.graderComment != null && (
          <div className="border-foreground border-l-2 pl-3">
            <p className="text-muted-foreground text-xs">
              {t("result.teacherComment")}
            </p>
            <p className="mt-1 text-sm leading-relaxed whitespace-pre-wrap">
              {question.graderComment}
            </p>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function Body({
  question,
  review,
}: Readonly<{
  question: ResultQuestion;
  review: AttemptResult["review"];
}>) {
  const { t } = useTranslation();
  const given: Answer | null = question.answer;
  switch (question.type) {
    case "short_answer":
      return (
        <>
          <QuestionProse
            className="text-sm"
            text={question.prompt}
            content={question.promptContent}
          />
          <div className="bg-muted/50 rounded-md p-3">
            {given !== null && "value" in given && String(given.value).trim() !== "" ? (
              <p className="text-sm leading-relaxed whitespace-pre-wrap">
                {String(given.value)}
              </p>
            ) : (
              <p className="text-muted-foreground text-sm">{t("result.unanswered")}</p>
            )}
          </div>
        </>
      );
    case "fill_blank": {
      const values = given !== null && "values" in given ? given.values : {};
      const key = new Map(
        (question.correctAnswers ?? []).map((c) => [c.blankId, c.answer]),
      );
      return (
        <>
          {question.promptContent != null ? (
            <RichBlankPrompt
              text={question.prompt}
              content={question.promptContent}
              blanks={question.blanks ?? []}
              className="text-sm"
              renderBlank={(blank) => (
                <span className="mx-0.5 inline-block rounded-sm border px-1.5 underline decoration-dotted">
                  {values[blank.id] || "…"}
                </span>
              )}
            />
          ) : (
            <Markdown
              className="text-sm"
              plugins={[blankInputs]}
              components={{
                span: (props) => {
                  const ordinal = props.node?.properties?.["data-blank"];
                  if (ordinal === undefined || ordinal === null)
                    return <span {...props} />;
                  const blank = (question.blanks ?? []).find(
                    (b) => String(b.ordinal) === String(ordinal),
                  );
                  const typed = blank === undefined ? "" : (values[blank.id] ?? "");
                  return (
                    <span className="mx-0.5 inline-block rounded-sm border px-1.5 underline decoration-dotted">
                      {typed === "" ? "…" : typed}
                    </span>
                  );
                },
              }}
            >
              {question.prompt}
            </Markdown>
          )}
          {review.showCorrectAnswers && key.size > 0 && (
            <p className="text-muted-foreground text-xs">
              {t("result.correctAnswerIs", {
                answers: (question.blanks ?? [])
                  .map((b) => key.get(b.id))
                  .filter((a): a is string => a !== undefined)
                  .join(" · "),
              })}
            </p>
          )}
        </>
      );
    }
    default: {
      const chosen = new Set(
        given !== null && "optionIds" in given ? given.optionIds : [],
      );
      const correct = new Set(question.correctOptionIds ?? []);
      const options = question.options ?? [];
      // With the key withheld only the student's own choice is marked (S-09b).
      const rows = review.showCorrectAnswers
        ? options.filter((o) => chosen.has(o.id) || correct.has(o.id))
        : options;
      return (
        <>
          <QuestionProse
            className="text-sm"
            text={question.prompt}
            content={question.promptContent}
          />
          <div className="space-y-2">
            {rows.map((option) => {
              const picked = chosen.has(option.id);
              const right = correct.has(option.id);
              return (
                <div
                  key={option.id}
                  className={cn(
                    OPTION.base,
                    review.showCorrectAnswers && right && OPTION.correct,
                    picked &&
                      (review.showCorrectAnswers ? !right : true) &&
                      (review.showCorrectAnswers ? OPTION.wrong : OPTION.selected),
                  )}
                >
                  <span className={OPTION.key}>
                    {optionKey(options.indexOf(option))}
                  </span>
                  <span className="text-sm">
                    <OptionText text={option.text} content={option.content} />
                  </span>
                  <span className="text-muted-foreground ml-auto self-center text-xs">
                    {optionNote(picked, right, t)}
                  </span>
                </div>
              );
            })}
          </div>
        </>
      );
    }
  }
}

function ResultError({
  error,
  onRetry,
}: Readonly<{ error: unknown; onRetry: () => void }>) {
  const { t } = useTranslation();
  const requestId = error instanceof ApiError ? error.requestId : undefined;
  return (
    <div role="alert" className="flex items-start gap-2.5 rounded-lg border p-4">
      <CircleAlert
        className="text-muted-foreground mt-0.5 size-4.5 shrink-0"
        aria-hidden="true"
      />
      <div className="min-w-0 flex-1 text-sm">
        <p className="font-medium">{t("result.loadFailed")}</p>
        <p className="text-muted-foreground">{t("result.loadFailedBody")}</p>
        <div className="mt-3">
          <Button size="sm" onClick={onRetry}>
            <RefreshCw aria-hidden="true" />
            {t("common.retry")}
          </Button>
        </div>
        {requestId !== undefined && (
          <div className="mt-3 flex items-center gap-2">
            <span className="text-muted-foreground text-xs">
              {t("common.requestId")}
            </span>
            <code className="rounded-sm border px-1.5 py-0.5 font-mono text-xs">
              {requestId}
            </code>
            <Button
              variant="ghost"
              size="xs"
              onClick={() => {
                void navigator.clipboard.writeText(requestId);
                toast(t("common.copied"));
              }}
            >
              {t("common.copy")}
            </Button>
          </div>
        )}
      </div>
    </div>
  );
}

function ResultSkeleton() {
  const { t } = useTranslation();
  return (
    <div
      role="status"
      aria-live="polite"
      aria-label={t("common.loading")}
      className="space-y-4"
    >
      <Card>
        <CardContent className="text-center">
          <Skeleton className="mx-auto h-3 w-24" />
          <Skeleton className="mx-auto mt-2 h-10 w-32" />
          <Skeleton className="mt-4 h-2 w-full" />
          <Skeleton className="mx-auto mt-3 h-3 w-48" />
        </CardContent>
      </Card>
      <div className="flex items-center gap-1.5">
        <Skeleton className="h-6 w-16" />
        <Skeleton className="h-6 w-12" />
        <Skeleton className="h-6 w-20" />
      </div>
      {[0, 1].map((i) => (
        <Card key={i}>
          <CardContent className="space-y-3">
            <div className="flex items-center gap-2">
              <Skeleton className="size-6" />
              <Skeleton className="h-3 w-16" />
              <Skeleton className="ml-auto h-5 w-12" />
            </div>
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-56" />
          </CardContent>
        </Card>
      ))}
    </div>
  );
}

function earnedLabel(
  v: Verdict,
  question: ResultQuestion,
  locale: Locale,
  t: TFunction,
): string | null {
  if (v === "pending") return t("result.pointsPending", { total: question.points });
  if (v === "unknown") return null;
  return t("result.pointsOf", {
    earned: scoreText(question.earned ?? 0, question.points, locale, t).split("/")[0],
    total: question.points,
  });
}

const VERDICT_BADGE: Partial<
  Record<
    Verdict,
    { variant: "success" | "danger" | "warning" | "outline"; key: string }
  >
> = {
  correct: { variant: "success", key: "result.verdict.correct" },
  wrong: { variant: "danger", key: "result.verdict.wrong" },
  partial: { variant: "warning", key: "result.verdict.partial" },
  pending: { variant: "outline", key: "result.verdict.pending" },
};

/** S-06's verdict badge; a listening question is labelled as one instead. */
function VerdictBadge({
  v,
  isAudio,
  graded,
}: Readonly<{ v: Verdict; isAudio: boolean; graded: boolean }>) {
  const { t } = useTranslation();
  if (isAudio) {
    return (
      <Badge variant="outline">
        <Headphones aria-hidden="true" />
        {t("result.listening")}
      </Badge>
    );
  }
  const known = VERDICT_BADGE[v];
  if (known) return <Badge variant={known.variant}>{t(known.key)}</Badge>;
  return graded ? <Badge variant="success">{t("result.verdict.graded")}</Badge> : null;
}

function optionNote(picked: boolean, right: boolean, t: TFunction): string | null {
  if (picked) return t("result.youChose");
  return right ? t("result.correctOption") : null;
}
