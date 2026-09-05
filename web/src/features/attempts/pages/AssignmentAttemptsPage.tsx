import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useParams, useSearchParams } from "react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Ban, Eye, RotateCw } from "lucide-react";
import { EmptyState, ListSkeleton, LoadError } from "@/components/shared/ListState";
import { PageHeader } from "@/components/shared/PageHeader";
import { RowMenu } from "@/components/shared/RowMenu";
import { SearchInput } from "@/components/shared/SearchInput";
import { StatusBadge } from "@/components/shared/StatusBadge";
import { Avatar } from "@/components/ui/avatar";
import { Badge, badgeVariants } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { DropdownMenuItem, DropdownMenuSeparator } from "@/components/ui/dropdown-menu";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { getAssignment } from "@/features/assignments/api";
import { statusAt } from "@/features/assignments/status";
import { scoreText } from "@/features/assignments/studentTime";
import {
  InterventionDialog,
  type Intervention,
} from "@/features/attempts/components/InterventionDialog";
import { getMonitor, isHandedIn, type MonitorRow } from "@/features/attempts/api";
import { monitorKey } from "@/features/attempts/keys";
import { ApiError } from "@/lib/api/errors";
import { fold } from "@/lib/fold";
import { formatDateTime } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";
import { useDebounced } from "@/lib/useDebounced";
import { cn } from "@/lib/utils";

const TABS = ["all", "submitted", "pending", "flagged", "notStarted"] as const;
type Tab = (typeof TABS)[number];

/**
 * G-11: every paper of one assignment. The monitor's rows with the clock
 * columns dropped, in roster order, filtered by the results strip's numbers.
 */
export default function AssignmentAttemptsPage() {
  const { t } = useTranslation();
  const { id = "" } = useParams<{ id: string }>();
  const [params, setParams] = useSearchParams();
  const tab = readTab(params.get("tab"));
  const [query, setQuery] = useState("");
  const search = fold(useDebounced(query, 300).trim());
  const queryClient = useQueryClient();
  const [dialog, setDialog] = useState<{ kind: Intervention; row: MonitorRow } | null>(
    null,
  );

  const assignment = useQuery({
    queryKey: ["admin-assignment", id],
    queryFn: ({ signal }) => getAssignment(id, signal),
  });
  const monitor = useQuery({
    queryKey: monitorKey(id),
    queryFn: ({ signal }) => getMonitor(id, signal),
    enabled: assignment.isSuccess,
  });

  const refresh = async () => {
    await queryClient.invalidateQueries({ queryKey: monitorKey(id) });
    await queryClient.invalidateQueries({ queryKey: ["admin-assignment", id] });
    await queryClient.invalidateQueries({ queryKey: ["admin-dashboard"] });
  };

  if (assignment.isPending) return <ListSkeleton rows={8} />;
  if (assignment.isError) {
    const missing =
      assignment.error instanceof ApiError && assignment.error.status === 404;
    return missing ? (
      <EmptyState
        action={
          <Button variant="outline" size="sm" asChild>
            <Link to="/admin/assignments">{t("assignments.detail.backToList")}</Link>
          </Button>
        }
      >
        {t("assignments.detail.notFound")}
      </EmptyState>
    ) : (
      <LoadError error={assignment.error} onRetry={() => void assignment.refetch()}>
        {t("assignments.detail.loadFailed")}
      </LoadError>
    );
  }

  const a = assignment.data;
  const rows = monitor.data?.rows ?? [];
  const counts = Object.fromEntries(
    TABS.map((key) => [key, rows.filter((row) => inTab(row, key)).length]),
  ) as Record<Tab, number>;
  const shown = rows.filter(
    (row) => inTab(row, tab) && (search === "" || fold(row.fullName).includes(search)),
  );

  return (
    <>
      <PageHeader
        title={t("papers.title", { title: a.testTitle })}
        backTo={`/admin/assignments/${a.id}`}
        meta={
          <>
            <StatusBadge kind="assignment" status={statusAt(a, new Date())} />
            <span className="text-muted-foreground text-xs">
              {t("papers.meta", {
                submitted: a.submittedCount ?? 0,
                total: a.targetCount ?? 0,
                pending: a.pendingGradingCount ?? 0,
                flagged: a.flaggedCount ?? 0,
              })}
            </span>
          </>
        }
      />
      <div className="space-y-4">
        <div className="flex items-center gap-2">
          <Tabs
            value={tab}
            onValueChange={(next) =>
              setParams(
                (current) => {
                  const out = new URLSearchParams(current);
                  if (next === "all") out.delete("tab");
                  else out.set("tab", next);
                  return out;
                },
                { replace: true },
              )
            }
          >
            <TabsList aria-label={t("papers.filter")}>
              {TABS.map((key) => (
                <TabsTrigger key={key} value={key}>
                  {t(`papers.tabs.${key}`)}
                  {monitor.isSuccess ? (
                    <span className="text-muted-foreground ml-1 tabular-nums">
                      {counts[key]}
                    </span>
                  ) : null}
                </TabsTrigger>
              ))}
            </TabsList>
          </Tabs>
          <SearchInput
            className="ml-auto w-56"
            value={query}
            onChange={setQuery}
            placeholder={t("papers.search")}
          />
        </div>

        {monitor.isPending ? (
          <ListSkeleton />
        ) : monitor.isError ? (
          <LoadError error={monitor.error} onRetry={() => void monitor.refetch()}>
            {t("monitor.loadFailed")}
          </LoadError>
        ) : rows.length === 0 ? (
          <EmptyState>{t("monitor.empty")}</EmptyState>
        ) : shown.length === 0 ? (
          <EmptyState>
            {search === ""
              ? t("papers.emptyTab")
              : t("papers.noMatches", { query: query.trim() })}
          </EmptyState>
        ) : (
          <Card className="gap-0 overflow-hidden py-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-[24%]">{t("monitor.student")}</TableHead>
                  <TableHead>{t("monitor.state")}</TableHead>
                  <TableHead>{t("papers.attempt")}</TableHead>
                  <TableHead>{t("papers.submittedAt")}</TableHead>
                  <TableHead className="text-right">{t("papers.took")}</TableHead>
                  <TableHead className="text-right">{t("monitor.focusLoss")}</TableHead>
                  <TableHead className="text-right">{t("monitor.score")}</TableHead>
                  <TableHead className="w-10">
                    <span className="sr-only">{t("common.actions")}</span>
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {shown.map((row) => (
                  <Row
                    key={row.studentId}
                    row={row}
                    maxAttempts={a.maxAttempts}
                    onAct={(kind) => setDialog({ kind, row })}
                  />
                ))}
              </TableBody>
            </Table>
          </Card>
        )}
        {rows.length > 0 ? (
          <p className="text-muted-foreground text-xs">{t("papers.hint")}</p>
        ) : null}
      </div>
      <InterventionDialog
        kind={dialog?.kind ?? null}
        row={dialog?.row ?? null}
        onOpenChange={(open) => {
          if (!open) setDialog(null);
        }}
        onDone={refresh}
      />
    </>
  );
}

