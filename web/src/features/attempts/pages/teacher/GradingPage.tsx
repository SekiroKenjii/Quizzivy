import { useEffect, useEffectEvent, useRef, type MouseEvent } from "react";
import { useTranslation } from "react-i18next";
import { Link, useLocation, useNavigate } from "react-router";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { ListSkeleton, LoadError, EmptyState } from "@/components/shared/ListState";
import { useCan, useWorkspace } from "@/features/auth/permissions";
import { useAuthStore } from "@/stores/auth";
import { useLocale } from "@/lib/i18n/useLocale";
import { formatDateTime, useDisplayTimeZone } from "@/lib/i18n/datetime";
import type { GradingQueueParams } from "../../api";
import { gradingGroupKey, gradingItemKey, scoreOptions } from "./gradingRecovery";
import { useGradingQueue } from "./useGradingQueue";
import { GradingQueueAside } from "./GradingQueueAside";
import { GradingAnswerCard } from "./GradingAnswerCard";
import "@/features/attempts/gradingMessages";
import { cn } from "@/lib/utils";
import { X } from "lucide-react";

const HEADING_FOCUS = "grading-heading";

/** GradingPage presents pending work and explicit Finish recovery without changing independent regrading. */
export default function GradingPage() {
  const { t } = useTranslation();
  const location = useLocation();
  const generation = useAuthStore((state) => state.actorGeneration);
  const grading = useCan("teaching.grading");
  const workspace = useWorkspace("teacher");
  const allowed = grading && workspace;
  const search = new URLSearchParams(location.search);
  const mode = search.get("mode") === "question" ? "question" : "student";
  const assignment = search.get("assignment") ?? "";
  const student = search.get("student") ?? "";
  if (!allowed)
    return (
      <Alert variant="danger">
        <AlertDescription>{t("grading.unavailable")}</AlertDescription>
      </Alert>
    );
  return (
    <QueuePage
      key={JSON.stringify([generation, mode, assignment, student])}
      params={{
        mode,
        ...(assignment ? { assignmentId: assignment } : {}),
        ...(student ? { studentId: student } : {}),
      }}
    />
  );
}

