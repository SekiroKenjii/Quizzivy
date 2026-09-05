import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { ArrowRight, X } from "lucide-react";
import { ListSkeleton, QueryStates } from "@/components/shared/ListState";
import { PageAside } from "@/components/shared/PageAside";
import { Avatar } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import type { Assignment } from "@/features/assignments/api";
import { scoreText } from "@/features/assignments/studentTime";
import { getMonitor, isHandedIn, type MonitorRow } from "@/features/attempts/api";
import { monitorKey } from "@/features/attempts/keys";
import { useLocale } from "@/lib/i18n/useLocale";

type Group = "pending" | "flagged" | "inProgress" | "notStarted" | "done" | "voided";
const ORDER: Group[] = [
  "pending",
  "flagged",
  "inProgress",
  "notStarted",
  "done",
  "voided",
];

/**
 * G-09's "Mở bảng học viên": every targeted student beside the closed page,
 * grouped by what the teacher still has to do. A paper that is both unmarked
 * and flagged sits in both groups -- each is a separate piece of work.
 */
export function StudentPanel({
  assignment,
  open,
  onOpenChange,
}: Readonly<{
  assignment: Assignment;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}>) {
  const { t } = useTranslation();
  const monitor = useQuery({
    queryKey: monitorKey(assignment.id),
    queryFn: ({ signal }) => getMonitor(assignment.id, signal),
  });
  const rows = monitor.data?.rows ?? [];
  const submitted = rows.filter((r) => isHandedIn(r.state)).length;

  return (
    <PageAside label={t("assignments.panel.title")} sheet={{ open, onOpenChange }}>
      <div className="flex items-start gap-3">
        <div className="min-w-0 flex-1">
          <p className="text-base font-semibold">{t("assignments.panel.title")}</p>
          <p className="text-muted-foreground text-xs">
            {t("assignments.panel.summary", { total: rows.length, submitted })}
          </p>
        </div>
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={t("assignments.panel.close")}
          onClick={() => onOpenChange(false)}
        >
          <X aria-hidden="true" />
        </Button>
      </div>
      <QueryStates
        query={monitor}
        skeleton={<ListSkeleton rows={6} />}
        failed={t("monitor.loadFailed")}
      >
        {() => (
          <div className="space-y-4">
            {ORDER.map((group) => {
              const members = rows.filter((row) => inGroup(row, group));
              if (members.length === 0) return null;
              return (
                <section key={group} aria-labelledby={`panel-${group}`}>
                  <p
                    id={`panel-${group}`}
                    className="text-muted-foreground mb-1 text-xs font-medium tracking-wide uppercase"
                  >
                    {t(`assignments.panel.groups.${group}`)} · {members.length}
                  </p>
                  <ul>
                    {members.map((row) => (
                      <li key={row.studentId}>
                        <PanelRow row={row} group={group} />
                      </li>
                    ))}
                  </ul>
                </section>
              );
            })}
          </div>
        )}
      </QueryStates>
      <Button variant="outline" className="w-full" asChild>
        <Link to={`/admin/assignments/${assignment.id}/attempts`}>
          {t("assignments.panel.viewAll")}
          <ArrowRight aria-hidden="true" />
        </Link>
      </Button>
    </PageAside>
  );
}

function inGroup(row: MonitorRow, group: Group): boolean {
  const pending = (row.score?.pendingManual ?? 0) > 0;
  switch (group) {
    case "pending":
      return isHandedIn(row.state) && pending;
    case "flagged":
      return row.flagged === true && row.state !== "voided";
    case "inProgress":
      return row.state === "in_progress";
    case "notStarted":
      return row.state === "not_started";
    case "done":
      return isHandedIn(row.state) && !pending;
    case "voided":
      return row.state === "voided";
  }
}

function PanelRow({ row, group }: Readonly<{ row: MonitorRow; group: Group }>) {
  const body = (
    <>
      <Avatar name={row.fullName} size="sm" />
      <span className="flex-1 truncate text-sm font-medium">{row.fullName}</span>
      <PanelValue row={row} group={group} />
    </>
  );
  const className = "flex items-center gap-2 rounded-md py-1.5";
  return row.attemptId ? (
    <Link
      to={`/admin/attempts/${row.attemptId}`}
      className={`${className} hover:bg-accent -mx-2 px-2`}
    >
      {body}
    </Link>
  ) : (
    <div className={className}>{body}</div>
  );
}

/** The row's right edge: the strike count, the score with what is unmarked, or nothing yet. */
function PanelValue({ row, group }: Readonly<{ row: MonitorRow; group: Group }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  if (group === "flagged") {
    return (
      <Badge variant="warning" className="tabular-nums">
        {row.focusLossCount ?? 0}
      </Badge>
    );
  }
  if (row.score && isHandedIn(row.state)) {
    return (
      <>
        <span className="text-xs tabular-nums">
          {scoreText(row.score.earned, row.score.total, locale, t)}
        </span>
        {row.score.pendingManual > 0 ? (
          <Badge variant="outline">
            {t("assignments.panel.pending", { count: row.score.pendingManual })}
          </Badge>
        ) : null}
      </>
    );
  }
  return <span className="text-muted-foreground text-xs">—</span>;
}
