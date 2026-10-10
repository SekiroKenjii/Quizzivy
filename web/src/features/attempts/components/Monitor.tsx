import { useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useLocation, useNavigate } from "react-router";
import { Ban, Clock, RotateCw } from "lucide-react";
import {
  DataTable,
  type DataColumn,
  type DataTableMenuContext,
} from "@/components/shared/data/DataTable";
import { Pager } from "@/components/shared/data/Pager";
import { EmptyState } from "@/components/shared/ListState";
import { SearchInput } from "@/components/shared/SearchInput";
import { StatusBadge } from "@/components/shared/StatusBadge";
import { Avatar } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { DropdownMenuItem, DropdownMenuSeparator } from "@/components/ui/dropdown-menu";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { Assignment } from "@/features/assignments/api";
import {
  elapsedMinutes,
  orderedRoster,
  pendingAnswers,
  rosterFilter,
  rosterMatches,
} from "@/features/assignments/pages/teacher/assignmentDetail";
import { assignmentDetailLocation } from "@/features/assignments/pages/teacher/assignmentDetailUrl";
import { scoreText } from "@/features/assignments/studentTime";
import { useCan } from "@/features/auth/permissions";
import { FLAGGED } from "@/features/integrity/tones";
import { usePage, usePageSize } from "@/hooks/usePage";
import { compactMoment, useDisplayTimeZone } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";
import { pageRange } from "@/lib/pagination";
import { cn } from "@/lib/utils";
import type { Monitor as MonitorData, MonitorRow } from "../api";
import { FocusLossCell } from "./FocusLossCell";
import { InterventionDialog, type Intervention } from "./InterventionDialog";

type StudentRow = MonitorRow & { id: string };
const FILTERS = ["all", "submitted", "pending", "flagged", "notStarted"] as const;

function RowStatus({
  row,
  compact = false,
}: Readonly<{ row: MonitorRow; compact?: boolean }>) {
  const { t } = useTranslation();
  return pendingAnswers(row) > 0 ? (
    <Badge variant="warning" className={compact ? "block min-w-0 truncate" : undefined}>
      {t("assignmentDetail.needsGrading")}
    </Badge>
  ) : (
    <StatusBadge
      kind="attempt"
      status={row.state}
      className={compact ? "block min-w-0 truncate" : undefined}
    />
  );
}

