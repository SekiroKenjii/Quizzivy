import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { Flag, Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  listAssignments,
  type Assignment,
  type AssignmentStatus,
} from "@/features/assignments/api";
import { statusAt } from "@/features/assignments/status";
import { StatusBadge } from "@/components/shared/StatusBadge";
import type { Locale } from "@/lib/i18n";
import { useLocale } from "@/lib/i18n/useLocale";
import { formatDateTime } from "@/lib/i18n/datetime";
import { PageHeader } from "@/components/shared/PageHeader";
import { Pager } from "@/components/shared/Pager";
import { usePage } from "@/hooks/usePage";

const TABS: (AssignmentStatus | "all")[] = [
  "all",
  "draft",
  "open",
  "scheduled",
  "closed",
];

/**
 * §8's assignments list.
 *
 * The deck has no board for this route -- it goes straight from G-01 to the
 * monitor -- so the columns are §8's list verbatim and the shape is A-03's,
 * which is the deck's answer for every other list screen.
 */
const PAGE_SIZE = 20;

export default function AssignmentsListPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [tab, setTab] = useState<AssignmentStatus | "all">("all");
  const locale = useLocale();
  const now = new Date();

  const [page] = usePage(tab);
  const assignments = useQuery({
    queryKey: ["admin-assignments", { tab, page }],
    queryFn: ({ signal }) =>
      listAssignments(
        { limit: PAGE_SIZE, page, ...(tab === "all" ? {} : { status: tab }) },
        signal,
      ),
    placeholderData: keepPreviousData,
  });

  const items = assignments.data?.items ?? [];

  return (
    <div className="space-y-4">
      <PageHeader
        variant="title"
        title={t("nav.assignments")}
        subtitle={
          assignments.isSuccess
            ? t("assignments.summary", { count: items.length })
            : " "
        }
        actions={
          <Button size="sm" onClick={() => void navigate("/admin/assignments/new")}>
            <Plus aria-hidden="true" />
            {t("assignments.new")}
          </Button>
        }
      />

      <Tabs
        value={tab}
        onValueChange={(next) => setTab(next as AssignmentStatus | "all")}
      >
        <TabsList aria-label={t("assignments.statusFilter")}>
          {TABS.map((value) => (
            <TabsTrigger key={value} value={value}>
              {value === "all" ? t("assignments.all") : t(`status.assignment.${value}`)}
            </TabsTrigger>
          ))}
        </TabsList>
      </Tabs>

      {assignments.isPending ? (
        <p role="status" aria-live="polite" className="text-muted-foreground text-sm">
          {t("common.loading")}
        </p>
      ) : assignments.isError ? (
        <div className="space-y-3">
          <p role="alert" className="text-sm">
            {t("assignments.loadFailed")}
          </p>
          <Button
            variant="outline"
            size="sm"
            onClick={() => void assignments.refetch()}
          >
            {t("common.retry")}
          </Button>
        </div>
      ) : items.length === 0 ? (
        <div className="space-y-3">
          <p className="text-muted-foreground text-sm">
            {tab === "all" ? t("assignments.empty") : t("assignments.noneWithStatus")}
          </p>
          <Button size="sm" onClick={() => void navigate("/admin/assignments/new")}>
            {t("assignments.new")}
          </Button>
        </div>
      ) : (
        <>
          <Card className="gap-0 overflow-hidden py-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-[34%]">{t("assignments.test")}</TableHead>
                  <TableHead>{t("assignments.targets")}</TableHead>
                  <TableHead>{t("assignments.window")}</TableHead>
                  <TableHead>{t("assignments.statusColumn")}</TableHead>
                  <TableHead className="text-right">
                    {t("assignments.progress")}
                  </TableHead>
                  <TableHead className="text-right">
                    {t("assignments.flagged")}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map((assignment) => (
                  <Row
                    key={assignment.id}
                    assignment={assignment}
                    locale={locale}
                    now={now}
                    onOpen={() => void navigate(`/admin/assignments/${assignment.id}`)}
                  />
                ))}
              </TableBody>
            </Table>
          </Card>

          {assignments.data && (
            <Pager
              page={assignments.data.page}
              pageSize={assignments.data.pageSize}
              total={assignments.data.total}
            />
          )}
        </>
      )}
    </div>
  );
}

function Row({
  assignment,
  locale,
  now,
  onOpen,
}: {
  assignment: Assignment;
  locale: Locale;
  now: Date;
  onOpen: () => void;
}) {
  const { t } = useTranslation();
  const status = statusAt(assignment, now);
  const submitted = assignment.submittedCount ?? 0;
  const total = assignment.targetCount ?? 0;
  const flagged = assignment.flaggedCount ?? 0;

  return (
    <TableRow>
      <TableCell>
        <button
          type="button"
          className="truncate text-left font-medium"
          onClick={onOpen}
        >
          {assignment.testTitle}
        </button>
        <span className="text-muted-foreground ml-2 text-xs tabular-nums">
          {t("tests.versionNumber", { n: assignment.testVersion })}
        </span>
      </TableCell>
      <TableCell className="text-muted-foreground">
        {t("assignments.targetSummary", {
          classes: assignment.targets.classes.length,
          students: assignment.targets.students.length,
        })}
      </TableCell>
      <TableCell className="text-muted-foreground text-xs">
        {t("assignments.windowValue", {
          opens: formatDateTime(assignment.window.opensAt, locale),
          closes: formatDateTime(assignment.window.closesAt, locale),
        })}
      </TableCell>
      <TableCell>
        <StatusBadge kind="assignment" status={status} />
      </TableCell>
      <TableCell className="text-right tabular-nums">
        {t("assignments.progressValue", { submitted, total })}
      </TableCell>
      <TableCell className="text-right">
        {flagged === 0 ? (
          <span className="text-muted-foreground">—</span>
        ) : (
          <span className="text-warning-ink inline-flex items-center gap-1 tabular-nums">
            <Flag className="size-3.5" aria-hidden="true" />
            {flagged}
          </span>
        )}
      </TableCell>
    </TableRow>
  );
}
