import { useEffect, useId, useMemo, useRef, type MouseEvent, type Ref } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import {
  ArrowUpRight,
  ChevronDown,
  ChevronUp,
  CircleAlert,
  CircleDashed,
  Cpu,
  Flag,
  Info,
  RefreshCw,
  SquarePen,
  TimerOff,
  TriangleAlert,
} from "lucide-react";
import { Avatar } from "@/components/ui/avatar";
import { Skeleton } from "@/components/ui/skeleton";
import { ListSkeleton, LoadError } from "@/components/shared/ListState";
import { FLAGGED } from "@/features/integrity/tones";
import { ApiError } from "@/lib/api/errors";
import { useLocale } from "@/lib/i18n/useLocale";
import { dayDate, dayMonth, formatTime, useDisplayTimeZone } from "@/lib/i18n/datetime";
import { cn } from "@/lib/utils";
import type { AttemptListRow } from "../../api";
import { paperCheck, type PaperKind, type ReadyGroup } from "./gradingRecovery";
import type { GradingQueueModel } from "./useGradingQueue";

/** ReadyScan is how far the check of papers ready to finish has got. */
export type ReadyScan = "running" | "failed" | "done";

/** ROWS_SHOWN is how many papers a group lists before "Show more". */
export const ROWS_SHOWN = 5;

const outline =
  "bg-card hover:bg-hover inline-flex items-center justify-center gap-1.5 rounded-lg border font-medium whitespace-nowrap disabled:pointer-events-none disabled:opacity-60";
const small = "h-11 text-sm min-[768px]:h-9 lg:h-8";
const large = "h-11 text-ui min-[768px]:h-10 lg:h-9";

/**
 * GradingReadyCard is "Ready to finish": the papers whose answers are all
 * marked and which wait for an explicit Finish, grouped by assignment. Each
 * row's Review opens its check, read fresh from `getAttemptForReview`, and
 * only that check offers Finish. While answers wait the card folds to its
 * title line (`foldable`), and `open` says whether its body shows.
 */
