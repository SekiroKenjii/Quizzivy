import { useEffect, useEffectEvent, useRef, type MouseEvent } from "react";
import { useTranslation } from "react-i18next";
import { Link, useLocation, useNavigate } from "react-router";
import { Button } from "@/components/ui/button";
import { Alert } from "@/components/ui/alert";
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
  if (!allowed) return <Alert variant="danger">{t("grading.unavailable")}</Alert>;
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
  const change = (key: string, value: string) => {
    if (model.busy) return;
    const next = new URLSearchParams(location.search);
    if (value) next.set(key, value);
    else next.delete(key);
    void model.leave(() => {
      void navigate({
        pathname: location.pathname,
        search: next.toString(),
        hash: location.hash,
      });
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
  const root = useRef<HTMLElement>(null);
  const shortcuts = useEffectEvent((event: KeyboardEvent) => {
    if (!(event.target instanceof Node) || !root.current?.contains(event.target))
      return;
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
        "input, textarea, select, [contenteditable]:not([contenteditable=false]), [role=textbox]",
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
      ref={root}
      data-scale="deck"
      aria-label={t("grading.title")}
      className="flex min-w-0 flex-col gap-4"
    >
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">
            {t("grading.title")}
          </h1>
          {model.queue.data && (
            <p className="text-muted-foreground mt-0.5 text-sm">
              {t("grading.remaining", {
                answers: model.queue.data.answersRemaining,
                students: model.queue.data.studentsWaiting,
              })}
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
              variant={mode === value ? "secondary" : "ghost"}
              aria-pressed={mode === value}
              disabled={model.busy}
              className="h-7.5 rounded-[7px] px-3"
              onClick={() => change("mode", value)}
            >
              {t(value === "student" ? "grading.byStudent" : "grading.byQuestion")}
            </Button>
          ))}
        </div>
      </header>
      <Filters model={model} params={params} onChange={change} />
      {model.error && <Alert variant="danger">{model.error}</Alert>}
      {Object.values(model.finishes).map((state) => (
        <Alert key={state.item.attemptId} variant="danger">
          <p>{t("grading.finishFailed")}</p>
          <p>{state.error}</p>
          <div className="mt-2 flex gap-2">
            <Button disabled={model.busy} onClick={() => void model.retryFinish(state)}>
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
                  onNext={() =>
                    void (model.finishReady?.attemptId === selected.attemptId
                      ? model.confirmFinish()
                      : model.next())
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

function Filters({
  model,
  params,
  onChange,
}: Readonly<{
  model: ReturnType<typeof useGradingQueue>;
  params: GradingQueueParams;
  onChange: (key: string, value: string) => void;
}>) {
  const { t } = useTranslation();
  const assignments = new Map(
    model.items.map((item) => [item.assignmentId, item.assignmentTitle]),
  );
  const students = new Map(
    model.items.map((item) => [item.studentId, item.studentName]),
  );
  for (const row of model.recovery.data ?? []) {
    assignments.set(row.assignmentId, row.testTitle);
    students.set(row.studentId, row.studentName);
  }
  if (params.assignmentId && !assignments.has(params.assignmentId))
    assignments.set(params.assignmentId, t("grading.appliedAssignment"));
  if (params.studentId && !students.has(params.studentId))
    students.set(params.studentId, t("grading.appliedStudent"));
  return (
    <div className="flex flex-wrap gap-3">
      {[
        {
          key: "assignment",
          value: params.assignmentId ?? "",
          options: assignments,
          label: "grading.assignment",
        },
        {
          key: "student",
          value: params.studentId ?? "",
          options: students,
          label: "grading.student",
        },
      ].map((filter) => (
        <label key={filter.key} className="flex flex-col gap-1 text-xs">
          <span>{t(filter.label)}</span>
          <select
            className="bg-card h-9 max-w-full rounded-lg border px-2 text-sm"
            disabled={model.busy}
            value={filter.value}
            onChange={(event) => onChange(filter.key, event.target.value)}
          >
            <option value="">{t("grading.all")}</option>
            {[...filter.options].map(([id, label]) => (
              <option key={id} value={id}>
                {label}
              </option>
            ))}
          </select>
        </label>
      ))}
    </div>
  );
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
