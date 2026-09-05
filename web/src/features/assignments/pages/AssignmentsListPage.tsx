import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useNavigate, useSearchParams } from "react-router";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { ArrowUpRight, Flag, GraduationCap, Pencil, Plus, X } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { DropdownMenuItem } from "@/components/ui/dropdown-menu";
import { RowMenu } from "@/components/shared/RowMenu";
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
import { fetchClass } from "@/features/classes/api";
import { StatusBadge } from "@/components/shared/StatusBadge";
import type { Locale } from "@/lib/i18n";
import { useLocale } from "@/lib/i18n/useLocale";
import { formatDateTime } from "@/lib/i18n/datetime";
import { EmptyState, ListSkeleton, QueryStates } from "@/components/shared/ListState";
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
  // G-12: arriving from a class narrows the list, and the chip is the way out.
  const [params, setParams] = useSearchParams();
  const classId = params.get("classId") ?? undefined;
  const klass = useQuery({
    queryKey: ["admin-class", classId],
    queryFn: ({ signal }) => fetchClass(classId ?? "", signal),
    enabled: classId !== undefined,
  });

  const [page] = usePage(`${tab}:${classId ?? ""}`);
  const assignments = useQuery({
    queryKey: ["admin-assignments", { tab, page, classId }],
    queryFn: ({ signal }) =>
      listAssignments(
        {
          limit: PAGE_SIZE,
          page,
          ...(tab === "all" ? {} : { status: tab }),
          ...(classId === undefined ? {} : { classId }),
        },
        signal,
      ),
    placeholderData: keepPreviousData,
  });

  const items = assignments.data?.items ?? [];
  const facets = assignments.data?.facets;

  return (
    <div className="space-y-4">
      <PageHeader
        variant="title"
        title={t("nav.assignments")}
        subtitle={
          facets
            ? t("assignments.summary", { count: facets.all, open: facets.open })
            : " "
        }
        actions={
          <Button size="sm" onClick={() => void navigate("/admin/assignments/new")}>
            <Plus aria-hidden="true" />
            {t("assignments.new")}
          </Button>
        }
      />

      <div className="flex items-center gap-2">
        <Tabs
          value={tab}
          onValueChange={(next) => setTab(next as AssignmentStatus | "all")}
        >
          <TabsList aria-label={t("assignments.statusFilter")}>
            {TABS.map((value) => (
              <TabsTrigger key={value} value={value}>
                {value === "all"
                  ? t("assignments.all")
                  : t(`status.assignment.${value}`)}
                {facets ? (
                  <span className="text-muted-foreground ml-1 tabular-nums">
                    {facets[value]}
                  </span>
                ) : null}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
        {classId === undefined ? null : (
          <Badge variant="secondary" className="gap-1.5 py-0.5">
            <GraduationCap aria-hidden="true" />
            {klass.data?.name ?? t("assignments.classFilterLoading")}
            <button
              type="button"
              aria-label={t("assignments.clearClassFilter")}
              className="hover:bg-accent -mr-1 rounded-sm p-0.5"
              onClick={() =>
                setParams(
                  (current) => {
                    const out = new URLSearchParams(current);
                    out.delete("classId");
                    out.delete("page");
                    return out;
                  },
                  { replace: true },
                )
              }
            >
              <X className="size-3" aria-hidden="true" />
            </button>
          </Badge>
        )}
      </div>

      <QueryStates
        query={assignments}
        skeleton={<ListSkeleton />}
        failed={t("assignments.loadFailed")}
      >
        {(data) =>
          items.length === 0 ? (
            <EmptyState
              action={
                <Button
                  size="sm"
                  onClick={() => void navigate("/admin/assignments/new")}
                >
                  {t("assignments.new")}
                </Button>
              }
            >
              {t(emptyKey(classId !== undefined, tab))}
            </EmptyState>
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
                      <TableHead className="w-10">
                        <span className="sr-only">{t("common.actions")}</span>
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
                      />
                    ))}
                  </TableBody>
                </Table>
              </Card>

              {data && (
                <Pager page={data.page} pageSize={data.pageSize} total={data.total} />
              )}
            </>
          )
        }
      </QueryStates>
    </div>
  );
}

function Row({
  assignment,
  locale,
  now,
}: Readonly<{
  assignment: Assignment;
  locale: Locale;
  now: Date;
}>) {
  const { t } = useTranslation();
  const status = statusAt(assignment, now);
  const submitted = assignment.submittedCount ?? 0;
  const total = assignment.targetCount ?? 0;
  const flagged = assignment.flaggedCount ?? 0;
  const href = `/admin/assignments/${assignment.id}`;

  return (
    <TableRow>
      <TableCell>
        <Link to={href} className="truncate font-medium hover:underline">
          {assignment.testTitle}
        </Link>
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
      <TableCell className="text-right">
        <RowMenu>
          <DropdownMenuItem asChild>
            <Link to={href}>
              <ArrowUpRight className="text-muted-foreground" aria-hidden="true" />
              {t("assignments.rowOpen")}
            </Link>
          </DropdownMenuItem>
          {status === "draft" || status === "scheduled" ? (
            <DropdownMenuItem asChild>
              <Link to={`${href}/edit`}>
                <Pencil className="text-muted-foreground" aria-hidden="true" />
                {t("assignments.detail.edit")}
              </Link>
            </DropdownMenuItem>
          ) : null}
        </RowMenu>
      </TableCell>
    </TableRow>
  );
}

function emptyKey(forClass: boolean, tab: string): string {
  if (tab !== "all") return "assignments.noneWithStatus";
  return forClass ? "assignments.emptyForClass" : "assignments.empty";
}
