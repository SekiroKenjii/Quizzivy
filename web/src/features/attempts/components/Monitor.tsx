import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Ban, Clock, Eye, RotateCw, UserPlus } from "lucide-react";
import { EmptyState, ListSkeleton, LoadError } from "@/components/shared/ListState";
import { RowMenu } from "@/components/shared/RowMenu";
import { StatusBadge } from "@/components/shared/StatusBadge";
import { Avatar } from "@/components/ui/avatar";
import { badgeVariants } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { DropdownMenuItem, DropdownMenuSeparator } from "@/components/ui/dropdown-menu";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { Assignment } from "@/features/assignments/api";
import { scoreText } from "@/features/assignments/studentTime";
import { useLocale } from "@/lib/i18n/useLocale";
import { countdown, formatTime } from "@/lib/i18n/datetime";
import { cn } from "@/lib/utils";
import { useTick } from "@/hooks/useTick";
import {
  getMonitor,
  isHandedIn,
  type Monitor as MonitorData,
  type MonitorRow,
} from "../api";
import { POLL_MS, monitorKey } from "../keys";
import { FocusLossCell } from "./FocusLossCell";
import { InterventionDialog, type Intervention } from "./InterventionDialog";
import type { TFunction } from "i18next";

/** Under five minutes the remaining time turns warning ink, as G-02 draws it. */
const URGENT_MS = 5 * 60_000;
const HOUR_MS = 60 * 60_000;
const DAY_MS = 24 * HOUR_MS;

/**
 * G-02: one row per targeted student, polled every 15s only while the
 * assignment is open (the query pauses itself in a hidden tab), with the three
 * interventions behind one menu.
 */
export function Monitor({
  assignment,
  live,
}: Readonly<{
  assignment: Assignment;
  /** Polls and shows a countdown while true; a closed assignment reads once. */
  live: boolean;
}>) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [dialog, setDialog] = useState<{ kind: Intervention; row: MonitorRow } | null>(
    null,
  );

  const monitor = useQuery({
    queryKey: monitorKey(assignment.id),
    queryFn: ({ signal }) => getMonitor(assignment.id, signal),
    refetchInterval: live ? POLL_MS : false,
    refetchIntervalInBackground: false,
  });

  const refresh = async () => {
    await queryClient.invalidateQueries({ queryKey: monitorKey(assignment.id) });
    await queryClient.invalidateQueries({
      queryKey: ["admin-assignment", assignment.id],
    });
    await queryClient.invalidateQueries({ queryKey: ["admin-dashboard"] });
  };

  if (monitor.isPending) return <ListSkeleton />;
  if (monitor.isError) {
    return (
      <LoadError error={monitor.error} onRetry={() => void monitor.refetch()}>
        {t("monitor.loadFailed")}
      </LoadError>
    );
  }
  const data = monitor.data;
  if (data.rows.length === 0) {
    const firstClass = assignment.targets.classes[0];
    return (
      <EmptyState
        action={
          <Button size="sm" asChild>
            <Link
              to={firstClass ? `/admin/classes/${firstClass.id}` : "/admin/classes"}
            >
              <UserPlus aria-hidden="true" />
              {t("monitor.addStudents")}
            </Link>
          </Button>
        }
      >
        {t("monitor.empty")}
      </EmptyState>
    );
  }

  return (
    <div className="space-y-4">
      <Cards
        data={data}
        assignment={assignment}
        receivedAt={monitor.dataUpdatedAt}
        live={live}
      />
      <Card className="gap-0 overflow-hidden py-0">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-[22%]">{t("monitor.student")}</TableHead>
              <TableHead>{t("monitor.state")}</TableHead>
              <TableHead>{t("monitor.startedAt")}</TableHead>
              <TableHead className="text-right">{t("monitor.remaining")}</TableHead>
              <TableHead className="w-[150px]">{t("monitor.progress")}</TableHead>
              <TableHead className="text-right">{t("monitor.focusLoss")}</TableHead>
              <TableHead className="text-right">{t("monitor.score")}</TableHead>
              <TableHead className="w-10">
                <span className="sr-only">{t("common.actions")}</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {data.rows.map((row) => (
              <Row
                key={row.studentId}
                row={row}
                questionCount={data.questionCount}
                serverTime={data.serverTime}
                receivedAt={monitor.dataUpdatedAt}
                live={live}
                onAct={(kind) => setDialog({ kind, row })}
              />
            ))}
          </TableBody>
        </Table>
      </Card>
      <InterventionDialog
        kind={dialog?.kind ?? null}
        row={dialog?.row ?? null}
        onOpenChange={(open) => {
          if (!open) setDialog(null);
        }}
        onDone={refresh}
      />
    </div>
  );
}

function Cards({
  data,
  assignment,
  receivedAt,
  live,
}: Readonly<{
  data: MonitorData;
  assignment: Assignment;
  receivedAt: number;
  live: boolean;
}>) {
  const { t } = useTranslation();
  const rows = data.rows;
  const submitted = rows.filter((r) => isHandedIn(r.state)).length;
  const inProgress = rows.filter((r) => r.state === "in_progress").length;
  const notStarted = rows.filter((r) => r.state === "not_started").length;
  const flagged = rows.filter((r) => r.flagged === true).length;
  const closesIn = useCountdown(
    assignment.window.closesAt,
    data.serverTime,
    receivedAt,
    live,
  );
  return (
    <div className="grid grid-cols-5 gap-4">
      <Stat label={t("monitor.cards.submitted")}>
        {submitted}
        <span className="text-muted-foreground text-base">/{rows.length}</span>
      </Stat>
      <Stat label={t("monitor.cards.inProgress")}>{inProgress}</Stat>
      <Stat label={t("monitor.cards.notStarted")}>{notStarted}</Stat>
      <Stat label={t("monitor.cards.flagged")}>{flagged}</Stat>
      <Stat label={t("monitor.cards.closesIn")} urgent={closesIn < URGENT_MS}>
        {closesInText(closesIn, t)}
      </Stat>
    </div>
  );
}

