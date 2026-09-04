import { useEffect, useState } from "react";
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
import { Markdown } from "@/components/shared/Markdown";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "@/components/ui/sonner";
import { DOT, OPTION, optionKey } from "@/features/attempts/components/answerStyles";
import { scoreText } from "@/features/assignments/studentTime";
import { AudioPlayer } from "@/features/media/components/AudioPlayer";
import type { Answer } from "@/features/take-test/api";
import { blankInputs } from "@/features/take-test/components/blankInputs";
import { useDetailShell } from "@/layouts/detailShell";
import { ApiError } from "@/lib/api/errors";
import { useLocale } from "@/lib/i18n/useLocale";
import { formatTime, shortDate } from "@/lib/i18n/datetime";
import { cn } from "@/lib/utils";
import { getAttemptResult, type AttemptResult, type ResultQuestion } from "../api";

type Chip = "all" | "wrong" | "pending";

/**
 * S-09: the score if allowed, then every question with exactly what the
 * policy released. A withheld block is a muted sentence, never a gap.
 */
export default function ResultPage() {
  const { t } = useTranslation();
  const { attemptId = "" } = useParams<{ attemptId: string }>();
  const shell = useDetailShell();
  const [chip, setChip] = useState<Chip>("all");

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
  const shown = questions.filter((q) => {
    if (chip === "wrong") return verdict(q, review) === "wrong";
    if (chip === "pending") return q.pendingManual === true;
    return true;
  });
  const hasAudio = questions.some((q) => q.media?.kind === "audio");

  return (
    <div className="space-y-4">
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

      {review.showScore ? (
        <ScoreTile data={data} pending={pending} />
      ) : (
        <>
          <Card>
            <CardContent className="text-center">
              <Lock
                className="text-muted-foreground mx-auto size-6"
                aria-hidden="true"
              />
              <p className="mt-3 text-sm font-medium">{t("result.hiddenTitle")}</p>
              <p className="text-muted-foreground mt-1.5 text-xs leading-relaxed">
                {t("result.hiddenBody", {
                  time: attempt.submittedAt ? formatTime(attempt.submittedAt) : "",
                  date: attempt.submittedAt ? shortDate(attempt.submittedAt) : "",
                })}
              </p>
            </CardContent>
          </Card>
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
        </>
      )}

      <div
        className="flex items-center gap-1.5"
        role="group"
        aria-label={t("result.filter")}
      >
        {(["all", "wrong", "pending"] as Chip[]).map((value) => (
          <Button
            key={value}
            size="xs"
            variant={chip === value ? "secondary" : "ghost"}
            className={cn(chip !== value && "text-muted-foreground")}
            aria-pressed={chip === value}
            onClick={() => setChip(value)}
          >
            {t(`result.chips.${value}`, {
              count:
                value === "all"
                  ? questions.length
                  : value === "wrong"
                    ? wrong
                    : pending,
            })}
          </Button>
        ))}
      </div>

      {!review.showCorrectAnswers && (
        <p className="text-muted-foreground flex items-start gap-1.5 text-xs leading-relaxed">
          <Info className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
          <span>{t("result.noKey")}</span>
        </p>
      )}

      {shown.map((question) => (
        <QuestionCard
          key={question.id}
          question={question}
          number={questions.indexOf(question) + 1}
          review={review}
        />
      ))}

      {!review.showExplanations && hasAny(questions) && (
        <p className="text-muted-foreground text-center text-xs leading-relaxed">
          {t("result.noExplanations")}
        </p>
      )}
      {hasAudio && null}
    </div>
  );
}

function hasAny(questions: ResultQuestion[]): boolean {
  return questions.length > 0;
}

function ScoreTile({ data, pending }: { data: AttemptResult; pending: number }) {
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
        {attempt.submittedAt && (
          <p className="text-muted-foreground mt-3 text-xs">
            {t("result.submittedMeta", {
              time: formatTime(attempt.submittedAt),
              date: shortDate(attempt.submittedAt),
              n: attempt.attemptNo,
              total: data.maxAttempts,
            })}
          </p>
        )}
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
}: {
  question: ResultQuestion;
  number: number;
  review: AttemptResult["review"];
}) {
  const { t } = useTranslation();
  const locale = useLocale();
  const v = verdict(question, review);
  const isAudio = question.media?.kind === "audio";
  const earnedText =
    v === "pending"
      ? t("result.pointsPending", { total: question.points })
      : v === "unknown"
        ? null
        : t("result.pointsOf", {
            earned: scoreText(question.earned ?? 0, question.points, locale, t).split(
              "/",
            )[0],
            total: question.points,
          });
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
            {isAudio ? (
              <Badge variant="outline">
                <Headphones aria-hidden="true" />
                {t("result.listening")}
              </Badge>
            ) : v === "correct" ? (
              <Badge variant="success">{t("result.verdict.correct")}</Badge>
            ) : v === "wrong" ? (
              <Badge variant="danger">{t("result.verdict.wrong")}</Badge>
            ) : v === "partial" ? (
              <Badge variant="warning">{t("result.verdict.partial")}</Badge>
            ) : v === "pending" ? (
              <Badge variant="outline">{t("result.verdict.pending")}</Badge>
            ) : question.graderComment != null ? (
              <Badge variant="success">{t("result.verdict.graded")}</Badge>
            ) : null}
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
            <Markdown className="text-xs">{question.explanation}</Markdown>
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
}: {
  question: ResultQuestion;
  review: AttemptResult["review"];
}) {
  const { t } = useTranslation();
  const given: Answer | null = question.answer;
  switch (question.type) {
    case "short_answer":
      return (
        <>
          <Markdown className="text-sm">{question.prompt}</Markdown>
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
          <Markdown className="text-sm">{question.prompt}</Markdown>
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
                  <span className="text-sm">{option.text}</span>
                  <span className="text-muted-foreground ml-auto self-center text-xs">
                    {picked
                      ? t("result.youChose")
                      : right
                        ? t("result.correctOption")
                        : null}
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

function ResultError({ error, onRetry }: { error: unknown; onRetry: () => void }) {
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
