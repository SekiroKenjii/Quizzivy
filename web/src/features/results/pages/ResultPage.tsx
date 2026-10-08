import { useEffect, useId, useReducer, useRef, useState } from "react";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { Link, useParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { ArrowLeft, Lock } from "lucide-react";
import { modules } from "@/app/modules";
import { Button } from "@/components/ui/button";
import { Segmented } from "@/components/ui/segmented";
import { listMyAssignments } from "@/features/assignments/api";
import { ReviewGroup } from "@/features/media";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { useTick } from "@/hooks/useTick";
import { ApiError } from "@/lib/api/errors";
import type { Locale } from "@/lib/i18n";
import { clockTime, dayDate, useDisplayTimeZone } from "@/lib/i18n/datetime";
import { cn } from "@/lib/utils";
import { getAttemptResult, type AttemptResult } from "../api";
import { ResultFailure, ResultSkeleton } from "../components/ResultStates";
import { ResultTiles } from "../components/ResultTiles";
import { ReviewItem } from "../components/ReviewItem";
import { ScoreRing } from "../components/ScoreRing";
import {
  classNameOf,
  justNow,
  resultView,
  shownUnder,
  type Filter,
  type Summary,
} from "../resultView";

const COLUMN = "mx-auto flex w-full max-w-205 flex-col gap-4.5";

const PAPER = [
  "[&_img]:bg-paper",
  "[&_.content-table-scroll]:bg-paper [&_.content-table-scroll]:text-paper-fg [&_.content-table-scroll]:rounded-md",
  "[&_.content-table-scroll_:is(th,td)]:border-[color-mix(in_oklab,var(--paper-fg)_14%,var(--paper))]!",
  "[&_.content-table-scroll_th]:bg-[color-mix(in_oklab,var(--paper-fg)_5%,var(--paper))]!",
].join(" ");

const NOTE =
  "text-muted-fg text-ui flex items-center gap-2.5 rounded-xl border border-dashed px-3.5 py-3 leading-normal";

function sentence(summary: Summary, t: TFunction): string {
  switch (summary.kind) {
    case "graded":
      return t(summary.key ? "result.summary.gradedKey" : "result.summary.graded", {
        correct: summary.correct,
        total: summary.total,
      });
    case "partly":
      return [
        t("result.summary.auto", { count: summary.marked }),
        t("result.summary.teacher", { count: summary.waiting }),
      ].join(" ");
    case "pending":
      return t(
        modules.notifications
          ? "result.summary.pendingNotify"
          : "result.summary.pending",
      );
    case "withheld":
      return t(
        summary.until === null
          ? "result.summary.withheld"
          : "result.summary.withheldUntil",
        { answered: summary.answered, total: summary.total },
      );
  }
}

function submittedLine(
  submittedAt: string,
  className: string | null,
  fresh: boolean,
  locale: Locale,
  t: TFunction,
): string {
  const when = dayDate(submittedAt, locale);
  if (className === null)
    return fresh ? t("result.meta.submittedNow") : t("result.meta.submitted", { when });
  return fresh
    ? t("result.meta.submittedNowIn", { class: className })
    : t("result.meta.submittedIn", { class: className, when });
}

function metaLine(
  data: AttemptResult,
  className: string | null,
  fresh: boolean,
  locale: Locale,
  t: TFunction,
): string | null {
  const { attempt } = data;
  const parts = [
    attempt.submittedAt == null
      ? className
      : submittedLine(attempt.submittedAt, className, fresh, locale, t),
    data.maxAttempts > 1
      ? t("result.meta.attempt", { n: attempt.attemptNo, total: data.maxAttempts })
      : null,
  ].filter((part) => part !== null);
  return parts.length > 0 ? parts.join(" · ") : null;
}

/**
 * ResultPage is the student's result for one attempt, as the design deck
 * draws it: the summary card with the score ring, the paper's class and
 * title and one sentence about where grading stands; the tiles; then every
 * answer under a filter, with a line naming what the review policy hides.
 * The class comes from the assignment lists the shell already holds, which
 * the page reads again only when nothing holds them. Shared
 * material stays above the first of its questions that the filter keeps, and
 * a gap in it leads to its question even when the filter hides that one.
 * Images and tables in the material and the prompts keep the light paper
 * surface in dark mode. From 768px the page draws its own way back; below
 * that the shell's header has it.
 */
export default function ResultPage() {
  useDisplayTimeZone();
  const { t } = useTranslation();
  const { attemptId = "" } = useParams<{ attemptId: string }>();
  const wide = useMediaQuery("(min-width: 768px)");
  const result = useQuery({
    queryKey: ["attempt-result", attemptId],
    queryFn: ({ signal }) => getAttemptResult(attemptId, signal),
    retry: (count, error) => !(error instanceof ApiError) && count < 2,
  });
  const retry = () => void result.refetch();

  return (
    <div className={COLUMN}>
      {wide && (
        <Link
          to="/app"
          className="text-muted-fg hover:text-fg text-ui inline-flex items-center gap-1.5 self-start leading-4.5"
        >
          <ArrowLeft aria-hidden="true" className="size-[15px]" />
          {t("student.shell.home")}
        </Link>
      )}
      {result.data === undefined && result.isPending && <ResultSkeleton />}
      {result.data === undefined && !result.isPending && (
        <ResultFailure error={result.error} onRetry={retry} />
      )}
      {result.data !== undefined && (
        <Loaded data={result.data} wide={wide} onRetry={retry} />
      )}
    </div>
  );
}

function SubmittedMeta({
  data,
  classTitle,
}: Readonly<{ data: AttemptResult; classTitle: string | null }>) {
  const { t, i18n } = useTranslation();
  const fresh = justNow(data.attempt.submittedAt, new Date());
  useTick(fresh);
  const meta = metaLine(data, classTitle, fresh, i18n.language as Locale, t);
  if (meta === null) return null;
  return <p className="text-muted-fg text-sm break-words">{meta}</p>;
}

function Loaded({
  data,
  wide,
  onRetry,
}: Readonly<{ data: AttemptResult; wide: boolean; onRetry: () => void }>) {
  const { t, i18n } = useTranslation();
  const heading = useId();
  const [chip, setChip] = useState<Filter>("all");
  const target = useRef<string | null>(null);
  const [focusRequest, requestFocus] = useReducer((n: number) => n + 1, 0);
  useEffect(() => {
    if (target.current === null) return;
    const element = document.getElementById(`result-question-${target.current}`);
    element?.focus({ preventScroll: true });
    element?.scrollIntoView?.({ block: "start" });
    target.current = null;
  }, [focusRequest]);
  const lists = useQuery({
    queryKey: ["my-assignments"],
    queryFn: ({ signal }) => listMyAssignments(signal),
    staleTime: Infinity,
  });

  const { attempt, review, questions } = data;
  const view = resultView(data);
  const active = view.filters.includes(chip) ? chip : "all";
  const shown = shownUnder(active, questions, review);

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

  return (
    <>
      <section className="bg-card shadow-card flex flex-wrap items-center gap-5.5 rounded-2xl border p-5.5">
        <ScoreRing ring={view.ring} />
        <div className="flex min-w-0 flex-[1_1_260px] flex-col gap-1.5">
          <SubmittedMeta
            data={data}
            classTitle={classNameOf(lists.data, attempt.assignmentId)}
          />
          <h1 className="text-xl leading-[1.3] font-semibold tracking-[-0.01em] break-words">
            {data.testTitle}
          </h1>
          <p className="text-muted-fg text-base text-pretty">
            {sentence(view.summary, t)}
          </p>
          {view.classAverage !== null && (
            <p data-slot="class-average" className="text-muted-fg text-sm text-pretty">
              {t("result.classAverage", {
                percent: new Intl.NumberFormat(i18n.language as Locale, {
                  maximumFractionDigits: 1,
                }).format(view.classAverage),
              })}
            </p>
          )}
        </div>
      </section>

      <ResultTiles tiles={view.tiles} wide={wide} />

      <section
        aria-labelledby={heading}
        data-slot="result-answers"
        className={cn("flex flex-col gap-2.5", PAPER)}
      >
        <div className="flex flex-wrap items-center justify-between gap-2.5">
          <h2 id={heading} className="text-title leading-normal font-semibold">
            {t("result.yourAnswers")}
          </h2>
          {view.filters.length > 1 && (
            <Segmented
              label={t("result.filter")}
              value={active}
              options={view.filters.map((value) => ({
                value,
                label: t(`result.filters.${value}`),
              }))}
              onChange={(value) => setChip(value as Filter)}
              className="[&_button]:min-h-0 [&_button]:min-w-0"
            />
          )}
        </div>
        {view.lock !== null && (
          <p className={NOTE}>
            <Lock aria-hidden="true" className="size-4 flex-none" />
            {t(
              `result.lock.${view.lock}`,
              view.releasesAt === null ? {} : { time: clockTime(view.releasesAt) },
            )}
          </p>
        )}
        {shown.length === 0 && questions.length > 0 && (
          <div className={cn(NOTE, "flex-wrap justify-between")}>
            <p>{t("result.empty.wrong")}</p>
            <Button variant="outline" size="sm" onClick={() => setChip("all")}>
              {t("result.showAll")}
            </Button>
          </div>
        )}
        {shown.map((question) => {
          const group = contexts.get(question.id);
          return (
            <div key={question.id} className="flex min-w-0 flex-col gap-2.5">
              {group && data.sharedContext && (
                <ReviewGroup
                  group={group}
                  numbers={numbers}
                  transcripts={data.sharedContext.transcripts}
                  plays={data.sharedContext.audioPlays}
                  onQuestion={jump}
                  onRetry={onRetry}
                />
              )}
              <div
                id={`result-question-${question.id}`}
                tabIndex={-1}
                className="outline-none"
              >
                <ReviewItem
                  question={question}
                  number={numbers.get(question.id) ?? 0}
                  review={review}
                  onRetry={onRetry}
                />
              </div>
            </div>
          );
        })}
      </section>
    </>
  );
}