function Stat({
  label,
  urgent = false,
  children,
}: Readonly<{
  label: string;
  urgent?: boolean;
  children: React.ReactNode;
}>) {
  return (
    <Card>
      <CardContent>
        <p className="text-muted-foreground text-xs">{label}</p>
        <p
          className={cn(
            "mt-1 text-2xl font-semibold tabular-nums",
            urgent && "text-warning-ink",
          )}
        >
          {children}
        </p>
      </CardContent>
    </Card>
  );
}

/**
 * Milliseconds until `until` on the server's clock: the offset between
 * serverTime and the moment it arrived is applied to every later second.
 */
function useCountdown(
  until: string,
  serverTime: string,
  receivedAt: number,
  live: boolean,
): number {
  const tick = useTick(live);
  const skew = new Date(serverTime).getTime() - receivedAt;
  const now = live ? tick * 1000 : receivedAt;
  return new Date(until).getTime() - (now + skew);
}

function Row({
  row,
  questionCount,
  serverTime,
  receivedAt,
  live,
  onAct,
}: Readonly<{
  row: MonitorRow;
  questionCount: number;
  serverTime: string;
  receivedAt: number;
  live: boolean;
  onAct: (kind: Intervention) => void;
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const remaining = useCountdown(
    row.deadlineAt ?? serverTime,
    serverTime,
    receivedAt,
    live && row.state === "in_progress",
  );
  const answered = row.answeredCount ?? null;
  const handedIn = isHandedIn(row.state);
  const dash = <span className="text-muted-foreground">—</span>;

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
      <TableCell className="text-muted-foreground tabular-nums">
        {row.startedAt ? formatTime(row.startedAt) : dash}
      </TableCell>
      <TableCell
        className={cn(
          "text-right tabular-nums",
          row.state === "in_progress" &&
            remaining < URGENT_MS &&
            "text-warning-ink font-medium",
        )}
      >
        {row.state === "in_progress" ? countdown(remaining) : dash}
      </TableCell>
      <TableCell>
        {answered === null ? (
          dash
        ) : (
          <ProgressCell
            answered={answered}
            total={questionCount}
            settled={handedIn && row.state !== "timed_out"}
          />
        )}
      </TableCell>
      <TableCell className="text-right tabular-nums">
        <FocusLossCell count={row.focusLossCount} flagged={row.flagged} />
      </TableCell>
      <TableCell className="text-right">
        {row.score ? (
          <span className="inline-flex items-center gap-1.5">
            <span
              className={cn("tabular-nums", row.state === "graded" && "font-medium")}
            >
              {scoreText(row.score.earned, row.score.total, locale, t)}
            </span>
            {row.score.pendingManual > 0 && row.attemptId && (
              <Link
                to={`/admin/attempts/${row.attemptId}`}
                className={cn(badgeVariants({ variant: "outline" }), "hover:bg-accent")}
              >
                {t("monitor.pendingBadge", { count: row.score.pendingManual })}
              </Link>
            )}
          </span>
        ) : (
          dash
        )}
      </TableCell>
      <TableCell className="text-right">
        {row.attemptId && (
          <RowMenu>
            <DropdownMenuItem asChild>
              <Link to={`/admin/attempts/${row.attemptId}`}>
                <Eye aria-hidden="true" />
                {t("monitor.menu.view")}
              </Link>
            </DropdownMenuItem>
            {row.state === "in_progress" && (
              <DropdownMenuItem onSelect={() => onAct("extend")}>
                <Clock aria-hidden="true" />
                {t("monitor.menu.extend")}
              </DropdownMenuItem>
            )}
            {row.state !== "voided" && (
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
            )}
          </RowMenu>
        )}
      </TableCell>
    </TableRow>
  );
}

/** "Đóng sau": days, then hours, then the running clock, as G-02 draws it. */
function closesInText(closesIn: number, t: TFunction): string {
  if (closesIn >= DAY_MS)
    return t("monitor.closesInDays", { count: Math.floor(closesIn / DAY_MS) });
  if (closesIn >= HOUR_MS)
    return t("monitor.closesInHours", { count: Math.floor(closesIn / HOUR_MS) });
  return countdown(closesIn);
}

/** A handed-in paper shows its count; a running one draws the bar. */
function ProgressCell({
  answered,
  total,
  settled,
}: Readonly<{ answered: number; total: number; settled: boolean }>) {
  const { t } = useTranslation();
  if (settled) {
    return (
      <span className="text-muted-foreground text-xs tabular-nums">
        {answered}/{total}
      </span>
    );
  }
  const percent = total === 0 ? 0 : Math.round((answered / total) * 100);
  return (
    <div className="flex items-center gap-2">
      <span
        className="bg-secondary h-1.5 flex-1 overflow-hidden rounded-full"
        role="img"
        aria-label={t("monitor.answeredOf", { answered, total })}
      >
        <span
          className="bg-foreground block h-full rounded-full"
          style={{ width: `${percent}%` }}
        />
      </span>
      <span className="text-muted-foreground text-xs tabular-nums">
        {answered}/{total}
      </span>
    </div>
  );
}