export function GradingReadyCard({
  model,
  scan,
  foldable,
  open,
  headingRef,
  expanded,
  finishing,
  focusFinish,
  onFold,
  onMore,
  onToggleCheck,
  onFinish,
  onReview,
}: Readonly<{
  model: GradingQueueModel;
  scan: ReadyScan;
  foldable: boolean;
  open: boolean;
  headingRef: Ref<HTMLHeadingElement>;
  expanded: ReadonlySet<string>;
  finishing: string | null;
  focusFinish: string | null;
  onFold: () => void;
  onMore: (assignmentId: string) => void;
  onToggleCheck: (row: AttemptListRow) => void;
  onFinish: () => void;
  onReview: (event: MouseEvent<HTMLAnchorElement>, to: string) => void;
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const zone = useDisplayTimeZone();
  const heading = useId();
  const body = useId();
  const recovery = model.recovery;
  const refreshing = recovery.isFetching;
  const groups = model.readyGroups;
  const titles = groups.map((group) => group.title);
  const shared = new Set(
    titles.filter((title, index) => titles.indexOf(title) !== index),
  );
  const requestId =
    recovery.error instanceof ApiError ? recovery.error.requestId : undefined;
  const refreshLabel = t(refreshing ? "grading.refreshing" : "grading.refresh");
  return (
    <section
      aria-labelledby={heading}
      data-grading-ready=""
      className="bg-card shadow-card overflow-hidden rounded-xl border"
    >
      <div className="flex flex-wrap items-start justify-between gap-x-4 gap-y-2.5 px-4 py-3.5">
        <div className="flex min-w-0 flex-[1_1_260px] flex-col gap-1">
          <div className="flex items-center gap-2">
            <h2
              id={heading}
              ref={headingRef}
              tabIndex={-1}
              className="text-md font-semibold outline-none"
            >
              {t("grading.recoveryTitle")}
            </h2>
            {scan === "done" && (
              <span className="bg-muted grid h-5 min-w-5.5 place-items-center rounded-full px-1.75 text-xs leading-normal font-semibold">
                {model.readyPapers.length}
              </span>
            )}
          </div>
          {open && (
            <p className="text-muted-fg max-w-[72ch] text-sm text-pretty">
              {t("grading.recoveryHint")}
            </p>
          )}
        </div>
        <div className="flex flex-none items-center gap-2.5">
          {open && (
            <>
              {!refreshing && recovery.dataUpdatedAt > 0 && (
                <span className="text-muted-fg text-meta leading-normal whitespace-nowrap">
                  {t("grading.checkedAt", {
                    time: formatTime(new Date(recovery.dataUpdatedAt), locale, zone),
                  })}
                </span>
              )}
              <button
                type="button"
                disabled={refreshing || model.busy}
                className={cn(outline, small, "min-w-11 px-0 min-[768px]:px-3")}
                onClick={() => {
                  void recovery.refetch();
                  void model.queue.refetch();
                }}
              >
                <RefreshCw aria-hidden="true" className="size-3.5" />
                <span className="max-[767px]:sr-only">{refreshLabel}</span>
              </button>
            </>
          )}
          {foldable && (
            <button
              type="button"
              aria-expanded={open}
              aria-controls={body}
              className={cn(outline, small, "pr-2.5 pl-3")}
              onClick={onFold}
            >
              {t(open ? "grading.hide" : "grading.show")}
              {open ? (
                <ChevronUp aria-hidden="true" className="size-3.75" />
              ) : (
                <ChevronDown aria-hidden="true" className="size-3.75" />
              )}
            </button>
          )}
        </div>
      </div>
      {open && (
        <div id={body}>
          {scan === "failed" && (
            <div
              role="alert"
              className="bg-warning-soft text-warning-ink mx-4 mb-3.5 flex flex-wrap items-center gap-x-3 gap-y-2.5 rounded-[10px] px-3.5 py-3"
            >
              <TriangleAlert aria-hidden="true" className="size-4 shrink-0" />
              <span className="text-ui flex-[1_1_220px] font-medium text-pretty">
                {t("grading.scanFailed")}
              </span>
              <button
                type="button"
                className={cn(
                  small,
                  "rounded-lg border border-current px-3 font-medium whitespace-nowrap",
                )}
                onClick={() => void recovery.refetch()}
              >
                {t("common.retry")}
              </button>
              {requestId !== undefined && (
                <p className="basis-full text-xs">
                  {t("common.requestId")}{" "}
                  <code className="font-mono break-all select-all">{requestId}</code>
                </p>
              )}
            </div>
          )}
          {scan === "running" && (
            <div className="px-4 pt-1 pb-4">
              <ListSkeleton rows={3} />
            </div>
          )}
          {scan !== "running" &&
            groups.map((group) => (
              <ReadyGroupList
                key={group.assignmentId}
                group={group}
                header={groups.length > 1}
                date={
                  shared.has(group.title) && group.earliest
                    ? dayMonth(group.earliest, locale, zone)
                    : null
                }
                more={expanded.has(group.assignmentId)}
                model={model}
                finishing={finishing}
                focusFinish={focusFinish}
                onMore={() => onMore(group.assignmentId)}
                onToggleCheck={onToggleCheck}
                onFinish={onFinish}
                onReview={onReview}
              />
            ))}
        </div>
      )}
    </section>
  );
}

function ReadyGroupList({
  group,
  header,
  date,
  more,
  model,
  finishing,
  focusFinish,
  onMore,
  onToggleCheck,
  onFinish,
  onReview,
}: Readonly<{
  group: ReadyGroup;
  header: boolean;
  date: string | null;
  more: boolean;
  model: GradingQueueModel;
  finishing: string | null;
  focusFinish: string | null;
  onMore: () => void;
  onToggleCheck: (row: AttemptListRow) => void;
  onFinish: () => void;
  onReview: (event: MouseEvent<HTMLAnchorElement>, to: string) => void;
}>) {
  const { t } = useTranslation();
  const rows = more ? group.rows : group.rows.slice(0, ROWS_SHOWN);
  const hidden = group.rows.length - ROWS_SHOWN;
  return (
    <div className="border-t">
      {header && (
        <h3 className="bg-sidebar text-meta flex flex-wrap items-center gap-x-1.5 gap-y-0.5 px-4 py-2.25 leading-normal font-normal">
          <strong className="font-semibold">{group.title}</strong>
          {date !== null && <span className="text-muted-fg">· {date}</span>}
          <span className="text-muted-fg">
            · {t("grading.groupCount", { count: group.rows.length })}
          </span>
        </h3>
      )}
      <ul>
        {rows.map((row, index) => (
          <ReadyRow
            key={row.id}
            row={row}
            first={index === 0}
            open={model.candidate?.id === row.id}
            model={model}
            finishing={finishing === row.id}
            focusFinish={focusFinish === row.id}
            onToggle={() => onToggleCheck(row)}
            onFinish={onFinish}
            onReview={onReview}
          />
        ))}
      </ul>
      {hidden > 0 && (
        <div className="flex border-t px-2">
          <button
            type="button"
            aria-expanded={more}
            className="text-muted-fg hover:text-fg inline-flex h-11 flex-1 items-center justify-center gap-1.5 text-sm font-medium min-[768px]:h-10 lg:h-9"
            onClick={onMore}
          >
            {more ? t("grading.showFewer") : t("grading.showMore", { count: hidden })}
            {more ? (
              <ChevronUp aria-hidden="true" className="size-3.5" />
            ) : (
              <ChevronDown aria-hidden="true" className="size-3.5" />
            )}
          </button>
        </div>
      )}
    </div>
  );
}

function ReadyRow({
  row,
  first,
  open,
  model,
  finishing,
  focusFinish,
  onToggle,
  onFinish,
  onReview,
}: Readonly<{
  row: AttemptListRow;
  first: boolean;
  open: boolean;
  model: GradingQueueModel;
  finishing: boolean;
  focusFinish: boolean;
  onToggle: () => void;
  onFinish: () => void;
  onReview: (event: MouseEvent<HTMLAnchorElement>, to: string) => void;
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const zone = useDisplayTimeZone();
  const check = useId();
  return (
    <li className={cn(!first && "border-t", open && "bg-sidebar")}>
      <div className="flex items-center gap-3 px-3.5 py-3 min-[768px]:px-4 min-[768px]:py-2.5">
        <Avatar name={row.studentName} size="30" />
        <span className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="text-ui truncate leading-normal font-medium">
            {row.studentName}
          </span>
          <span className="text-muted-fg flex flex-wrap items-center gap-x-2 gap-y-1 text-xs leading-normal">
            {row.submittedAt && (
              <span>
                {t("grading.handedIn", {
                  time: `${dayDate(row.submittedAt, locale, zone)}, ${formatTime(row.submittedAt, locale, zone)}`,
                })}
              </span>
            )}
            {row.status === "timed_out" && (
              <span className="text-caption inline-flex h-5 items-center gap-1 rounded-full border px-1.75 whitespace-nowrap">
                <TimerOff aria-hidden="true" className="size-3" />
                {t("grading.timedOut")}
              </span>
            )}
            {row.flagged && (
              <span
                className={cn(
                  "text-caption inline-flex h-5 items-center gap-1 rounded-full px-1.75 font-medium whitespace-nowrap",
                  FLAGGED.soft,
                  FLAGGED.ink,
                )}
              >
                <Flag aria-hidden="true" className="size-3" />
                {t("grading.flagged")}
              </span>
            )}
          </span>
        </span>
        <button
          type="button"
          aria-expanded={open}
          aria-controls={open ? check : undefined}
          aria-label={
            open
              ? undefined
              : t("grading.candidate", { name: row.studentName, title: row.testTitle })
          }
          className={cn(outline, small, "flex-none px-3")}
          onClick={onToggle}
        >
          {t(open ? "grading.close" : "grading.review")}
        </button>
      </div>
      {open && (
        <PaperCheckPanel
          id={check}
          row={row}
          model={model}
          finishing={finishing}
          focusFinish={focusFinish}
          onFinish={onFinish}
          onReview={onReview}
        />
      )}
    </li>
  );
}

const KIND_LOOK = {
  hand: { icon: SquarePen, tone: "bg-success-soft text-success-ink" },
  auto: { icon: Cpu, tone: "bg-info-soft text-info-ink" },
  blank: { icon: CircleDashed, tone: "bg-muted text-muted-fg" },
} as const;

function kindLabel(marking: PaperKind, t: ReturnType<typeof useTranslation>["t"]) {
  if (marking.kind === "hand") return t("grading.kindHand", { count: marking.marked });
  return t(marking.kind === "auto" ? "grading.kindAuto" : "grading.kindBlank");
}

function PaperCheckPanel({
  id,
  row,
  model,
  finishing,
  focusFinish,
  onFinish,
  onReview,
}: Readonly<{
  id: string;
  row: AttemptListRow;
  model: GradingQueueModel;
  finishing: boolean;
  focusFinish: boolean;
  onFinish: () => void;
  onReview: (event: MouseEvent<HTMLAnchorElement>, to: string) => void;
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const figure = useMemo(() => new Intl.NumberFormat(locale), [locale]);
  const panel = useRef<HTMLDivElement>(null);
  const finish = useRef<HTMLButtonElement>(null);
  const review = useRef<HTMLAnchorElement>(null);
  const query = model.candidateReview;
  const data = query.data;
  const matches =
    data?.attempt.id === row.id &&
    data.attempt.assignmentId === row.assignmentId &&
    data.attempt.studentId === row.studentId;
  const check = matches ? paperCheck(data) : null;
  const finishable = check?.finishable === true;
  const failed = query.isError && !matches;
  useEffect(() => {
    if (!focusFinish) return;
    if (matches) (finishable ? finish.current : review.current)?.focus();
    else if (failed) panel.current?.focus();
  }, [focusFinish, matches, finishable, failed]);
  const to = `/teacher/attempts/${row.id}`;
  const look = check ? KIND_LOOK[check.marking.kind] : null;
  return (
    <div
      id={id}
      ref={panel}
      tabIndex={-1}
      className="flex flex-col gap-3 px-3.5 pt-1 pb-4 outline-none min-[768px]:pr-4 min-[768px]:pl-14.5"
    >
      {!matches && !failed && (
        <div role="status" className="flex flex-col gap-2">
          <span className="text-muted-fg text-meta">{t("grading.checkLoading")}</span>
          <Skeleton className="h-7 w-55 max-w-full rounded-[7px]" />
          <Skeleton className="h-9 w-75 max-w-full rounded-lg" />
        </div>
      )}
      {failed && (
        <LoadError error={query.error} onRetry={() => void query.refetch()}>
          {t("grading.reviewFailed")}
        </LoadError>
      )}
      {check && look && (
        <>
          <div className="flex flex-wrap items-end gap-x-4.5 gap-y-2">
            <span className="flex flex-col">
              <span className="text-muted-fg text-xs">{t("grading.checkScore")}</span>
              <span className="text-[20px] leading-[1.2] font-semibold">
                {check.score
                  ? `${figure.format(check.score.earned)} / ${figure.format(check.score.total)}`
                  : "—"}
              </span>
            </span>
            <span className="text-muted-fg pb-0.5 text-sm">
              {t("grading.checkAnswered", {
                answered: check.answered,
                total: check.questions,
              })}
            </span>
            <span
              className={cn(
                "mb-px inline-flex h-6 items-center gap-1.25 rounded-full px-2.25 text-xs font-medium whitespace-nowrap",
                look.tone,
              )}
            >
              <look.icon aria-hidden="true" className="size-3.25" />
              {kindLabel(check.marking, t)}
            </span>
          </div>
          {check.flagged && (
            <p
              className={cn(
                "flex items-start gap-2 rounded-[9px] px-3 py-2.5 text-sm text-pretty",
                FLAGGED.soft,
                FLAGGED.ink,
              )}
            >
              <Flag aria-hidden="true" className="mt-0.5 size-3.5 shrink-0" />
              <span>{t("grading.checkFlagged", { count: check.focusLosses })}</span>
            </p>
          )}
          {check.marking.kind === "blank" && (
            <p className="bg-muted flex items-start gap-2 rounded-[9px] px-3 py-2.5 text-sm text-pretty">
              <Info
                aria-hidden="true"
                className="text-muted-fg mt-0.5 size-3.5 shrink-0"
              />
              <span>{t("grading.checkBlank")}</span>
            </p>
          )}
          {finishable ? (
            <p className="text-muted-fg text-meta leading-normal">
              {t("grading.ready", { name: row.studentName })}
            </p>
          ) : (
            <p className="flex items-start gap-2 text-sm text-pretty">
              <Info
                aria-hidden="true"
                className="text-muted-fg mt-0.5 size-3.75 shrink-0"
              />
              <span>{t("grading.pendingUnknown")}</span>
            </p>
          )}
          {model.checkFailure === row.id && (
            <p
              role="alert"
              className="text-danger-ink flex items-start gap-2 text-sm font-medium text-pretty"
            >
              <CircleAlert aria-hidden="true" className="mt-0.5 size-3.75 shrink-0" />
              <span>{t("grading.checkFinishFailed")}</span>
            </p>
          )}
          <div className="flex flex-wrap gap-2">
            <button
              ref={finish}
              type="button"
              disabled={model.busy || !finishable}
              className={cn(
                large,
                "bg-primary text-primary-fg flex-[1_1_140px] rounded-[9px] px-4 font-semibold whitespace-nowrap disabled:opacity-40 min-[768px]:flex-none",
                finishing && "disabled:opacity-60",
              )}
              onClick={onFinish}
            >
              {t(finishing ? "grading.finishing" : "review.finish")}
            </button>
            <Link
              ref={review}
              to={to}
              aria-disabled={model.busy}
              className={cn(
                outline,
                large,
                "flex-[1_1_140px] rounded-[9px] px-3.5 min-[768px]:flex-none",
              )}
              onClick={(event) => onReview(event, to)}
            >
              {t("grading.openReview")}
              <ArrowUpRight aria-hidden="true" className="size-3.5" />
            </Link>
          </div>
        </>
      )}
    </div>
  );
}
