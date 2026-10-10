import type { TFunction } from "i18next";
import type { Assignment, AssignmentStatus } from "@/features/assignments/api";
import { compactMoment, formatTime, sameAppDay } from "@/lib/i18n/datetime";

/** ListTab is a tab of the assignments list: the server's status it lists. */
export type ListTab = AssignmentStatus;

const DAY = 24 * 60 * 60 * 1000;

/** moment is a window time as the deck writes it: "Today, 21:00", "Tomorrow, 08:00", else "08:00 · 26/09". */
export function moment(utc: string, now: Date, t: TFunction): string {
  if (sameAppDay(utc, now))
    return t("assignments.list.today", { time: formatTime(utc) });
  if (sameAppDay(utc, new Date(now.getTime() + DAY)))
    return t("assignments.list.tomorrow", { time: formatTime(utc) });
  return compactMoment(utc);
}

/**
 * windowOf is the moment the window column shows on `tab`: the close of a live
 * assignment, the opening of a scheduled one, the actual close of a closed one
 * (an early close included) and the planned opening of a draft.
 */
export function windowOf(
  assignment: Assignment,
  tab: ListTab,
  now: Date,
  t: TFunction,
) {
  const { opensAt, closesAt, closedAt } = assignment.window;
  if (tab === "scheduled" || tab === "draft") return moment(opensAt, now, t);
  if (tab === "closed") return moment(closedAt ?? closesAt, now, t);
  return moment(closesAt, now, t);
}

/** targetsOf names an assignment's classes, then the students assigned by name, or says nobody is assigned. */
export function targetsOf(assignment: Assignment, t: TFunction): string {
  const { classes, students } = assignment.targets;
  const parts = classes.map((klass) => klass.name);
  if (students.length > 0)
    parts.push(t("assignments.list.students", { count: students.length }));
  return parts.length === 0 ? t("assignments.list.notAssigned") : parts.join(", ");
}
