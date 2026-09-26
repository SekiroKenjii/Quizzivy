import { useTranslation } from "react-i18next";
import { Link, useNavigate } from "react-router";
import { useMutation, useQueries, useQueryClient } from "@tanstack/react-query";
import { ArrowUpRight, Plus, Send } from "lucide-react";
import { ListSkeleton, QueryStates } from "@/components/shared/ListState";
import { Skeleton } from "@/components/ui/skeleton";
import { createTest } from "@/features/tests/api";
import { Avatar } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { statusAt } from "@/features/assignments/status";
import {
  getDashboard,
  listDashboardAssignments,
  type Assignment,
} from "@/features/dashboard/api";
import { useLocale } from "@/lib/i18n/useLocale";
import { compactMoment, weekdayDate, formatRelative } from "@/lib/i18n/datetime";
import { PageHeader } from "@/components/shared/PageHeader";
import { StatusBadge } from "@/components/shared/StatusBadge";

/**
 * §8's /admin, as A-01: a work queue rather than a wall of statistics.
 *
 * The three cards at the top are the only things that can need the teacher
 * today; everything below them is reference. Each card states the number, what
 * it is, and the one action that clears it.
 */
export default function AdminDashboardPage() {
  const { t } = useTranslation();
  const locale = useLocale();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  // A-01's "Đề thi mới" does what the tests list's does: a draft, then the builder.
  const create = useMutation({
    mutationFn: () => createTest(t("tests.untitled")),
    onSuccess: async (test) => {
      await queryClient.invalidateQueries({ queryKey: ["admin-tests"] });
      void navigate(`/admin/tests/${test.id}/edit`);
    },
  });

  const [summary, open] = useQueries({
    queries: [
      {
        queryKey: ["admin-dashboard"],
        queryFn: ({ signal }: Q) => getDashboard(signal),
      },
      {
        queryKey: ["admin-assignments", "open"],
        queryFn: ({ signal }: Q) => listDashboardAssignments(signal),
      },
    ],
  });

  return (
    <div className="space-y-8">
      <PageHeader
        variant="title"
        title={t("nav.dashboard")}
        subtitle={`${weekdayDate(new Date(), locale, true)} · ${t("dashboard.overviewHint")}`}
        actions={
          <>
            <Button
              variant="outline"
              size="sm"
              disabled={create.isPending}
              onClick={() => create.mutate()}
            >
              <Plus aria-hidden="true" />
              {t("tests.new")}
            </Button>
            <Button asChild size="sm">
              <Link to="/admin/assignments/new">
                <Send aria-hidden="true" />
                {t("dashboard.assign")}
              </Link>
            </Button>
          </>
        }
      />

      <section aria-labelledby="queue-heading">
        <h2
          id="queue-heading"
          className="text-muted-foreground mb-3 text-xs font-medium tracking-wide uppercase"
        >
          {t("dashboard.needsYou")}
        </h2>

        <QueryStates
          query={summary}
          skeleton={
            <div
              role="status"
              aria-live="polite"
              aria-label={t("common.loading")}
              className="grid gap-4 lg:grid-cols-3"
            >
              <Skeleton className="h-[4.5rem]" />
              <Skeleton className="h-[4.5rem]" />
              <Skeleton className="h-[4.5rem]" />
            </div>
          }
          failed={t("dashboard.loadFailed")}
        >
          {(data) => (
            <div className="grid gap-4 lg:grid-cols-3">
              <QueueCard
                count={data.awaitingGrading}
                label={t("dashboard.awaitingGrading")}
                hint={
                  data.oldestWaitingAt
                    ? t("dashboard.waitingContext", {
                        count: data.waitingStudents ?? 0,
                        age: formatRelative(data.oldestWaitingAt, locale),
                      })
                    : t("dashboard.noWaiting")
                }
                action={t("dashboard.grade")}
                to="/admin/grading"
              />
              <QueueCard
                count={data.flaggedAttempts}
                label={t("dashboard.flagged")}
                hint={t("dashboard.flaggedHint")}
                action={t("dashboard.review")}
                to="/admin/grading?tab=flagged"
              />
              <QueueCard
                count={data.closingSoon ?? 0}
                label={t("dashboard.closingSoon")}
                hint={
                  data.nextClosing
                    ? t("dashboard.closingContext", {
                        title: data.nextClosing.title,
                        when: formatRelative(data.nextClosing.closesAt, locale),
                        submitted: data.nextClosing.submittedCount,
                        total: data.nextClosing.targetCount,
                      })
                    : t("dashboard.noClosingSoon")
                }
                action={t("dashboard.monitor")}
                to={
                  data.nextClosing
                    ? `/admin/assignments/${data.nextClosing.id}`
                    : "/admin/assignments"
                }
              />
            </div>
          )}
        </QueryStates>
      </section>

      <div className="space-y-6">
        <Card asChild className="min-w-0 gap-0 overflow-hidden py-0 shadow-sm">
          <section aria-labelledby="open-heading">
            <div className="flex items-center justify-between gap-3 px-5 pt-4 pb-3">
              <h2
                id="open-heading"
                className="text-[0.9375rem] font-semibold tracking-[-0.01em]"
              >
                {t("dashboard.openNow")}
              </h2>
              <Link
                to="/admin/assignments"
                className="text-muted-foreground hover:text-foreground text-sm"
              >
                {t("dashboard.allAssignments")}
              </Link>
            </div>

            <p className="text-muted-foreground px-5 pb-4 text-xs">
              {t("dashboard.activeAssignmentsHint")}
            </p>
            <QueryStates
              query={open}
              skeleton={<ListSkeleton rows={3} />}
              failed={t("dashboard.loadFailed")}
              className="px-5 pb-5"
            >
              {(data) =>
                data.items.length === 0 ? (
                  <p className="text-muted-foreground px-5 pb-6 text-sm">
                    {t("dashboard.noAssignments")}
                  </p>
                ) : (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t("dashboard.assignment")}</TableHead>
                        <TableHead>{t("assignments.classes")}</TableHead>
                        <TableHead>{t("dashboard.closesAt")}</TableHead>
                        <TableHead className="w-40">
                          {t("dashboard.progress")}
                        </TableHead>
                        <TableHead className="w-24">
                          <span className="sr-only">{t("dashboard.state")}</span>
                        </TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {data.items.map((assignment) => (
                        <AssignmentRow key={assignment.id} assignment={assignment} />
                      ))}
                    </TableBody>
                  </Table>
                )
              }
            </QueryStates>
          </section>
        </Card>

        <Card asChild className="gap-0 py-0 shadow-sm">
          <section aria-labelledby="activity-heading" className="self-start">
            <div className="px-5 pt-4 pb-3">
              <h2
                id="activity-heading"
                className="text-[0.9375rem] font-semibold tracking-[-0.01em]"
              >
                {t("dashboard.recent")}
              </h2>
            </div>
            <div className="grid gap-4 px-5 pb-5 md:grid-cols-2 xl:grid-cols-3">
              {summary.data?.recentAttempts.length === 0 ? (
                <p className="text-muted-foreground text-sm">
                  {t("dashboard.noActivity")}
                </p>
              ) : (
                (summary.data?.recentAttempts ?? []).map((attempt) => (
                  <div
                    key={attempt.id}
                    className="flex min-w-0 items-start gap-3 rounded-md border p-3"
                  >
                    <Avatar name={attempt.studentName} size="sm" className="mt-0.5" />
                    <div className="min-w-0">
                      <p className="truncate text-sm">
                        <span className="font-medium">{attempt.studentName}</span>{" "}
                        {t(`dashboard.status.${attempt.status}`)} {attempt.testTitle}
                      </p>
                      <p className="text-muted-foreground text-xs">
                        {attempt.submittedAt
                          ? formatRelative(attempt.submittedAt, locale)
                          : t("dashboard.inProgress")}
                        {attempt.flagged ? ` · ${t("dashboard.flaggedShort")}` : ""}
                      </p>
                    </div>
                  </div>
                ))
              )}

              {summary.data ? (
                <div className="flex items-center justify-between border-t pt-4 text-sm md:col-span-2 xl:col-span-3">
                  <span className="text-muted-foreground">
                    {t("dashboard.activeStudents")}
                  </span>
                  <span className="tabular-nums">
                    {summary.data.activeStudents} / {summary.data.totalStudents ?? "—"}
                  </span>
                </div>
              ) : null}
            </div>
          </section>
        </Card>
      </div>
    </div>
  );
}