function QueuePage({ params }: Readonly<{ params: GradingQueueParams }>) {
  const { t } = useTranslation();
  const location = useLocation();
  const navigate = useNavigate();
  const model = useGradingQueue(params);
  const mode = params.mode ?? "student";
  const selected = model.selected;
  const groupItems = selected
    ? model.items.filter(
        (item) => gradingGroupKey(item, mode) === gradingGroupKey(selected, mode),
      )
    : [];
  const position = selected
    ? groupItems.findIndex(
        (item) => gradingItemKey(item) === gradingItemKey(selected),
      ) + 1
    : 0;
  const acknowledged = model.savedCount;
  const heading = useRef<HTMLHeadingElement>(null);
  const focusHeading =
    (location.state as { focus?: string } | null)?.focus === HEADING_FOCUS;
  useEffect(() => {
    if (focusHeading) heading.current?.focus();
  }, [focusHeading]);
  const change = (key: string, value: string, focus?: typeof HEADING_FOCUS) => {
    if (model.busy) return;
    const next = new URLSearchParams(location.search);
    if (value) next.set(key, value);
    else next.delete(key);
    void model.leave(() => {
      void navigate(
        {
          pathname: location.pathname,
          search: next.toString(),
          hash: location.hash,
        },
        focus ? { state: { focus } } : undefined,
      );
    });
  };
  const openReview = (event: MouseEvent<HTMLAnchorElement>, to: string) => {
    if (
      event.button !== 0 ||
      event.ctrlKey ||
      event.metaKey ||
      event.shiftKey ||
      event.altKey
    )
      return;
    event.preventDefault();
    void model.leave(() => {
      void navigate(to);
    });
  };
  const shortcuts = useEffectEvent((event: KeyboardEvent) => {
    if (
      model.busy ||
      model.candidate ||
      !selected ||
      event.isComposing ||
      event.altKey ||
      event.ctrlKey ||
      event.metaKey ||
      event.shiftKey
    )
      return;
    if (
      event.target instanceof Element &&
      event.target.closest(
        "input, textarea, select, [contenteditable]:not([contenteditable=false]), [role=textbox], [role=dialog], [role=menu], [role=listbox]",
      )
    )
      return;
    if (event.key === "j" || event.key === "k") {
      event.preventDefault();
      model.move(event.key === "j" ? 1 : -1);
      return;
    }
    const choices = scoreOptions(selected.points);
    const index = /^[1-9]$/.test(event.key) ? Number(event.key) - 1 : -1;
    const points = choices[index];
    if (points !== undefined) {
      event.preventDefault();
      void model.pickScore(selected, points);
    }
  });
  useEffect(() => {
    const handle = (event: KeyboardEvent) => shortcuts(event);
    document.addEventListener("keydown", handle);
    return () => document.removeEventListener("keydown", handle);
  }, []);
  return (
    <section
      data-scale="deck"
      aria-label={t("grading.title")}
      className="flex min-w-0 flex-col gap-4"
    >
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1
            ref={heading}
            tabIndex={-1}
            className="text-2xl leading-[1.5] font-semibold tracking-[-0.02em] outline-none"
          >
            {t("grading.title")}
          </h1>
          {(model.queue.data || params.assignmentId || params.studentId) && (
            <p className="text-muted-foreground mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-[14px]">
              {model.queue.data &&
                t("grading.remaining", {
                  answers: model.queue.data.answersRemaining,
                  students: model.queue.data.studentsWaiting,
                })}
              <FilterChips model={model} params={params} onChange={change} />
            </p>
          )}
        </div>
        <div
          role="group"
          aria-label={t("grading.mode")}
          className="bg-muted flex gap-0.5 rounded-[9px] p-0.75"
        >
          {(["student", "question"] as const).map((value) => (
            <Button
              key={value}
              size="sm"
              variant="ghost"
              aria-pressed={mode === value}
              disabled={model.busy}
              className={cn(
                "h-7.5 rounded-[7px] px-3",
                mode === value
                  ? "bg-card! text-foreground shadow-[var(--qz-shadow),0_0_0_1px_var(--border)]"
                  : "text-muted-foreground",
              )}
              onClick={() => change("mode", value)}
            >
              {t(value === "student" ? "grading.byStudent" : "grading.byQuestion")}
            </Button>
          ))}
        </div>
      </header>
      {model.error && (
        <Alert variant="danger">
          <AlertDescription>{model.error}</AlertDescription>
        </Alert>
      )}
      {Object.values(model.finishes).map((state) => (
        <Alert key={state.item.attemptId} variant="danger">
          <AlertDescription>
            <p>{t("grading.finishFailed")}</p>
            <p>{state.error}</p>
            <div className="mt-2 flex gap-2">
              <Button
                disabled={model.busy}
                onClick={() => void model.retryFinish(state)}
              >
                {t("grading.retryFinish")}
              </Button>
              <Button asChild variant="outline">
                <Link
                  to={`/teacher/attempts/${state.item.attemptId}`}
                  aria-disabled={model.busy}
                  onClick={(event) =>
                    openReview(event, `/teacher/attempts/${state.item.attemptId}`)
                  }
                >
                  {t("grading.openReview")}
                </Link>
              </Button>
            </div>
          </AlertDescription>
        </Alert>
      ))}
      {model.queue.isPending && <ListSkeleton />}
      {model.queue.isError && (
        <LoadError error={model.queue.error} onRetry={() => void model.queue.refetch()}>
          {t("grading.loadFailed")}
        </LoadError>
      )}
      {model.queue.data && (
        <>
          {model.queue.data.answersRemaining > model.queue.data.items.length && (
            <p role="status" className="text-muted-foreground text-xs">
              {t("grading.prefix", {
                shown: model.queue.data.items.length,
                total: model.queue.data.answersRemaining,
              })}
            </p>
          )}
          {selected && !model.candidate ? (
            <div className="flex min-w-0 flex-wrap items-start gap-3.5">
              <GradingQueueAside
                queue={model.queue.data}
                items={model.items}
                selected={selected}
                mode={mode}
                busy={model.busy}
                onSelect={(item) => void model.select(item)}
              />
              <div className="w-full min-w-0 flex-[3_1_520px]">
                <div
                  role="progressbar"
                  aria-label={t("grading.windowProgress")}
                  aria-valuemin={0}
                  aria-valuemax={model.items.length}
                  aria-valuenow={acknowledged}
                  className="bg-muted h-0.75 overflow-hidden rounded-t-xl"
                >
                  <div
                    className="bg-brand h-full transition-[width] duration-200 [transition-timing-function:ease] motion-reduce:transition-none"
                    style={{
                      width: `${model.items.length ? (acknowledged / model.items.length) * 100 : 0}%`,
                    }}
                  />
                </div>
                <GradingAnswerCard
                  key={gradingItemKey(selected)}
                  item={selected}
                  draft={model.draft}
                  busy={model.busy}
                  finishReady={model.finishReady?.attemptId === selected.attemptId}
                  position={position}
                  total={groupItems.length}
                  onScore={(points) => void model.pickScore(selected, points)}
                  onComment={(comment) => model.comment(selected, comment)}
                  onPrevious={() => model.move(-1)}
                  onNext={(numericIntent = false) =>
                    void (model.finishReady?.attemptId === selected.attemptId
                      ? model.confirmFinish()
                      : model.next(numericIntent))
                  }
                  onRetryMaterial={() => void model.queue.refetch()}
                  onReview={(event) =>
                    openReview(event, `/teacher/attempts/${selected.attemptId}`)
                  }
                />
              </div>
            </div>
          ) : (
            !model.candidate && (
              <EmptyState hint={t("grading.emptyHint")}>
                {t("grading.empty")}
              </EmptyState>
            )
          )}
        </>
      )}
      <Recovery model={model} onReview={openReview} />
    </section>
  );
}

