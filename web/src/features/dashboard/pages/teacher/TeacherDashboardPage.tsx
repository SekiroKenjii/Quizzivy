import { useRef, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link, useLocation, useNavigate } from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Activity, ArrowRight, Clock, Flag, Plus, Send, SquarePen } from "lucide-react";
import { Avatar } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { BarChart } from "@/components/shared/charts/BarChart";
import { KpiTile, type KpiTileProps } from "@/components/shared/stats/KpiTile";
import { ProgressBar } from "@/components/shared/stats/ProgressBar";
import {
  EmptyState,
  ListSkeleton,
  LoadError,
  QueryStates,
} from "@/components/shared/ListState";
import { useCan, useWorkspace } from "@/features/auth/permissions";
import { givenName } from "@/features/assignments/studentTime";
import { createTest } from "@/features/tests/api";
import { useIdlePolling, useRefetchOnResume } from "@/hooks/useIdlePolling";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { useMinute } from "@/hooks/useTick";
import { useAuthStore } from "@/stores/auth";
import { useLocale } from "@/lib/i18n/useLocale";
import { useDisplayTimeZone, formatRelative } from "@/lib/i18n/datetime";
import {
  getDashboard,
  listAssignments,
  type Assignment,
  type Dashboard,
} from "../../api";
import { dashboardKeys } from "../../keys";
import {
  dashboardActivities,
  dashboardBars,
  dashboardDestinations,
  dashboardGreeting,
  dashboardLongDate,
  dashboardRange,
  dashboardRangeLocation,
  type DashboardRange,
} from "../../view";

const RANGES: readonly DashboardRange[] = ["7d", "14d", "30d"];
const FRAME = "bg-card shadow-card min-w-0 rounded-xl border";

