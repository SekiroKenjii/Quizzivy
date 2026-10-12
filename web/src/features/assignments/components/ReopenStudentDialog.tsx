import { useState, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { RotateCcw } from "lucide-react";
import { FormDialog } from "@/components/shared/form/FormDialog";
import { toast } from "@/components/ui/sonner";
import type { MonitorRow } from "@/features/attempts/api";
import { ApiError } from "@/lib/api/errors";
import { fromDateTimeInput, toDateTimeInput } from "@/lib/i18n/datetime";
import { listStudentOverrides, setStudentOverrides, type Assignment } from "../api";
import { attemptsAllowed, needsAnotherAttempt, overridesKey } from "../overrides";

const DAY_MS = 24 * 60 * 60 * 1000;
const LIMITS = ["30", "45", "60"] as const;

type Values = {
  student: string;
  minutes: string;
  untilDate: string;
  untilTime: string;
  reason: string;
};

/**
 * ReopenStudentDialog is the More menu's "Reopen for a student": one student,
 * a time limit of 30, 45 or 60 minutes, the moment it stays open until and the
 * required reason (DG-160). It writes the student's override through
 * `setStudentOverrides`. A student whose latest attempt is handed in and was
 * the last they are allowed, counting the extra attempts their override
 * already holds, gets one more on top of those (DG-163); the dialog says which
 * of the two will happen, and waits for the stored overrides before it sends.
 * Focus returns to `returnFocus` when it closes.
 */
export function ReopenStudentDialog({
  assignment,
  rows,
  open,
  returnFocus,
  onOpenChange,
  onDone,
}: Readonly<{
  assignment: Assignment;
  rows: readonly MonitorRow[];
  open: boolean;
  returnFocus?: RefObject<HTMLElement | null> | undefined;
  onOpenChange: (open: boolean) => void;
  onDone: () => Promise<void> | void;
}>) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const [error, setError] = useState<string | null>(null);
  const byId = new Map(rows.map((row) => [row.studentId, row]));
  const overrides = useQuery({
    queryKey: overridesKey(assignment.id),
    queryFn: ({ signal }) => listStudentOverrides(assignment.id, signal),
    enabled: open,
  });
  const stored = overrides.data?.items ?? [];
  const allowed = (studentId: string) =>
    attemptsAllowed(assignment.maxAttempts, stored, studentId);
  const extraFor = (studentId: string) =>
    stored.find((override) => override.studentId === studentId)?.extraAttempts ?? 0;
  const reopen = useMutation({
    mutationFn: (values: Values) => {
      const row = byId.get(values.student);
      if (row === undefined || !overrides.isSuccess) throw new Error("not ready");
      return setStudentOverrides(assignment.id, {
        studentIds: [row.studentId],
        closesAt: until(values).toISOString(),
        durationMinutes: Number(values.minutes),
        reason: values.reason.trim(),
        ...(needsAnotherAttempt(row, allowed(row.studentId))
          ? { extraAttempts: extraFor(row.studentId) + 1 }
          : {}),
      });
    },
    onSuccess: async (_saved, values) => {
      setError(null);
      onOpenChange(false);
      toast(
        t("assignmentDetail.reopenStudent.done", {
          name: byId.get(values.student)?.fullName ?? "",
        }),
      );
      await Promise.all([
        client.invalidateQueries({ queryKey: overridesKey(assignment.id) }),
        onDone(),
      ]);
    },
    onError: (cause) =>
      setError(
        cause instanceof ApiError
          ? cause.message
          : t("assignmentDetail.reopenStudent.failed"),
      ),
  });
  const limit = (minutes: string) =>
    t("assignments.minutes", { count: Number(minutes) });
  const loadError = overrides.isError
    ? t("assignmentDetail.reopenStudent.unread")
    : null;
  return (
    <FormDialog<Values>
      open={open}
      onOpenChange={(next) => {
        if (!next) setError(null);
        if (!reopen.isPending) onOpenChange(next);
      }}
      icon={RotateCcw}
      title={t("assignmentDetail.reopenStudent.title")}
      description={(values) => {
        const row = byId.get(values.student);
        if (row === undefined || !overrides.isSuccess)
          return t("assignmentDetail.reopenStudent.description");
        return t(
          needsAnotherAttempt(row, allowed(row.studentId))
            ? "assignmentDetail.reopenStudent.oneMore"
            : "assignmentDetail.reopenStudent.canContinue",
          { name: row.fullName },
        );
      }}
      initial={{
        student: rows[0]?.studentId ?? "",
        minutes: "60",
        untilDate: tomorrow(),
        untilTime: "21:00",
        reason: "",
      }}
      fields={[
        {
          kind: "select",
          name: "student",
          label: t("assignmentDetail.reopenStudent.student"),
          options: rows.map((row) => ({ value: row.studentId, label: row.fullName })),
          required: true,
        },
        {
          kind: "seg",
          name: "minutes",
          label: t("assignments.wizard.timeLimit"),
          options: LIMITS.map((minutes) => ({ value: minutes, label: limit(minutes) })),
        },
        {
          kind: "date",
          name: "untilDate",
          label: t("assignmentDetail.reopenStudent.until"),
          noOptional: true,
        },
        {
          kind: "time",
          name: "untilTime",
          label: t("assignmentDetail.reopenStudent.untilTime"),
          noOptional: true,
        },
        {
          kind: "area",
          name: "reason",
          label: t("assignmentDetail.reason"),
          hint: t("assignments.detail.reopenReasonHint"),
          required: true,
          maxLength: 500,
        },
      ]}
      validate={(values) =>
        values.untilDate === "" ||
        values.untilTime === "" ||
        until(values).getTime() <= Date.now()
          ? { untilDate: t("assignmentDetail.reopenStudent.untilPast") }
          : null
      }
      submitLabel={t("assignmentDetail.reopenStudent.submit")}
      pending={reopen.isPending}
      disabled={!overrides.isSuccess}
      error={error ?? loadError}
      returnFocus={returnFocus}
      onSubmit={(values) => reopen.mutate(values)}
    />
  );
}

function until(values: Pick<Values, "untilDate" | "untilTime">): Date {
  return fromDateTimeInput(`${values.untilDate}T${values.untilTime}`);
}

function tomorrow(): string {
  return toDateTimeInput(new Date(Date.now() + DAY_MS)).slice(0, 10);
}