function FilterChips({
  model,
  params,
  onChange,
}: Readonly<{
  model: ReturnType<typeof useGradingQueue>;
  params: GradingQueueParams;
  onChange: (key: string, value: string, focus?: typeof HEADING_FOCUS) => void;
}>) {
  const { t } = useTranslation();
  const assignmentName = () =>
    model.items.find((item) => item.assignmentId === params.assignmentId)
      ?.assignmentTitle ??
    model.recovery.data?.find((row) => row.assignmentId === params.assignmentId)
      ?.testTitle ??
    t("grading.appliedAssignment");
  const studentName = () =>
    model.items.find((item) => item.studentId === params.studentId)?.studentName ??
    model.recovery.data?.find((row) => row.studentId === params.studentId)
      ?.studentName ??
    t("grading.appliedStudent");
  const chips = [
    ...(params.assignmentId ? [{ key: "assignment", name: assignmentName() }] : []),
    ...(params.studentId ? [{ key: "student", name: studentName() }] : []),
  ];
  return chips.map((chip) => (
    <span
      key={chip.key}
      className="bg-muted text-fg inline-flex h-6 max-w-full items-center gap-1 rounded-full pr-0.5 pl-2.5 text-xs font-medium"
    >
      <span className="truncate">{chip.name}</span>
      <button
        type="button"
        disabled={model.busy}
        aria-label={t("grading.removeFilter", { name: chip.name })}
        className="text-muted-fg hover:bg-hover hover:text-fg grid size-5 shrink-0 place-items-center rounded-full disabled:opacity-50"
        onClick={() => onChange(chip.key, "", HEADING_FOCUS)}
      >
        <X aria-hidden="true" className="size-3" />
      </button>
    </span>
  ));
}

function Recovery({
  model,
  onReview,
}: Readonly<{
  model: ReturnType<typeof useGradingQueue>;
  onReview: (event: MouseEvent<HTMLAnchorElement>, to: string) => void;
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const zone = useDisplayTimeZone();
  const review = model.candidateReview.data;
  const candidate = model.candidate;
  const matches =
    candidate &&
    review?.attempt.id === candidate.id &&
    review.attempt.assignmentId === candidate.assignmentId &&
    review.attempt.studentId === candidate.studentId;
  return (
    <section
      className="space-y-3 rounded-xl border p-4"
      aria-label={t("grading.recoveryTitle")}
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 className="text-sm font-semibold">{t("grading.recoveryTitle")}</h2>
        <Button
          variant="outline"
          size="sm"
          disabled={model.busy || model.recovery.isFetching}
          onClick={() => {
            void model.recovery.refetch();
            void model.queue.refetch();
          }}
        >
          {t("grading.rescan")}
        </Button>
      </div>
      <p className="text-muted-foreground text-xs">{t("grading.recoveryHint")}</p>
      {model.recovery.isPending && <ListSkeleton rows={2} />}
      {model.recovery.isError && (
        <LoadError
          error={model.recovery.error}
          onRetry={() => void model.recovery.refetch()}
        >
          {t("grading.scanFailed")}
        </LoadError>
      )}
      {model.recovery.data && (
        <div className="max-h-45 space-y-1 overflow-auto">
          {model.recovery.data
            .filter((row) => !model.completed.has(row.id))
            .map((row) => (
              <Button
                key={row.id}
                aria-label={t("grading.candidate", {
                  name: row.studentName,
                  title: row.testTitle,
                })}
                variant="ghost"
                className="h-auto w-full flex-wrap justify-start gap-2 text-left"
                disabled={model.busy}
                onClick={() => void model.selectCandidate(row)}
              >
                <span>{row.studentName}</span>
                <span className="text-muted-foreground">{row.testTitle}</span>
                {row.submittedAt && (
                  <span className="text-muted-foreground text-xs">
                    {formatDateTime(row.submittedAt, locale, zone)}
                  </span>
                )}
              </Button>
            ))}
        </div>
      )}
      {candidate && (
        <div className="space-y-2 rounded-lg border p-3">
          {model.candidateReview.isPending && <ListSkeleton rows={2} />}
          {model.candidateReview.isError && (
            <LoadError
              error={model.candidateReview.error}
              onRetry={() => void model.candidateReview.refetch()}
            >
              {t("grading.reviewFailed")}
            </LoadError>
          )}
          {matches && (
            <>
              <p className="text-sm">
                {review.student.fullName} · {review.testTitle}
              </p>
              <p className="text-muted-foreground text-xs">
                {review.attempt.score?.pendingManual === 0
                  ? t("grading.ready")
                  : t("grading.pendingUnknown")}
              </p>
              <div className="flex flex-wrap gap-2">
                <Button
                  disabled={
                    model.busy ||
                    review.attempt.score?.pendingManual !== 0 ||
                    !["submitted", "timed_out"].includes(review.attempt.status)
                  }
                  onClick={() => void model.finishCandidate()}
                >
                  {t("review.finish")}
                </Button>
                <Button asChild variant="outline">
                  <Link
                    to={`/teacher/attempts/${candidate.id}`}
                    aria-disabled={model.busy}
                    onClick={(event) =>
                      onReview(event, `/teacher/attempts/${candidate.id}`)
                    }
                  >
                    {t("grading.openReview")}
                  </Link>
                </Button>
              </div>
            </>
          )}
        </div>
      )}
    </section>
  );
}