/** Monitor renders the full assignment read as a filtered, paginated roster with authorized sheet and intervention actions. */
export function Monitor({
  assignment,
  data,
  selectedAttempt,
  onOpen,
  onRefresh,
}: Readonly<{
  assignment: Assignment;
  data: MonitorData;
  selectedAttempt: string | null;
  onOpen: (attemptId: string) => void;
  onRefresh: () => Promise<void>;
}>) {
  const zone = useDisplayTimeZone();
  const { t } = useTranslation();
  const locale = useLocale();
  const location = useLocation();
  const navigate = useNavigate();
  const params = new URLSearchParams(location.search);
  const query = params.get("q") ?? "";
  const filter = rosterFilter(params.get("roster"));
  const grade = useCan("teaching.grading");
  const intervene = useCan("teaching.attempts.intervene");
  const canRead = grade || intervene;
  const [dialog, setDialog] = useState<{ kind: Intervention; row: MonitorRow } | null>(
    null,
  );
  const returnFocus = useRef<HTMLElement | null>(null);
  const [page] = usePage(`${filter}:${query}`, true);
  const [size] = usePageSize(undefined, true);
  const rows = useMemo(
    () =>
      orderedRoster(data.rows)
        .filter((row) => rosterMatches(row, filter, query))
        .map((row) => ({ ...row, id: row.studentId })),
    [data.rows, filter, query],
  );
  const range = pageRange(page, size, rows.length);
  const shown = rows.slice((range.page - 1) * size, range.page * size);
  const columns = useMemo<readonly DataColumn<StudentRow>[]>(
    () => [
      {
        id: "student",
        header: t("monitor.student"),
        track: "minmax(180px,2fr)",
        cell: (row, visible) => (
          <span className="flex min-w-0 items-center gap-2.5">
            <Avatar name={row.fullName} className="text-2xs size-7.5" />
            <span className="min-w-0">
              <span className="block truncate font-medium">{row.fullName}</span>
              {(!visible.has("status") || (!visible.has("focus") && row.flagged)) && (
                <span className="flex min-w-0 items-center gap-1 text-xs leading-4">
                  {!visible.has("status") && <RowStatus row={row} compact />}
                  {row.flagged && !visible.has("focus") && (
                    <span
                      role="img"
                      className="inline-flex shrink-0 whitespace-nowrap"
                      aria-label={`${t("status.attention.flagged")} · ${row.focusLossCount == null ? "—" : t("assignmentDetail.focusCount", { count: row.focusLossCount })}`}
                    >
                      <span aria-hidden="true" className="inline-flex">
                        <FocusLossCell
                          count={row.focusLossCount}
                          flagged={row.flagged}
                          compact
                        />
                      </span>
                    </span>
                  )}
                </span>
              )}
            </span>
          </span>
        ),
      },
      {
        id: "status",
        header: t("monitor.state"),
        track: "130px",
        showFrom: 560,
        cell: (row) => <RowStatus row={row} />,
      },
      {
        id: "score",
        header: t("monitor.score"),
        track: "90px",
        cell: (row) => (
          <span
            className={cn(
              "tabular-nums",
              row.score === null || row.score === undefined
                ? "text-muted-fg"
                : "font-medium",
            )}
          >
            {row.score == null
              ? "—"
              : scoreText(row.score.earned, row.score.total, locale, t)}
            {pendingAnswers(row) > 0 && (
              <span
                aria-label={t("monitor.pendingBadge", { count: pendingAnswers(row) })}
              >
                {t("assignmentDetail.pendingMark")}
              </span>
            )}
          </span>
        ),
      },
      {
        id: "time",
        header: t("assignmentDetail.time"),
        track: "80px",
        showFrom: 820,
        cell: (row) => {
          const spent = elapsedMinutes(row);
          let spentLabel = "—";
          if (spent !== null) {
            spentLabel =
              spent < 1
                ? t("papers.tookUnderMinute")
                : t("papers.tookMinutes", { count: spent });
          }
          return <span className="text-muted-fg tabular-nums">{spentLabel}</span>;
        },
      },
      {
        id: "focus",
        header: t("assignmentDetail.focusLost"),
        track: "100px",
        showFrom: 700,
        cell: (row) => (
          <FocusLossCell count={row.focusLossCount} flagged={row.flagged} />
        ),
      },
      {
        id: "submitted",
        header: t("assignmentDetail.submitted"),
        track: "100px",
        showFrom: 940,
        cell: (row) => (
          <span className="text-muted-fg tabular-nums">
            {row.submittedAt ? compactMoment(row.submittedAt, zone) : "—"}
          </span>
        ),
      },
    ],
    [t, locale, zone],
  );
  const change = (key: string, value: string) =>
    navigate(assignmentDetailLocation(location, { [key]: value, page: null }), {
      replace: true,
    });
  const act = (kind: Intervention, row: MonitorRow, context: DataTableMenuContext) => {
    if (
      intervene &&
      row.attemptId &&
      row.state !== "voided" &&
      (kind !== "extend" || row.state === "in_progress")
    ) {
      returnFocus.current = context.triggerRef.current;
      setDialog({ kind, row });
    }
  };
  if (data.rows.length === 0)
    return (
      <EmptyState
        action={
          <Button variant="outline" asChild>
            <Link
              to={
                assignment.targets.classes[0]
                  ? `/teacher/classes/${assignment.targets.classes[0].id}`
                  : "/teacher/classes"
              }
            >
              {t("monitor.addStudents")}
            </Link>
          </Button>
        }
      >
        {t("assignmentDetail.noStudents")}
      </EmptyState>
    );
  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <SearchInput
          dense
          value={query}
          onChange={(value) => {
            void change("q", value);
          }}
          placeholder={t("papers.search")}
          className="min-w-0 flex-[1_1_180px]"
        />
        <Select
          value={filter}
          onValueChange={(value) => {
            void change("roster", value);
          }}
        >
          <SelectTrigger aria-label={t("papers.filter")} className="w-auto">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {FILTERS.map((value) => (
              <SelectItem key={value} value={value}>
                {t(`papers.tabs.${value}`)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <DataTable
        label={t("assignmentDetail.roster")}
        columns={columns}
        rows={shown}
        rowSize={{ height: 54 }}
        canOpen={(row) =>
          canRead && Boolean(row.attemptId) && row.state !== "not_started"
        }
        onOpen={(row) => {
          if (canRead && row.attemptId && row.state !== "not_started")
            onOpen(row.attemptId);
        }}
        rowTone={(row) => rowTone(row, selectedAttempt)}
        menu={
          intervene
            ? (row, context) =>
                !row.attemptId || row.state === "not_started" ? null : (
                  <>
                    {row.attemptId && (
                      <DropdownMenuItem
                        onSelect={() => {
                          if (canRead) onOpen(row.attemptId!);
                        }}
                      >
                        {t("monitor.menu.view")}
                      </DropdownMenuItem>
                    )}
                    {row.attemptId && row.state === "in_progress" && (
                      <DropdownMenuItem onSelect={() => act("extend", row, context)}>
                        <Clock aria-hidden="true" />
                        {t("monitor.menu.extend")}
                      </DropdownMenuItem>
                    )}
                    {row.attemptId && row.state !== "voided" && (
                      <>
                        <DropdownMenuItem onSelect={() => act("reset", row, context)}>
                          <RotateCw aria-hidden="true" />
                          {t("monitor.menu.reset")}
                        </DropdownMenuItem>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem
                          variant="destructive"
                          onSelect={() => act("void", row, context)}
                        >
                          <Ban aria-hidden="true" />
                          {t("monitor.menu.void")}
                        </DropdownMenuItem>
                      </>
                    )}
                  </>
                )
            : undefined
        }
        empty={t("papers.noMatches", { query })}
        footer={
          <Pager
            preserveHash
            page={range.page}
            pageSize={size}
            total={rows.length}
            noun={(count) => t("assignmentDetail.studentsNoun", { count })}
          />
        }
      />
      {intervene && (
        <InterventionDialog
          returnFocus={returnFocus}
          kind={dialog?.kind ?? null}
          row={dialog?.row ?? null}
          onOpenChange={(open) => {
            if (!open) setDialog(null);
          }}
          onDone={onRefresh}
        />
      )}
    </div>
  );
}

function rowTone(row: MonitorRow, selectedAttempt: string | null) {
  if (selectedAttempt !== null && row.attemptId === selectedAttempt) return "selected";
  return row.flagged ? FLAGGED.tone : undefined;
}