interface Q {
  signal: AbortSignal;
}

function QueueCard({
  count,
  label,
  hint,
  action,
  to,
}: Readonly<{
  count: number;
  label: string;
  hint: string;
  action: string;
  to: string;
}>) {
  return (
    <Card className="surface-lift min-w-0 gap-4 p-5 shadow-sm">
      <div className="flex items-center justify-between gap-3">
        <span className="text-3xl font-semibold tracking-tight tabular-nums">
          {count}
        </span>
        <ArrowUpRight className="text-muted-foreground size-4" aria-hidden="true" />
      </div>
      <div className="min-w-0 flex-1 space-y-1">
        <p className="text-sm font-medium">{label}</p>
        <p className="text-muted-foreground text-xs leading-relaxed">{hint}</p>
      </div>
      {/* A link cannot be disabled, so an empty queue gets a button that is. */}
      {count === 0 ? (
        <Button variant="outline" size="sm" className="self-start" disabled>
          {action}
        </Button>
      ) : (
        <Button asChild variant="outline" size="sm" className="self-start">
          <Link to={to}>{action}</Link>
        </Button>
      )}
    </Card>
  );
}

function AssignmentRow({
  assignment,
}: Readonly<{
  assignment: Assignment;
}>) {
  const { t } = useTranslation();
  const status = statusAt(assignment, new Date());
  const submitted = assignment.submittedCount ?? 0;
  const target = assignment.targetCount ?? 0;
  const percent =
    target === 0 ? 0 : Math.min(100, Math.round((submitted / target) * 100));

  return (
    <TableRow>
      <TableCell className="min-w-48 font-medium whitespace-normal">
        <Link
          to={`/admin/assignments/${assignment.id}`}
          className="inline-flex items-center gap-2 rounded-sm hover:underline"
        >
          {assignment.testTitle}
          <ArrowUpRight className="size-3.5 shrink-0" aria-hidden="true" />
        </Link>
      </TableCell>
      <TableCell className="text-muted-foreground max-w-64 whitespace-normal">
        {assignment.targets.classes.map((klass) => klass.name).join(", ") ||
          t("dashboard.byStudent")}
      </TableCell>
      <TableCell className="text-muted-foreground whitespace-nowrap">
        {compactMoment(assignment.window.closesAt)}
      </TableCell>
      <TableCell>
        {status === "scheduled" ? (
          <span className="text-muted-foreground text-xs">
            {t("dashboard.notOpenYet")}
          </span>
        ) : (
          <div className="flex items-center gap-2">
            <span
              className="bg-secondary h-1.5 flex-1 overflow-hidden rounded-full"
              role="img"
              aria-label={t("dashboard.progressOf", { submitted, target })}
            >
              <span
                className="bg-foreground block h-full rounded-full"
                style={{ width: `${percent}%` }}
              />
            </span>
            <span className="text-muted-foreground text-xs tabular-nums">
              {submitted}/{target}
            </span>
          </div>
        )}
      </TableCell>
      {/* Its own right-aligned column, as A-01 draws it. */}
      <TableCell className="text-right">
        <StatusBadge kind="assignment" status={status} />
      </TableCell>
    </TableRow>
  );
}
