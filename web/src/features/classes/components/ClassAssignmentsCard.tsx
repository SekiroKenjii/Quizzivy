import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { ArrowRight, ClipboardList } from "lucide-react";
import { EmptyState, ListSkeleton, LoadError } from "@/components/shared/ListState";
import { StatusBadge } from "@/components/shared/StatusBadge";
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
import { listAssignments, type Assignment } from "@/features/assignments/api";
import { statusAt } from "@/features/assignments/status";
import type { Locale } from "@/lib/i18n";
import { formatMoment } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";

const SHOWN = 5;
const ORDER = { open: 0, scheduled: 1, draft: 2, closed: 3 } as const;

/** G-06's second list: what the class has been given, open ones first. */
export function ClassAssignmentsCard({ classId }: { classId: string }) {
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
  const listHref = `/admin/assignments?classId=${classId}`;

  return (
    <Card asChild className="gap-0 py-0">
      <section aria-labelledby="class-assignments-heading">
        <div className="flex items-center justify-between gap-3 px-5 pt-4 pb-3">
          <div>
            <h2
              id="class-assignments-heading"
              className="text-[0.9375rem] font-semibold tracking-[-0.01em]"
            >
              {t("classDetail.assignments")}
            </h2>
            <p className="text-muted-foreground text-xs">
              {facets
                ? t("classDetail.assignmentsSummary", {
                    count: facets.all,
                    open: facets.open,
                  })
                : " "}
            </p>
          </div>
          <Button variant="link" size="sm" asChild>
            <Link to={listHref}>
              {t("classDetail.viewAllAssignments")}
              <ArrowRight aria-hidden="true" />
            </Link>
          </Button>
        </div>
        {assignments.isPending ? (
          <div className="px-5 pb-5">
            <ListSkeleton rows={3} />
          </div>
        ) : assignments.isError ? (
          <div className="px-5 pb-5">
            <LoadError
              error={assignments.error}
              onRetry={() => void assignments.refetch()}
            >
              {t("classDetail.assignmentsFailed")}
            </LoadError>
          </div>
        ) : items.length === 0 ? (
          <div className="px-5 pb-5">
            <EmptyState
              action={
                <Button variant="outline" size="sm" asChild>
                  <Link to={`/admin/assignments/new?classId=${classId}`}>
                    <ClipboardList aria-hidden="true" />
                    {t("classDetail.assignToClass")}
                  </Link>
                </Button>
              }
            >
              {t("classDetail.assignmentsEmpty")}
            </EmptyState>
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-[46%]">{t("assignments.test")}</TableHead>
                <TableHead>{t("assignments.statusColumn")}</TableHead>
                <TableHead>{t("assignments.detail.closes")}</TableHead>
                <TableHead className="text-right">
                  {t("assignments.progress")}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((a) => (
                <Row key={a.id} assignment={a} now={now} locale={locale} />
              ))}
            </TableBody>
          </Table>
        )}
      </section>
    </Card>
  );
}

function Row({
  assignment,
  now,
  locale,
}: {
  assignment: Assignment;
  now: Date;
  locale: Locale;
}) {
  const { t } = useTranslation();
  const status = statusAt(assignment, now);
  const submitted = assignment.submittedCount ?? 0;
  const total = assignment.targetCount ?? 0;
  return (
    <TableRow>
      <TableCell>
        <Link
          to={`/admin/assignments/${assignment.id}`}
          className="truncate font-medium hover:underline"
        >
          {assignment.testTitle}
        </Link>
        <span className="text-muted-foreground ml-2 text-xs tabular-nums">
          {t("tests.versionNumber", { n: assignment.testVersion })}
        </span>
      </TableCell>
      <TableCell>
        <StatusBadge kind="assignment" status={status} />
      </TableCell>
      <TableCell className="text-muted-foreground tabular-nums">
        {formatMoment(assignment.window.closedAt ?? assignment.window.closesAt, locale)}
      </TableCell>
      <TableCell className="text-right tabular-nums">
        {status === "draft" || status === "scheduled" ? (
          <span className="text-muted-foreground">—</span>
        ) : (
          t("assignments.progressValue", { submitted, total })
        )}
      </TableCell>
    </TableRow>
  );
}
