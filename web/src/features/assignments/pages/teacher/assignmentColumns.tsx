import type { TFunction } from "i18next";
import type { DataColumn } from "@/components/shared/data/DataTable";
import { StatusBadge } from "@/components/shared/StatusBadge";
import type { Assignment } from "@/features/assignments/api";
import { statusAt } from "@/features/assignments/status";
import { AssignmentCell, SubmittedBar } from "./AssignmentCells";
import { targetsOf, windowOf, type ListTab } from "./assignmentWindow";

/**
 * assignmentColumns is the list as the deck draws it: Assignment (its title,
 * "Test v{n} · {q} questions" and what is left to grade), then Assigned to
 * from 860px of content width, the tab's window column from 700, Submitted
 * from 560 and Status from 760. While Assigned to or the window column is
 * hidden, the Assignment cell adds "classes · when" on a third line.
 */
export function assignmentColumns(
  t: TFunction,
  tab: ListTab,
  now: Date,
): DataColumn<Assignment>[] {
  return [
    {
      id: "assignment",
      header: t("assignments.list.assignment"),
      track: "minmax(200px,2.2fr)",
      cell: (assignment, shown) => (
        <AssignmentCell
          assignment={assignment}
          inline={
            shown.has("targets") && shown.has("window")
              ? null
              : `${targetsOf(assignment, t)} · ${windowOf(assignment, tab, now, t)}`
          }
        />
      ),
    },
    {
      id: "targets",
      header: t("assignments.list.assignedTo"),
      track: "minmax(130px,1.3fr)",
      showFrom: 860,
      cell: (assignment) => (
        <span className="text-muted-fg block truncate text-[13px]">
          {targetsOf(assignment, t)}
        </span>
      ),
    },
    {
      id: "window",
      header: t(`assignments.list.window_${tab}`),
      track: "120px",
      showFrom: 700,
      cell: (assignment) => (
        <span className="text-[13px]">{windowOf(assignment, tab, now, t)}</span>
      ),
    },
    {
      id: "submitted",
      header: t("assignments.list.submitted"),
      track: "150px",
      showFrom: 560,
      cell: (assignment) => <SubmittedBar assignment={assignment} />,
    },
    {
      id: "status",
      header: t("assignments.list.status"),
      track: "110px",
      showFrom: 760,
      cell: (assignment) => (
        <StatusBadge kind="assignment" status={statusAt(assignment, now)} />
      ),
    },
  ];
}