/** TeacherDashboardPage presents the caller's current teaching work with permission-aware real destinations. */
export default function TeacherDashboardPage() {
  const { t } = useTranslation();
  const locale = useLocale();
  const zone = useDisplayTimeZone();
  const location = useLocation();
  const navigate = useNavigate();
  const client = useQueryClient();
  const workspace = useWorkspace("teacher");
  const writeTests = useCan("content.tests.write");
  const assign = useCan("teaching.assignments.write");
  const name = useAuthStore((state) =>
    givenName(state.user?.displayName ?? state.user?.fullName ?? ""),
  );
  const minute = useMinute(workspace);
  const now = new Date(minute * 60_000);
  const range = dashboardRange(location.search);
  const interval = useIdlePolling(30_000, workspace);
  const home = useQuery({
    queryKey: [...dashboardKeys.home(range), zone],
    queryFn: ({ signal }) => getDashboard(signal, range),
    enabled: workspace,
    staleTime: 30_000,
    refetchInterval: interval,
    refetchIntervalInBackground: false,
  });
  const live = useQuery({
    queryKey: dashboardKeys.live,
    queryFn: ({ signal }) => listAssignments({ status: "open", limit: 10 }, signal),
    enabled: workspace,
    staleTime: 30_000,
    refetchInterval: interval,
    refetchIntervalInBackground: false,
  });
  useRefetchOnResume(home.refetch, workspace);
  useRefetchOnResume(live.refetch, workspace);
  const creating = useRef(false);
  const create = useMutation({
    mutationFn: () => createTest(t("tests.untitled")),
    onSuccess: async (test) => {
      await client.invalidateQueries({ queryKey: ["admin-tests"] });
      if (workspace && writeTests) void navigate(`/teacher/tests/${test.id}/edit`);
    },
    onSettled: () => {
      creating.current = false;
    },
  });
  const newTest = () => {
    if (!workspace || !writeTests || creating.current || create.isPending) return;
    creating.current = true;
    create.mutate();
  };
  if (!workspace) return null;
  return (
    <div className="flex min-w-0 flex-col gap-5" data-slot="teacher-dashboard">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div className="min-w-0">
          <h1 className="text-h1 font-semibold tracking-[-0.02em] text-balance break-words">
            {t(`dashboard.home.greeting.${dashboardGreeting(now)}`, {
              name: name || t("dashboard.home.friend"),
            })}
          </h1>
          <p className="text-muted-fg mt-0.5 text-base">
            {dashboardLongDate(now, locale)}
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          {writeTests && (
            <Button variant="outline" disabled={create.isPending} onClick={newTest}>
              <Plus aria-hidden="true" />
              {t("dashboard.home.newTest")}
            </Button>
          )}
          {assign && <AssignButton />}
        </div>
      </header>
      {writeTests && create.isError && (
        <LoadError error={create.error} onRetry={newTest}>
          {t("dashboard.home.createFailed")}
        </LoadError>
      )}
      <QueryStates
        query={home}
        skeleton={<ListSkeleton rows={4} />}
        failed={t("dashboard.home.loadFailed")}
      >
        {(data) => <DashboardKpis data={data} />}
      </QueryStates>
      <div className="flex flex-wrap gap-3">
        <section
          className={`${FRAME} flex-[2_1_520px] px-4.5 pt-4.5 pb-3.5`}
          aria-labelledby="dashboard-submissions"
        >
          <div className="flex flex-wrap items-start justify-between gap-3">
            <h2 id="dashboard-submissions" className="text-md font-semibold">
              {t("dashboard.home.submissions")}
            </h2>
            <div
              role="group"
              aria-label={t("dashboard.home.rangeLabel")}
              className="bg-muted flex gap-0.5 rounded-lg p-0.75"
            >
              {RANGES.map((value) => (
                <button
                  key={value}
                  type="button"
                  aria-pressed={value === range}
                  onClick={() => {
                    if (value !== range)
                      void navigate(dashboardRangeLocation(location, value));
                  }}
                  className={`rounded-seg text-meta h-6.5 px-2.5 font-medium ${value === range ? "bg-card text-fg shadow-card" : "text-muted-fg"}`}
                >
                  {t("dashboard.home.range", { count: Number.parseInt(value) })}
                </button>
              ))}
            </div>
          </div>
          <QueryStates
            query={home}
            skeleton={<ListSkeleton rows={3} />}
            failed={t("dashboard.home.loadFailed")}
          >
            {(data) => <Submissions data={data} range={range} />}
          </QueryStates>
        </section>
        <section
          className={`${FRAME} flex-[1_1_300px] p-4.5`}
          aria-labelledby="dashboard-today"
        >
          <div className="flex items-center justify-between gap-2">
            <h2 id="dashboard-today" className="text-md font-semibold">
              {t("dashboard.home.today")}
            </h2>
            <time className="text-muted-fg text-meta">
              {new Intl.DateTimeFormat(locale, {
                timeZone: zone,
                weekday: "short",
                day: "numeric",
              }).format(now)}
            </time>
          </div>
          <QueryStates
            query={home}
            skeleton={<ListSkeleton rows={3} />}
            failed={t("dashboard.home.loadFailed")}
          >
            {(data) => <Today data={data} />}
          </QueryStates>
        </section>
      </div>
      <div className="flex flex-wrap gap-3">
        <section
          className={`${FRAME} flex-[2_1_520px] overflow-hidden`}
          aria-labelledby="dashboard-live"
        >
          <div className="flex items-center justify-between gap-3 px-4.5 pt-4 pb-3">
            <h2 id="dashboard-live" className="text-md font-semibold">
              {t("dashboard.home.live")}
            </h2>
            <Link
              to="/teacher/assignments"
              className="text-muted-fg hover:text-fg flex shrink-0 items-center gap-1 text-sm"
            >
              {t("dashboard.home.viewAll")}
              <ArrowRight aria-hidden="true" className="size-3.5" />
            </Link>
          </div>
          <QueryStates
            query={live}
            skeleton={<ListSkeleton rows={3} />}
            failed={t("dashboard.home.liveFailed")}
            className="p-4.5"
          >
            {(data) => (
              <LiveAssignments
                items={data.items}
                all={data.facets.all}
                assign={assign}
              />
            )}
          </QueryStates>
        </section>
        <section
          className={`${FRAME} flex-[1_1_300px] px-4.5 py-4`}
          aria-labelledby="dashboard-activity"
        >
          <h2 id="dashboard-activity" className="text-md mb-1.5 font-semibold">
            {t("dashboard.home.activity")}
          </h2>
          <QueryStates
            query={home}
            skeleton={<ListSkeleton rows={3} />}
            failed={t("dashboard.home.loadFailed")}
          >
            {(data) => <RecentActivity data={data} />}
          </QueryStates>
        </section>
      </div>
    </div>
  );
}

function AssignButton() {
  const { t } = useTranslation();
  return (
    <Button asChild>
      <Link to="/teacher/assignments/new">
        <Send aria-hidden="true" />
        {t("dashboard.home.assignTest")}
      </Link>
    </Button>
  );
}