function readTab(value: string | null): Tab {
  return (TABS as readonly string[]).includes(value ?? "") ? (value as Tab) : "all";
}

/** "Chưa nộp" is never started; a timed-out paper is handed in, as G-09 counts it. */
function inTab(row: MonitorRow, tab: Tab): boolean {
  switch (tab) {
    case "all":
      return true;
    case "submitted":
      return isHandedIn(row.state);
    case "pending":
      return isHandedIn(row.state) && (row.score?.pendingManual ?? 0) > 0;
    case "flagged":
      return row.flagged === true && row.state !== "voided";
    case "notStarted":
      return row.state === "not_started";
  }
}

function Row({
  row,
  maxAttempts,
  onAct,
}: {
  row: MonitorRow;
  maxAttempts: number;
  onAct: (kind: Intervention) => void;
}) {
  const { t } = useTranslation();
  const locale = useLocale();
  const dash = <span className="text-muted-foreground">—</span>;
  const handedIn = isHandedIn(row.state);
  const took =
    row.startedAt && row.submittedAt
      ? Math.round(
          (new Date(row.submittedAt).getTime() - new Date(row.startedAt).getTime()) /
            60_000,
        )
      : null;

  return (
    <TableRow>
      <TableCell>
        <div className="flex items-center gap-2">
          <Avatar name={row.fullName} size="sm" />
          <span className="font-medium">{row.fullName}</span>
        </div>
      </TableCell>
      <TableCell>
        <StatusBadge kind="attempt" status={row.state} />
      </TableCell>
      <TableCell className="tabular-nums">
        {row.attemptNo == null
          ? dash
          : t("papers.attemptOf", { no: row.attemptNo, max: maxAttempts })}
      </TableCell>
      <TableCell className="text-muted-foreground tabular-nums">
        {row.submittedAt ? formatDateTime(row.submittedAt, locale) : dash}
      </TableCell>
      <TableCell className="text-right tabular-nums">
        {took === null
          ? dash
          : took < 1
            ? t("papers.tookUnderMinute")
            : t("papers.tookMinutes", { count: took })}
      </TableCell>
      <TableCell className="text-right tabular-nums">
        {row.focusLossCount == null ? (
          dash
        ) : row.flagged ? (
          <Badge variant="warning" className="tabular-nums">
            {row.focusLossCount}
          </Badge>
        ) : (
          row.focusLossCount
        )}
      </TableCell>
      <TableCell className="text-right">
        {row.score && handedIn ? (
          <span className="inline-flex items-center gap-1.5">
            <span
              className={cn("tabular-nums", row.state === "graded" && "font-medium")}
            >
              {scoreText(row.score.earned, row.score.total, locale, t)}
            </span>
            {row.score.pendingManual > 0 && row.attemptId ? (
              <Link
                to={`/admin/attempts/${row.attemptId}`}
                className={cn(badgeVariants({ variant: "outline" }), "hover:bg-accent")}
              >
                {t("monitor.pendingBadge", { count: row.score.pendingManual })}
              </Link>
            ) : null}
          </span>
        ) : (
          dash
        )}
      </TableCell>
      <TableCell className="text-right">
        {row.attemptId ? (
          <RowMenu>
            <DropdownMenuItem asChild>
              <Link to={`/admin/attempts/${row.attemptId}`}>
                <Eye aria-hidden="true" />
                {t("monitor.menu.view")}
              </Link>
            </DropdownMenuItem>
            {row.state !== "voided" ? (
              <>
                <DropdownMenuItem onSelect={() => onAct("reset")}>
                  <RotateCw aria-hidden="true" />
                  {t("monitor.menu.reset")}
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem variant="destructive" onSelect={() => onAct("void")}>
                  <Ban aria-hidden="true" />
                  {t("monitor.menu.void")}
                </DropdownMenuItem>
              </>
            ) : null}
          </RowMenu>
        ) : null}
      </TableCell>
    </TableRow>
  );
}
