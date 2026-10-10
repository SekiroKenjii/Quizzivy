import type { TFunction } from "i18next";
import {
  CircleCheck,
  ClipboardList,
  Clock,
  FileCheck,
  Flag,
  RotateCw,
  SquarePen,
  UserPlus,
  type LucideIcon,
} from "lucide-react";
import { FLAGGED } from "@/features/integrity/tones";
import { formatDateTime } from "@/lib/i18n/datetime";
import type { Locale } from "@/lib/i18n";
import type {
  Notification,
  NotificationEvent,
  NotificationKind,
  NotificationParams,
  NotificationTarget,
} from "./api";

/** NotificationTone names the semantic soft and ink colours a reader draws. */
export type NotificationTone = "neutral" | "info" | "success" | "warning" | "danger";
/** NotificationView supplies plain text, a look and a safe optional local destination. */
export type NotificationView = Readonly<{
  icon: LucideIcon;
  tone: NotificationTone;
  text: string;
  to: string | null;
}>;
/** KIND_LOOK is the exhaustive shared look per kind, with submitted papers awaiting grading as its default. */
export const KIND_LOOK: Record<
  NotificationKind,
  Readonly<{ icon: LucideIcon; tone: NotificationTone }>
> = {
  "attempt.submitted": { icon: SquarePen, tone: "warning" },
  "attempt.flagged": { icon: Flag, tone: FLAGGED.tone },
  "assignment.closing": { icon: Clock, tone: "warning" },
  "class.joined": { icon: UserPlus, tone: "neutral" },
  "join_codes.rotated": { icon: RotateCw, tone: "info" },
  "assignment.opened": { icon: ClipboardList, tone: "info" },
  "assignment.due_soon": { icon: Clock, tone: "warning" },
  "assignment.extended": { icon: Clock, tone: "info" },
  "result.ready": { icon: CircleCheck, tone: "success" },
};
/** TEACHER_EVENTS lists the teacher's switches in contract order. */
export const TEACHER_EVENTS = [
  "attempt.submitted",
  "attempt.flagged",
  "assignment.closing",
] as const satisfies readonly NotificationEvent[];
/** STUDENT_EVENTS lists the student's switches in contract order. */
export const STUDENT_EVENTS = [
  "assignment.due_soon",
  "result.ready",
] as const satisfies readonly NotificationEvent[];

const teacherKinds = new Set<NotificationKind>([
  "attempt.submitted",
  "attempt.flagged",
  "assignment.closing",
  "class.joined",
  "join_codes.rotated",
]);
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
function idPath(prefix: string, id: string | undefined, suffix = "") {
  return id && uuid.test(id) ? `${prefix}/${id}${suffix}` : null;
}

/** audienceOf names the console that reads a supported kind. */
export function audienceOf(kind: NotificationKind): "teacher" | "student" {
  return teacherKinds.has(kind) ? "teacher" : "student";
}

/** notificationPath resolves only known local routes with the contract's UUID identifiers. */
export function notificationPath(target: NotificationTarget | null): string | null {
  if (!target) return null;
  switch (target.route) {
    case "assignment":
      return idPath("/teacher/assignments", target.assignmentId);
    case "attempt":
      return idPath("/teacher/attempts", target.attemptId);
    case "grading":
      return "/teacher/grading";
    case "classes":
      return "/teacher/classes";
    case "result":
      return idPath("/app/attempts", target.attemptId, "/result");
    case "studentAssignment":
      return idPath("/app/assignments", target.assignmentId);
    default:
      return null;
  }
}

