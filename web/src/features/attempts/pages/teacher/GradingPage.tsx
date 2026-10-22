import {
  useEffect,
  useEffectEvent,
  useRef,
  useState,
  type MouseEvent,
  type Ref,
} from "react";
import { useTranslation } from "react-i18next";
import { Link, useBlocker, useLocation, useNavigate } from "react-router";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { ListSkeleton, LoadError, EmptyState } from "@/components/shared/ListState";
import { useCan, useWorkspace } from "@/features/auth/permissions";
import { useAuthStore } from "@/stores/auth";
import type { AttemptListRow, GradingQueueParams } from "../../api";
import {
  gradingGroupKey,
  gradingItemKey,
  pageKey,
  scoreOptions,
} from "./gradingRecovery";
import { useGradingQueue, type GradingQueueModel } from "./useGradingQueue";
import { GradingQueueAside } from "./GradingQueueAside";
import { GradingAnswerCard } from "./GradingAnswerCard";
import { GradingReadyCard, ROWS_SHOWN, type ReadyScan } from "./GradingReadyCard";
import { AiScoreBox } from "../../components/AiScoreBox";
import "@/features/attempts/gradingMessages";
import { cn } from "@/lib/utils";
import {
  ArrowDown,
  CircleAlert,
  CircleCheck,
  ClipboardCheck,
  Hourglass,
  TriangleAlert,
  X,
} from "lucide-react";

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
  const panes = Boolean(model.queue.data && selected);
  const heading = useRef<HTMLHeadingElement>(null);
  const cardHeading = useRef<HTMLHeadingElement>(null);
  const emptyTitle = useRef<HTMLHeadingElement>(null);
  const [cardOpen, setCardOpen] = useState(false);
  const [expanded, setExpanded] = useState<ReadonlySet<string>>(new Set());
  const [finishing, setFinishing] = useState<string | null>(null);
  const [focusFinish, setFocusFinish] = useState<string | null>(null);
  const [focusTick, setFocusTick] = useState(0);
  const focusEmpty = useRef(false);
  useEffect(() => {
    if (!focusEmpty.current) return;
    focusEmpty.current = false;
    (emptyTitle.current ?? heading.current)?.focus();
  }, [focusTick]);
  const scan = readyScan(model.recovery);
  const readyCount = model.readyPapers.length;
  const showCheck = (row: AttemptListRow, focus: boolean) => {
    const group = model.readyGroups.find(
      (candidate) => candidate.assignmentId === row.assignmentId,
    );
    if (group && group.rows.findIndex((paper) => paper.id === row.id) >= ROWS_SHOWN)
      setExpanded((open) => new Set(open).add(row.assignmentId));
    model.openCheck(row);
    setFocusFinish(focus ? row.id : null);
  };
  const toggleCheck = (row: AttemptListRow) => {
    if (model.candidate?.id === row.id) {
      model.openCheck(null);
      setFocusFinish(null);
    } else showCheck(row, false);
  };
  const finishPaper = async () => {
    const row = model.candidate;
    if (!row) return;
    setFocusFinish(null);
    setFinishing(row.id);
    const outcome = await model.finishCandidate();
    setFinishing(null);
    if (!outcome) return;
    if (outcome.next) showCheck(outcome.next, true);
    else {
      focusEmpty.current = true;
      setFocusTick((tick) => tick + 1);
    }
  };
  const toggleMore = (assignmentId: string) =>
    setExpanded((open) => {
      const next = new Set(open);
      if (next.has(assignmentId)) next.delete(assignmentId);
      else next.add(assignmentId);
      return next;
    });
  const focusHeading =
    (location.state as { focus?: string } | null)?.focus === HEADING_FOCUS;
  useEffect(() => {
    if (focusHeading) heading.current?.focus();
  }, [focusHeading]);
  const departing = useRef(false);
  const locationKey = location.key;
  useEffect(() => {
    departing.current = false;
  }, [locationKey]);
  const blocker = useBlocker(
    ({ currentLocation, nextLocation }) =>
      !departing.current &&
      model.unsettled &&
      (currentLocation.pathname !== nextLocation.pathname ||
        currentLocation.search !== nextLocation.search),
  );
  const unsettled = model.unsettled;
  useEffect(() => {
    if (!unsettled) return;
    const warn = (event: BeforeUnloadEvent) => event.preventDefault();
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [unsettled]);
  const change = (key: string, value: string, focus?: typeof HEADING_FOCUS) => {
    if (model.busy) return;
    const next = new URLSearchParams(location.search);
    if (value) next.set(key, value);
    else next.delete(key);
    if (next.toString() === new URLSearchParams(location.search).toString()) return;
    void model.leave(() => {
      departing.current = true;
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
      departing.current = true;
      void navigate(to);
    });
  };
  const shortcuts = useEffectEvent((event: KeyboardEvent) => {
    if (
      model.busy ||
      !selected ||
      event.altKey ||
      event.ctrlKey ||
      event.metaKey ||
      event.shiftKey ||
      !pageKey(event)
    )
      return;
    const letter = event.key.toLowerCase();
    if (letter === "j" || letter === "k") {
      event.preventDefault();
      model.move(letter === "j" ? 1 : -1);
      return;
    }
    // An essay's band keys are its card's: a digit is a band there, not a position.
    if (selected.type === "essay") return;
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
    <section aria-label={t("grading.title")} className="flex min-w-0 flex-col gap-4">
      <div
        className={cn(
          "flex min-w-0 flex-col gap-4",
          panes && "min-[768px]:h-[calc(100vh-120px)] min-[768px]:min-h-140",
        )}
      >
        <header className="flex flex-wrap items-end justify-between gap-3">
          <div>
            <h1 ref={heading} tabIndex={-1} className="text-h1 outline-none">
              {t("grading.title")}
            </h1>
            {(model.queue.data || params.assignmentId || params.studentId) && (
              <p className="text-muted-foreground mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-[14px]">
                {model.queue.data &&
                  (model.queue.data.answersRemaining === 0
                    ? t("grading.remainingNone")
                    : t("grading.remaining", {
                        answers: model.queue.data.answersRemaining,
                        students: model.queue.data.studentsWaiting,
                      }))}
                {panes && scan === "done" && readyCount > 0 && (
                  <button
                    type="button"
                    className="text-fg text-[14px] font-medium underline underline-offset-3"
                    onClick={() => {
                      setCardOpen(true);
                      cardHeading.current?.focus();
                    }}
                  >
                    {t("grading.readyLink", { count: readyCount })}
                  </button>
                )}
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
                  "h-7.5 px-3",
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
          <Alert key={state.item.attemptId} variant="danger" className="px-3.5 py-3">
            <CircleAlert aria-hidden="true" />
            <AlertDescription>
              <p className="text-ui font-medium">
                {t("grading.finishFailed", { name: state.item.studentName })}
              </p>
              <div className="mt-2.5 flex flex-wrap gap-2">
                <Button
                  size="sm"
                  disabled={model.busy}
                  className="bg-danger-ink text-bg hover:bg-danger-ink/90 px-3 font-semibold"
                  onClick={() => void model.retryFinish(state)}
                >
                  {t("grading.retryFinish")}
                </Button>
                <Button
                  asChild
                  size="sm"
                  variant="outline"
                  className="text-danger-ink! border-current! bg-transparent! px-3"
                >
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
          <LoadError
            error={model.queue.error}
            onRetry={() => void model.queue.refetch()}
          >
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
            {selected ? (
              <div className="flex min-w-0 flex-wrap items-start gap-3.5 min-[768px]:min-h-0 min-[768px]:flex-[1_1_auto] min-[768px]:flex-nowrap min-[768px]:items-stretch">
                <GradingQueueAside
                  queue={model.queue.data}
                  items={model.items}
                  selected={selected}
                  mode={mode}
                  busy={model.busy}
                  onSelect={(item) => void model.select(item)}
                />
                <div className="w-full min-w-0 flex-[3_1_520px] min-[768px]:flex min-[768px]:min-h-0 min-[768px]:flex-col">
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
                    onBand={(mark) => void model.pickBand(selected, mark)}
                    fieldError={fieldErrorOf(model, selected)}
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
                    suggestion={
                      selected.type === "short_answer" && (
                        <AiScoreBox
                          key={gradingItemKey(selected)}
                          attemptId={selected.attemptId}
                          questionId={selected.questionId}
                          max={selected.points}
                          busy={model.busy}
                          onUse={(points, comment) =>
                            model.fill(selected, points, comment)
                          }
                        />
                      )
                    }
                  />
                </div>
              </div>
            ) : (
              <QueueEmpty
                model={model}
                params={params}
                scan={scan}
                titleRef={emptyTitle}
                onReviewOldest={() => {
                  const oldest = model.readyPapers[0];
                  if (oldest) showCheck(oldest, true);
                }}
                onShowAll={(key) => change(key, "", HEADING_FOCUS)}
              />
            )}
          </>
        )}
      </div>
      {model.queue.data &&
        (panes
          ? readyCount > 0 || scan === "failed"
          : scan !== "done" || readyCount > 0) && (
          <GradingReadyCard
            model={model}
            scan={scan}
            foldable={panes}
            open={!panes || cardOpen}
            headingRef={cardHeading}
            expanded={expanded}
            finishing={finishing}
            focusFinish={focusFinish}
            onFold={() => setCardOpen((open) => !open)}
            onMore={toggleMore}
            onToggleCheck={toggleCheck}
            onFinish={() => void finishPaper()}
            onReview={openReview}
          />
        )}
      <p aria-live="polite" className="sr-only">
        {model.announcement}
      </p>
      <ConfirmDialog
        open={blocker.state === "blocked" || model.leaveHeld}
        onOpenChange={(open) => {
          if (open) return;
          model.keepMarking();
          if (blocker.state === "blocked") blocker.reset();
        }}
        title={t(model.unsavedScore ? "grading.leaveScoreTitle" : "grading.leaveTitle")}
        description={t(leaveBody(model))}
        confirmLabel={t(
          model.partial ? "grading.leaveWithoutSaving" : "grading.saveAndLeave",
        )}
        cancelLabel={t("grading.stay")}
        destructive={model.partial}
        pending={model.busy}
        error={blocker.state === "blocked" ? model.error : null}
        onConfirm={() => {
          const go = () => {
            departing.current = true;
            if (blocker.state === "blocked") blocker.proceed();
          };
          if (model.partial) model.discard(go);
          else void model.leave(go);
        }}
      />
    </section>
  );
}

/** leaveBody is what the leave question says: that part-marked bands are dropped, or what is unsaved. */
function leaveBody(model: { partial: boolean; unsavedScore: boolean }) {
  if (model.partial) return "grading.leavePartialBody";
  return model.unsavedScore ? "grading.leaveScoreBody" : "grading.leaveBody";
}

/** fieldErrorOf is the refused save's message for the open answer's marks, if it is that answer's. */
function fieldErrorOf(
  model: GradingQueueModel,
  item: NonNullable<GradingQueueModel["selected"]>,
) {
  const error = model.fieldError;
  return error?.key === gradingItemKey(item) ? error.message : null;
}

function FilterChips({
  model,
  params,
  onChange,
}: Readonly<{
  model: GradingQueueModel;
  params: GradingQueueParams;
  onChange: (key: string, value: string, focus?: typeof HEADING_FOCUS) => void;
}>) {
  const { t } = useTranslation();
  const assignmentName = () =>
    knownName(model, "assignment", params.assignmentId ?? "") ??
    t("grading.appliedAssignment");
  const studentName = () =>
    knownName(model, "student", params.studentId ?? "") ?? t("grading.appliedStudent");
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

function readyScan(recovery: GradingQueueModel["recovery"]): ReadyScan {
  if (recovery.isPending || (recovery.isError && recovery.isFetching)) return "running";
  return recovery.isError ? "failed" : "done";
}

function knownName(
  model: GradingQueueModel,
  key: "assignment" | "student",
  id: string,
): string | undefined {
  if (key === "assignment")
    return (
      model.items.find((item) => item.assignmentId === id)?.assignmentTitle ??
      model.recovery.data?.find((row) => row.assignmentId === id)?.testTitle ??
      model.finished.find((item) => item.assignmentId === id)?.assignmentTitle
    );
  return (
    model.items.find((item) => item.studentId === id)?.studentName ??
    model.recovery.data?.find((row) => row.studentId === id)?.studentName ??
    model.finished.find((item) => item.studentId === id)?.studentName
  );
}

function filterName(
  model: GradingQueueModel,
  params: GradingQueueParams,
): { key: "assignment" | "student"; name: string | undefined } | null {
  if (params.assignmentId)
    return {
      key: "assignment",
      name: knownName(model, "assignment", params.assignmentId),
    };
  if (params.studentId)
    return { key: "student", name: knownName(model, "student", params.studentId) };
  return null;
}

function QueueEmpty({
  model,
  params,
  scan,
  titleRef,
  onReviewOldest,
  onShowAll,
}: Readonly<{
  model: GradingQueueModel;
  params: GradingQueueParams;
  scan: ReadyScan;
  titleRef: Ref<HTMLHeadingElement>;
  onReviewOldest: () => void;
  onShowAll: (key: "assignment" | "student") => void;
}>) {
  const { t } = useTranslation();
  const count = model.readyPapers.length;
  const action = "h-11 text-ui rounded-[9px] px-3.5 min-[768px]:h-10 lg:h-9";
  if (scan === "running")
    return (
      <EmptyState
        key="running"
        icon={Hourglass}
        tone="muted"
        titleRef={titleRef}
        hint={t("grading.emptyChecking")}
      >
        {t("grading.emptyToFinish")}
      </EmptyState>
    );
  if (scan === "failed")
    return (
      <EmptyState
        key="failed"
        icon={TriangleAlert}
        tone="warning"
        titleRef={titleRef}
        hint={t("grading.emptyUnsure")}
        action={
          <Button
            variant="outline"
            className={action}
            onClick={() => void model.recovery.refetch()}
          >
            {t("common.retry")}
          </Button>
        }
      >
        {t("grading.emptyToFinish")}
      </EmptyState>
    );
  if (count > 0)
    return (
      <EmptyState
        key="finish"
        icon={ClipboardCheck}
        tone="neutral"
        titleRef={titleRef}
        hint={t("grading.emptyToFinishHint", { count })}
        action={
          <Button
            variant="outline"
            disabled={model.busy}
            className={action}
            onClick={onReviewOldest}
          >
            {t("grading.reviewPapers", { count })}
            <ArrowDown aria-hidden="true" />
          </Button>
        }
      >
        {t("grading.emptyToFinish")}
      </EmptyState>
    );
  const filter = filterName(model, params);
  let hint = t("grading.emptyHint");
  if (filter?.name)
    hint =
      filter.key === "assignment"
        ? t("grading.emptyHintAssignment", { title: filter.name })
        : t("grading.emptyHintStudent", { name: filter.name });
  return (
    <EmptyState
      key="graded"
      icon={CircleCheck}
      tone="success"
      titleRef={titleRef}
      hint={hint}
      action={
        filter ? (
          <Button
            variant="outline"
            disabled={model.busy}
            className={action}
            onClick={() => onShowAll(filter.key)}
          >
            {t(
              filter.key === "assignment"
                ? "grading.showAllAssignments"
                : "grading.showAllStudents",
            )}
          </Button>
        ) : undefined
      }
    >
      {t("grading.empty")}
    </EmptyState>
  );
}
