import type { TFunction } from "i18next";
import type { DataColumn } from "@/components/shared/data/DataTable";
import type { Assignment } from "@/features/assignments/api";
import {
  AssignmentCell,
  AssignmentStatusPill,
  SubmittedBar,
  ToGradeCount,
} from "./AssignmentCells";
import { targetsOf, windowOf, type ListTab } from "./assignmentWindow";

/**
 * assignmentColumns is the list as the approved page-width design draws it
 * (DG-205), by the content width: Assignment (its title, "Test v{n} · {q}
 * questions" and, while To grade is hidden, what is left to grade), Assigned
 * to from 1240px, the tab's window column always, Submitted from 760, To
 * grade from 900, and Status from 760. While Assigned to is hidden, the
 * Assignment cell names the classes on a third line.
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
          toGrade={!shown.has("toGrade")}
          inline={shown.has("targets") ? null : targetsOf(assignment, t)}
        />
      ),
    },
    {
      id: "targets",
      header: t("assignments.list.assignedTo"),
      track: "minmax(130px,1.2fr)",
      showFrom: 1240,
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
      cell: (assignment) => (
        <span className="text-[13px]">{windowOf(assignment, tab, now, t)}</span>
      ),
    },
    {
      id: "submitted",
      header: t("assignments.list.submitted"),
      track: "minmax(140px,1fr)",
      showFrom: 760,
      cell: (assignment) => <SubmittedBar assignment={assignment} />,
    },
    {
      id: "toGrade",
      header: t("assignments.detail.pending"),
      track: "80px",
      showFrom: 900,
      cell: (assignment) => <ToGradeCount assignment={assignment} />,
    },
    {
      id: "status",
      header: t("assignments.list.status"),
      track: "110px",
      showFrom: 760,
      cell: (assignment) => <AssignmentStatusPill assignment={assignment} now={now} />,
    },
  ];
}