function hasText(value: unknown): value is string {
  return typeof value === "string" && value.length > 0;
}
function hasCount(value: unknown): value is number {
  return typeof value === "number" && Number.isInteger(value) && value >= 0;
}
function literalSentence(
  key: string,
  params: Readonly<Record<string, string | number>>,
  t: TFunction,
) {
  const values: string[] = [];
  const opaque: Record<string, string | number> = Object.fromEntries(
    Object.entries(params).map(([name, value]) => {
      if (typeof value !== "string") return [name, value];
      const token = `\uE000quizzivy-notification:${values.length}\uE001`;
      values.push(value);
      return [name, token];
    }),
  );
  return t(key, { ...opaque, returnObjects: false, returnDetails: false }).replace(
    /\uE000quizzivy-notification:(\d+)\uE001/g,
    (token, index: string) => values[Number(index)] ?? token,
  );
}
function timedSentence(
  key: string,
  params: NotificationParams,
  t: TFunction,
  locale: Locale,
) {
  if (
    !hasText(params.title) ||
    !hasText(params.closesAt) ||
    !Number.isFinite(new Date(params.closesAt).getTime())
  )
    return null;
  return literalSentence(
    key,
    { title: params.title, when: formatDateTime(params.closesAt, locale) },
    t,
  );
}
const sentences: Record<
  NotificationKind,
  (params: NotificationParams, t: TFunction, locale: Locale) => string | null
> = {
  "attempt.submitted": (p, t) => {
    if (!hasText(p.title) || !hasCount(p.toGrade)) return null;
    if (p.toGrade > 0)
      return literalSentence(
        "notifications.kind.toGrade",
        { title: p.title, count: p.toGrade },
        t,
      );
    return hasCount(p.count)
      ? literalSentence(
          "notifications.kind.submitted",
          { title: p.title, count: p.count },
          t,
        )
      : null;
  },
  "attempt.flagged": (p, t) =>
    hasText(p.studentName) && hasCount(p.focusLost)
      ? literalSentence(
          "notifications.kind.flagged",
          {
            studentName: p.studentName,
            count: p.focusLost,
          },
          t,
        )
      : null,
  "assignment.closing": (p, t) =>
    hasText(p.title) && hasCount(p.notSubmitted)
      ? literalSentence(
          "notifications.kind.closing",
          { title: p.title, count: p.notSubmitted },
          t,
        )
      : null,
  "class.joined": (p, t) =>
    hasText(p.studentName) && hasText(p.className)
      ? literalSentence(
          "notifications.kind.joined",
          {
            studentName: p.studentName,
            className: p.className,
          },
          t,
        )
      : null,
  "join_codes.rotated": (p, t) => {
    if (
      !hasCount(p.count) ||
      !Array.isArray(p.classNames) ||
      !p.classNames.every(hasText)
    )
      return null;
    const names = p.classNames.join(", ") + (p.count > p.classNames.length ? "…" : "");
    return literalSentence(
      "notifications.kind.codesRotated",
      { count: p.count, names },
      t,
    );
  },
  "assignment.opened": (p, t, locale) =>
    timedSentence("notifications.kind.opened", p, t, locale),
  "assignment.due_soon": (p, t, locale) =>
    timedSentence("notifications.kind.dueSoon", p, t, locale),
  "assignment.extended": (p, t, locale) =>
    timedSentence("notifications.kind.extended", p, t, locale),
  "result.ready": (p, t) =>
    hasText(p.title)
      ? literalSentence("notifications.kind.resultReady", { title: p.title }, t)
      : null,
};

/** describeNotification returns a complete localised row or null for an unknown kind or missing sentence input, preserving grading fallback for readers who cannot grade. */
export function describeNotification(
  notification: Notification,
  t: TFunction,
  locale: Locale,
  options?: Readonly<{ canGrade?: boolean }>,
): NotificationView | null {
  if (!Object.hasOwn(KIND_LOOK, notification.kind)) return null;
  const text = sentences[notification.kind](notification.params, t, locale);
  if (text === null) return null;
  const submitted = notification.kind === "attempt.submitted";
  const waiting = submitted && (notification.params.toGrade ?? 0) > 0;
  const look =
    submitted && !waiting
      ? { icon: FileCheck, tone: "neutral" as const }
      : KIND_LOOK[notification.kind];
  const to =
    waiting && options?.canGrade !== false
      ? "/teacher/grading"
      : notificationPath(notification.target);
  return { ...look, text, to };
}
