import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { ArrowRight } from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
import { LoadError } from "@/components/shared/ListState";
import { listAssignments, type Assignment } from "@/features/assignments/api";
import { AssignmentStatusPill } from "@/features/assignments/pages/teacher/AssignmentCells";
import { statusAt } from "@/features/assignments/status";
import type { Locale } from "@/lib/i18n";
import { formatMoment, useDisplayTimeZone } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";

const SHOWN = 5;
const ORDER = { open: 0, scheduled: 1, draft: 2, closed: 3 } as const;
const ROW =
  "grid items-center gap-3 px-4.5 grid-cols-[minmax(0,2.2fr)_110px_90px] @[560px]/assignments:grid-cols-[minmax(0,2.2fr)_110px_140px_90px]";

/**
 * ClassAssignmentsCard is what the class has been given, as the deck draws
 * it: "{n} assignments · {open} open", View all, and up to five rows, open
 * ones first, each the test with its version, its status, when it closes
 * (from 560px of the card) and how many have handed in. A row opens its
 * assignment. With none yet it says so.
 */
export function ClassAssignmentsCard({ classId }: Readonly<{ classId: string }>) {
  useDisplayTimeZone();
  const { t } = useTranslation();
  const locale = useLocale();
  const now = new Date();
  const assignments = useQuery({
    queryKey: ["admin-assignments", { classId }],
    queryFn: ({ signal }) => listAssignments({ classId, limit: 100 }, signal),
  });
  const facets = assignments.data?.facets;
  const items = [...(assignments.data?.items ?? [])]
    .sort((a, b) => ORDER[statusAt(a, now)] - ORDER[statusAt(b, now)])
    .slice(0, SHOWN);

  return (
    <section
      aria-labelledby="class-assignments-heading"
      className="bg-card shadow-card @container/assignments overflow-hidden rounded-xl border"
    >
      <div className="flex items-center justify-between gap-3 px-4.5 pt-4 pb-3">
        <div className="min-w-0">
          <h2
            id="class-assignments-heading"
            className="text-[0.9375rem] font-semibold tracking-[-0.01em]"
          >
            {t("classDetail.assignments")}
          </h2>
          <p className="text-muted-fg text-meta mt-0.5">
            {facets
              ? t("classDetail.assignmentsSummary", {
                  count: facets.all,
                  open: facets.open,
                })
              : " "}
          </p>
        </div>
        <Link
          to={`/teacher/assignments?classId=${classId}`}
          className="inline-flex items-center gap-1 text-[0.8125rem] font-medium whitespace-nowrap hover:underline"
        >
          {t("classDetail.viewAllAssignments")}
          <ArrowRight aria-hidden="true" className="size-3.5" />
        </Link>
      </div>
      <div role="table" aria-labelledby="class-assignments-heading">
        <div role="rowgroup">
          <div
            role="row"
            className={`${ROW} bg-muted text-muted-fg h-10 text-[0.78125rem] font-medium`}
          >
            <span role="columnheader">{t("assignments.test")}</span>
            <span role="columnheader">{t("assignments.statusColumn")}</span>
            <span role="columnheader" className="hidden @[560px]/assignments:block">
              {t("assignments.detail.closes")}
            </span>
            <span role="columnheader" className="text-right">
              {t("assignments.progress")}
            </span>
          </div>
        </div>
        <div role="rowgroup">
          {items.map((a) => (
            <Row key={a.id} assignment={a} now={now} locale={locale} />
          ))}
        </div>
      </div>
      {assignments.isPending ? (
        <div
          role="status"
          aria-label={t("common.loading")}
          className="border-t px-4.5 py-3"
        >
          <Skeleton className="h-4 w-2/3" />
        </div>
      ) : null}
      {assignments.isError ? (
        <div className="border-t px-4.5 py-4">
          <LoadError
            error={assignments.error}
            onRetry={() => void assignments.refetch()}
          >
            {t("classDetail.assignmentsFailed")}
          </LoadError>
        </div>
      ) : null}
      {assignments.isSuccess && items.length === 0 ? (
        <p className="text-muted-fg border-t px-4.5 py-8 text-center text-sm">
          {t("classDetail.assignmentsEmpty")}
        </p>
      ) : null}
    </section>
  );
}

function Row({
  assignment,
  now,
  locale,
}: Readonly<{ assignment: Assignment; now: Date; locale: Locale }>) {
  const { t } = useTranslation();
  const status = statusAt(assignment, now);
  return (
    <div
      role="row"
      className={`${ROW} hover:bg-muted relative min-h-12.5 border-t py-2 text-[0.84375rem]`}
    >
      <span role="cell" className="min-w-0 truncate">
        <Link
          to={`/teacher/assignments/${assignment.id}`}
          className="font-medium after:absolute after:inset-0"
        >
          {assignment.testTitle}
        </Link>
        <span className="text-muted-fg ml-2 text-xs tabular-nums">
          {t("tests.versionNumber", { n: assignment.testVersion })}
        </span>
      </span>
      <span role="cell">
        <AssignmentStatusPill assignment={assignment} now={now} />
      </span>
      <span
        role="cell"
        className="text-muted-fg hidden text-[0.8125rem] whitespace-nowrap @[560px]/assignments:block"
      >
        {formatMoment(assignment.window.closedAt ?? assignment.window.closesAt, locale)}
      </span>
      <span role="cell" className="text-right tabular-nums">
        {status === "draft" || status === "scheduled"
          ? "—"
          : t("assignments.progressValue", {
              submitted: assignment.submittedCount ?? 0,
              total: assignment.targetCount ?? 0,
            })}
      </span>
    </div>
  );
}