function Kpi({
  figure,
  to,
  action,
}: Readonly<{
  figure: Omit<KpiTileProps, "to" | "action">;
  to: string | null;
  action: string;
}>) {
  return <KpiTile {...figure} {...(to === null ? {} : { to, action })} />;
}

function DashboardKpis({ data }: Readonly<{ data: Dashboard }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const grade = useCan("teaching.grading");
  const intervene = useCan("teaching.attempts.intervene");
  const to = dashboardDestinations(data, grade, grade || intervene);
  const waiting =
    data.waitingStudents === undefined
      ? "—"
      : t("dashboard.home.waitingHint", {
          count: data.waitingStudents,
          age: data.oldestWaitingAt
            ? formatRelative(data.oldestWaitingAt, locale)
            : "—",
        });
  const closing = data.nextClosing
    ? t("dashboard.home.closingHint", {
        title: data.nextClosing.title,
        submitted: data.nextClosing.submittedCount,
        target: data.nextClosing.targetCount,
      })
    : "—";
  return (
    <div className="grid grid-cols-[repeat(auto-fit,minmax(min(210px,100%),1fr))] gap-3">
      <Kpi
        figure={{
          label: t("dashboard.home.toGrade"),
          icon: SquarePen,
          tone: "warning",
          value: String(data.awaitingGrading),
          hint: waiting,
        }}
        to={to.grade}
        action={t("dashboard.home.grade")}
      />
      <Kpi
        figure={{
          label: t("dashboard.home.flagged"),
          icon: Flag,
          tone: "danger",
          value: String(data.flaggedAttempts),
          hint: t("dashboard.home.flaggedHint"),
        }}
        to={to.flagged}
        action={t("dashboard.home.review")}
      />
      <Kpi
        figure={{
          label: t("dashboard.home.closing"),
          icon: Clock,
          tone: "muted",
          value: data.closingSoon === undefined ? "—" : String(data.closingSoon),
          hint: closing,
        }}
        to={to.closing}
        action={t("dashboard.home.monitor")}
      />
      <Kpi
        figure={{
          label: t("dashboard.home.taking"),
          icon: Activity,
          tone: "success",
          value: String(data.takingNow.students),
          hint: t("dashboard.home.across", { count: data.takingNow.assignments }),
          live: true,
        }}
        to={to.taking}
        action={t("dashboard.home.watch")}
      />
    </div>
  );
}

function Submissions({
  data,
  range,
}: Readonly<{ data: Dashboard; range: DashboardRange }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const wide = useMediaQuery("(min-width: 768px)");
  const n = new Intl.NumberFormat(locale);
  const average =
    data.submissions.averagePercent === null
      ? t("dashboard.home.noAverage")
      : `${n.format(data.submissions.averagePercent)}%`;
  return (
    <>
      <p className="text-muted-fg mt-0.5 text-sm" data-slot="submission-summary">
        {t("dashboard.home.submissionSummary", {
          total: n.format(data.submissions.total),
          days: Number.parseInt(range),
          average,
        })}
      </p>
      <BarChart
        className="mt-4.5"
        caption={t("dashboard.home.submissions")}
        columns={[t("dashboard.home.chartDate"), t("dashboard.home.submissions")]}
        labelEvery={wide ? 1 : 2}
        data={dashboardBars(data.submissions.days, locale, (count, date) =>
          t("dashboard.home.chartTip", { count, date }),
        )}
      />
    </>
  );
}

function Today({ data }: Readonly<{ data: Dashboard }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const zone = useDisplayTimeZone();
  if (data.today.length === 0)
    return <EmptyState>{t("dashboard.home.noToday")}</EmptyState>;
  return (
    <ul className="mt-3">
      {data.today.map((event) => (
        <li
          key={`${event.assignmentId}:${event.kind}:${event.at}`}
          className="flex gap-3 border-t py-2.5 first:border-t-0"
        >
          <time
            dateTime={event.at}
            className="text-muted-fg text-meta w-11 shrink-0 pt-px tabular-nums"
          >
            {new Intl.DateTimeFormat(locale, {
              timeZone: zone,
              hour: "2-digit",
              minute: "2-digit",
              hourCycle: "h23",
            }).format(new Date(event.at))}
          </time>
          <span
            aria-hidden="true"
            className={`w-0.75 shrink-0 rounded-sm ${event.kind === "closes" ? "bg-warning" : "bg-brand"}`}
          />
          <span className="min-w-0 flex-1">
            <Link
              to={`/teacher/assignments/${event.assignmentId}?tab=students`}
              className="text-ui block font-medium break-words hover:underline"
            >
              {t(`dashboard.home.${event.kind}`, { title: event.title })}
            </Link>
            {event.kind === "closes" && (
              <span className="text-muted-fg text-meta block">
                {t("dashboard.home.notSubmitted", { count: event.notSubmitted })}
              </span>
            )}
          </span>
        </li>
      ))}
    </ul>
  );
}

