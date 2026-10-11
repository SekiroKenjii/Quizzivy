import { useState, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import { Clock } from "lucide-react";
import { FormDialog } from "@/components/shared/form/FormDialog";
import { toast } from "@/components/ui/sonner";
import {
  extendAssignment,
  reopenAssignment,
  setStudentOverrides,
  type Assignment,
} from "@/features/assignments/api";
import { statusAt } from "@/features/assignments/status";
import type { MonitorRow } from "@/features/attempts/api";
import { ApiError } from "@/lib/api/errors";
import { formatMoment } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";
import { nameOf } from "./assignmentWindow";

const STEPS = ["15", "30", "60", "1440"] as const;
const MAX_CHOSEN = 100;
const MINUTE_MS = 60 * 1000;

type Failure = { id: string; title: string; message: string };

type Values = {
  by: string;
  notify: boolean;
  who: string;
  students: readonly string[];
  reason: string;
};

type Chosen = { studentIds: readonly string[]; reason: string };

/**
 * ExtendAssignmentsDialog is "Extend deadline" for one assignment or for the
 * bulk bar's selection: Extend by 15 min, 30 min, 1 hour or 1 day (30 min for
 * one, 1 hour for several, as the deck's two forms default), and whether to
 * notify the students. It extends one assignment at a time; when some fail it
 * stays open on those alone, names each with the server's reason, and offers
 * to retry them, and its title counts those. `onExtended` hears the ids
 * that were extended. Focus returns to `returnFocus` when it closes.
 *
 * Given an `audience`, the roster of a single assignment, it also asks who the
 * extension is for: everyone moves the assignment's close, and chosen students
 * get their own later close through a student override, with the required
 * reason (DG-160). The caller gives an audience only to a teacher who may
 * intervene in attempts.
 *
 * A single closed assignment is extended from now: everyone reopens it
 * through `reopenAssignment`, which takes the reason and notifies nobody, and
 * chosen students get an override that closes then (DG-174).
 */
export function ExtendAssignmentsDialog({
  items,
  when,
  audience,
  open,
  returnFocus,
  onOpenChange,
  onExtended,
}: Readonly<{
  items: readonly Assignment[];
  when?: string | undefined;
  audience?: readonly MonitorRow[] | undefined;
  open: boolean;
  returnFocus?: RefObject<HTMLElement | null> | undefined;
  onOpenChange: (open: boolean) => void;
  onExtended: (ids: string[]) => void;
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const [pending, setPending] = useState(false);
  const [failures, setFailures] = useState<Failure[]>([]);
  const [opened, setOpened] = useState(open);
  if (opened !== open) {
    setOpened(open);
    if (open) setFailures([]);
  }
  const failedIds = new Set(failures.map((failure) => failure.id));
  const remaining =
    failures.length > 0 ? items.filter((item) => failedIds.has(item.id)) : items;
  const label = (minutes: string) => t(`assignments.list.by${minutes}`);
  const single = items.length === 1 ? items[0] : undefined;
  const roster = single === undefined ? undefined : audience;
  const closed = single !== undefined && statusAt(single, new Date()) === "closed";
  const chosenOf = (values: Values): Chosen | null =>
    roster !== undefined && values.who === "chosen"
      ? { studentIds: values.students, reason: values.reason.trim() }
      : null;
  const reopenUntil = (minutes: string) =>
    new Date(Date.now() + Number(minutes) * MINUTE_MS);

  function send(item: Assignment, values: Values, chosen: Chosen | null) {
    const minutes = Number(values.by);
    if (closed) {
      const closesAt = reopenUntil(values.by).toISOString();
      if (chosen === null)
        return reopenAssignment(item.id, { closesAt, reason: values.reason.trim() });
      return setStudentOverrides(item.id, {
        studentIds: [...chosen.studentIds],
        closesAt,
        reason: chosen.reason,
        notify: values.notify,
      });
    }
    if (chosen === null) return extendAssignment(item.id, minutes, values.notify);
    return setStudentOverrides(item.id, {
      studentIds: [...chosen.studentIds],
      extendBy: minutes,
      reason: chosen.reason,
      notify: values.notify,
    });
  }

  function announce(done: number, minutes: string, chosen: Chosen | null) {
    if (closed) {
      const when = formatMoment(reopenUntil(minutes).toISOString(), locale);
      if (chosen === null) toast(t("assignments.list.reopened", { when }));
      else
        toast(
          t("assignments.list.reopenedFor", { count: chosen.studentIds.length, when }),
        );
    } else if (chosen !== null)
      toast(
        t("assignments.list.extendedFor", {
          count: chosen.studentIds.length,
          by: label(minutes),
        }),
      );
    else if (done === 1) toast(t("assignments.list.extended", { by: label(minutes) }));
    else toast(t("assignments.list.extendedMany", { count: done, by: label(minutes) }));
  }

  async function extend(values: Values) {
    setPending(true);
    const chosen = chosenOf(values);
    const done: string[] = [];
    const failed: Failure[] = [];
    const now = new Date();
    for (const item of remaining) {
      try {
        await send(item, values, chosen);
        done.push(item.id);
      } catch (cause) {
        failed.push({
          id: item.id,
          title: nameOf(item, now, t),
          message: cause instanceof ApiError ? cause.message : t("common.actionFailed"),
        });
      }
    }
    setPending(false);
    setFailures(failed);
    if (done.length > 0) {
      onExtended(done);
      announce(done.length, values.by, chosen);
    }
    if (failed.length === 0) onOpenChange(false);
  }

  const chosenOnly = (values: Values) => chosenOf(values) !== null;
  const needsReason = (values: Values) => closed || chosenOnly(values);
  const notifies = (values: Values) => !closed || chosenOnly(values);
  return (
    <FormDialog<Values>
      open={open}
      onOpenChange={(next) => {
        if (!pending) onOpenChange(next);
      }}
      icon={Clock}
      title={
        remaining.length === 1
          ? t("assignments.list.extendTitle")
          : t("assignments.list.extendTitleMany", { count: remaining.length })
      }
      description={(values) => {
        if (single === undefined || when === undefined) return undefined;
        if (!closed)
          return t("assignments.list.extendWhen", { title: single.testTitle, when });
        return t("assignments.list.reopenWhen", {
          title: single.testTitle,
          when,
          until: formatMoment(reopenUntil(values.by).toISOString(), locale),
        });
      }}
      initial={{
        by: items.length > 1 ? "60" : "30",
        notify: true,
        who: "everyone",
        students: [],
        reason: "",
      }}
      fields={[
        {
          kind: "seg",
          name: "by",
          label: t("assignments.list.extendBy"),
          options: STEPS.map((minutes) => ({ value: minutes, label: label(minutes) })),
        },
        {
          kind: "seg",
          name: "who",
          label: t("assignments.list.extendFor"),
          when: () => roster !== undefined,
          options: [
            { value: "everyone", label: t("assignments.list.extendEveryone") },
            { value: "chosen", label: t("assignments.list.extendChosen") },
          ],
        },
        {
          kind: "list",
          name: "students",
          label: t("assignments.list.extendStudents"),
          when: chosenOnly,
          required: true,
          requiredText: t("assignments.list.extendNoStudents"),
          options: (roster ?? []).map((row) => ({
            value: row.studentId,
            label: row.fullName,
            ...(row.state === "not_started" || row.state === "in_progress"
              ? { meta: t(`status.attempt.${row.state}`) }
              : {}),
          })),
        },
        {
          kind: "toggle",
          name: "notify",
          when: notifies,
          text: t("assignments.list.notify"),
          sub: t("assignments.list.notifySub"),
        },
        {
          kind: "area",
          name: "reason",
          label: t("assignmentDetail.reason"),
          hint: t("assignments.detail.reopenReasonHint"),
          when: needsReason,
          required: true,
          maxLength: 500,
        },
      ]}
      validate={(values) =>
        chosenOnly(values) && values.students.length > MAX_CHOSEN
          ? { students: t("assignments.list.extendTooMany", { count: MAX_CHOSEN }) }
          : null
      }
      submitLabel={
        failures.length > 0
          ? t("common.retryFailed", { count: failures.length })
          : t("assignments.list.extendSubmit")
      }
      pending={pending}
      returnFocus={returnFocus}
      onSubmit={(values) => void extend(values)}
    >
      {failures.length > 0 ? (
        <div role="alert" className="text-danger-ink flex flex-col gap-1 text-sm">
          <p>{t("assignments.list.failedItems", { count: failures.length })}</p>
          <ul className="flex flex-col gap-1">
            {failures.map((failure) => (
              <li key={failure.id}>
                {failure.title}: {failure.message}
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </FormDialog>
  );
}
