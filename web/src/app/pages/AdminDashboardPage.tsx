import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { useQueries } from "@tanstack/react-query";
import { Plus, Send } from "lucide-react";
import { Avatar } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
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
  listAssignments,
  type Assignment,
} from "@/features/dashboard/api";
import { fetchClasses } from "@/features/classes/api";
import { SUPPORTED_LOCALES, type Locale } from "@/lib/i18n";
import { formatDateTime, formatRelative } from "@/lib/i18n/datetime";
import { PageHeader } from "@/components/shared/PageHeader";

/**
 * §8's /admin, as A-01: a work queue rather than a wall of statistics.
 *
 * The three cards at the top are the only things that can need the teacher
 * today; everything below them is reference. Each card states the number, what
 * it is, and the one action that clears it.
 */
export default function AdminDashboardPage() {
  const { t, i18n } = useTranslation();
  const locale = currentLocale(i18n.language);

  const [summary, open, classes] = useQueries({
    queries: [
      {
        queryKey: ["admin-dashboard"],
        queryFn: ({ signal }: Q) => getDashboard(signal),
      },
      {
        queryKey: ["admin-assignments", "open"],
        queryFn: ({ signal }: Q) => listAssignments({ limit: 10 }, signal),
      },
      { queryKey: ["admin-classes"], queryFn: ({ signal }: Q) => fetchClasses(signal) },
    ],
  });

  const classNames = new Map(
    (classes.data?.items ?? []).map((klass) => [klass.id, klass.name]),
  );

  return (
    <div className="space-y-5">
      <PageHeader
        variant="title"
        title={t("nav.dashboard")}
        subtitle={formatDateTime(new Date(), locale)}
        actions={
          <>
            <Button asChild variant="outline" size="sm">
              <Link to="/admin/tests">
                <Plus aria-hidden="true" />
                {t("tests.new")}
              </Link>
            </Button>
            <Button asChild size="sm">
              <Link to="/admin/assignments">
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

        {summary.isPending ? (
          <p role="status" aria-live="polite" className="text-muted-foreground text-sm">
            {t("common.loading")}
          </p>
        ) : summary.isError ? (
          <p role="alert" className="text-destructive text-sm">
            {t("dashboard.loadFailed")}
          </p>
        ) : (
          <div className="grid gap-4 lg:grid-cols-3">
            <QueueCard
              count={summary.data.awaitingGrading}
              label={t("dashboard.awaitingGrading")}
              hint={t("dashboard.awaitingGradingHint")}
              action={t("dashboard.grade")}
              to="/admin/assignments"
            />
            <QueueCard
              count={summary.data.flaggedAttempts}
              label={t("dashboard.flagged")}
              hint={t("dashboard.flaggedHint")}
              action={t("dashboard.review")}
              to="/admin/assignments"
            />
            <QueueCard
              count={summary.data.openAssignments}
              label={t("dashboard.openAssignments")}
              hint={t("dashboard.openAssignmentsHint")}
              action={t("dashboard.monitor")}
              to="/admin/assignments"
            />
          </div>
        )}
      </section>

      <div className="grid gap-4 lg:grid-cols-3">
        <Card asChild className="gap-0 py-0 lg:col-span-2">
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

            {open.isPending ? (
              <p
                role="status"
                aria-live="polite"
                className="text-muted-foreground px-5 pb-6 text-sm"
              >
                {t("common.loading")}
              </p>
            ) : open.isError ? (
              <p role="alert" className="text-destructive px-5 pb-6 text-sm">
                {t("dashboard.loadFailed")}
              </p>
            ) : open.data.items.length === 0 ? (
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
                    <TableHead className="w-[180px]">
                      {t("dashboard.progress")}
                    </TableHead>
                    <TableHead className="w-24">
                      <span className="sr-only">{t("dashboard.state")}</span>
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {open.data.items.map((assignment) => (
                    <AssignmentRow
                      key={assignment.id}
                      assignment={assignment}
                      classNames={classNames}
                      locale={locale}
                    />
                  ))}
                </TableBody>
              </Table>
            )}
          </section>
        </Card>

        <Card asChild className="gap-0 py-0">
          <section aria-labelledby="activity-heading" className="self-start">
            <div className="px-5 pt-4 pb-3">
              <h2
                id="activity-heading"
                className="text-[0.9375rem] font-semibold tracking-[-0.01em]"
              >
                {t("dashboard.recent")}
              </h2>
            </div>
            <div className="space-y-3 px-5 pb-4">
              {summary.data?.recentAttempts.length === 0 ? (
                <p className="text-muted-foreground text-sm">
                  {t("dashboard.noActivity")}
                </p>
              ) : (
                (summary.data?.recentAttempts ?? []).map((attempt) => (
                  <div key={attempt.id} className="flex items-start gap-2.5">
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
                <div className="flex items-center justify-between border-t pt-3 text-sm">
                  <span className="text-muted-foreground">
                    {t("dashboard.activeStudents")}
                  </span>
                  <span className="tabular-nums">{summary.data.activeStudents}</span>
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
}: {
  count: number;
  label: string;
  hint: string;
  action: string;
  to: string;
}) {
  return (
    <Card className="flex-row items-center gap-4 p-4">
      <span className="text-2xl font-semibold tabular-nums">{count}</span>
      <div className="min-w-0 flex-1">
        <p className="text-sm font-medium">{label}</p>
        <p className="text-muted-foreground text-xs leading-relaxed">{hint}</p>
      </div>
      <Button asChild variant="outline" size="sm" disabled={count === 0}>
        <Link to={to}>{action}</Link>
      </Button>
    </Card>
  );
}

function AssignmentRow({
  assignment,
  classNames,
  locale,
}: {
  assignment: Assignment;
  classNames: Map<string, string>;
  locale: Locale;
}) {
  const { t } = useTranslation();
  const status = statusAt(assignment, new Date());
  const submitted = assignment.submittedCount ?? 0;
  const target = assignment.targetCount ?? 0;
  const percent = target === 0 ? 0 : Math.round((submitted / target) * 100);

  return (
    <TableRow>
      <TableCell className="font-medium">{assignment.testTitle}</TableCell>
      <TableCell className="text-muted-foreground">
        {assignment.targets.classIds
          .map((id) => classNames.get(id) ?? "—")
          .join(", ") || t("dashboard.byStudent")}
      </TableCell>
      <TableCell className="text-muted-foreground whitespace-nowrap">
        {formatDateTime(assignment.window.closesAt, locale)}
      </TableCell>
      <TableCell>
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
      </TableCell>
      {/* Its own right-aligned column, as A-01 draws it. Folded into the
        progress cell it widened that column enough to push the table into a
        horizontal scrollbar at 1440px. */}
      <TableCell className="text-right">
        <Badge variant={status === "open" ? "warning" : "outline"}>
          {t(`dashboard.assignmentStatus.${status}`)}
        </Badge>
      </TableCell>
    </TableRow>
  );
}

function currentLocale(language: string): Locale {
  return (SUPPORTED_LOCALES as readonly string[]).includes(language)
    ? (language as Locale)
    : "vi";
}