function LiveAssignments({
  items,
  all,
  assign,
}: Readonly<{ items: Assignment[]; all: number; assign: boolean }>) {
  const { t } = useTranslation();
  if (items.length === 0)
    return (
      <div className="p-4.5">
        <EmptyState action={assign ? <AssignButton /> : undefined}>
          {t(all === 0 ? "dashboard.home.empty" : "dashboard.home.noLive")}
        </EmptyState>
      </div>
    );
  return (
    <ul>
      {items.map((assignment) => (
        <li key={assignment.id}>
          <LiveAssignment assignment={assignment} />
        </li>
      ))}
    </ul>
  );
}

function LiveAssignment({ assignment }: Readonly<{ assignment: Assignment }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const known =
    assignment.submittedCount !== undefined && assignment.targetCount !== undefined;
  let progress: ReactNode = (
    <span className="text-muted-fg text-meta">
      {t("dashboard.home.progressUnknown")}
    </span>
  );
  if (known)
    progress = (
      <span className="flex min-w-35 flex-[0_1_180px] items-center gap-2.5">
        <ProgressBar
          value={assignment.submittedCount!}
          max={assignment.targetCount!}
          label={t("dashboard.home.submittedCount", {
            submitted: assignment.submittedCount,
            target: assignment.targetCount,
          })}
          cap="round"
          className="flex-1"
        />
        <span className="text-muted-fg text-meta text-right tabular-nums">
          {assignment.submittedCount}/{assignment.targetCount}
        </span>
      </span>
    );
  return (
    <Link
      to={`/teacher/assignments/${assignment.id}?tab=students`}
      className="hover:bg-muted flex flex-wrap items-center gap-3.5 border-t px-4.5 py-3"
    >
      <span className="min-w-0 flex-[1_1_220px]">
        <span className="text-ui block font-medium break-words">
          {assignment.testTitle}
        </span>
        <span className="text-muted-fg text-meta block">
          {assignment.targets.classes.map((item) => item.name).join(", ")}
          {assignment.targets.classes.length > 0 ? " · " : ""}
          {t("dashboard.home.closesWhen", {
            when: formatRelative(assignment.window.closesAt, locale),
          })}
        </span>
      </span>
      {progress}
      {(assignment.pendingGradingCount ?? 0) > 0 && (
        <Badge className="bg-warning-soft text-warning-ink rounded-full border-0 px-2 py-0.25 text-xs">
          {t("dashboard.home.rowToGrade", { count: assignment.pendingGradingCount })}
        </Badge>
      )}
    </Link>
  );
}

function RecentActivity({ data }: Readonly<{ data: Dashboard }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  if (data.recentActivity.length === 0)
    return <EmptyState>{t("dashboard.home.noActivity")}</EmptyState>;
  return (
    <ul>
      {dashboardActivities(data.recentActivity).map((event) => (
        <li key={event.key} className="flex gap-2.5 border-t py-2.5 first:border-t-0">
          <Avatar name={event.studentName} className="text-2xs size-7.5" />
          <span className="min-w-0 flex-1 text-sm leading-[1.45]">
            <span className="font-medium">{event.studentName}</span>{" "}
            {t(`dashboard.home.${event.kind}`, { subject: event.subject })}
            <span className="text-muted-fg mt-px flex flex-wrap items-center gap-1.5 text-xs">
              <time dateTime={event.at}>{formatRelative(event.at, locale)}</time>
              {event.flagged && (
                <span className="text-danger-ink inline-flex items-center gap-0.75">
                  <Flag aria-hidden="true" className="size-3" />
                  {t("dashboard.home.activityFlag")}
                </span>
              )}
            </span>
          </span>
        </li>
      ))}
    </ul>
  );
}
