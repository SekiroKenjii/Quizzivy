import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation } from "@tanstack/react-query";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { DateTimeField } from "@/components/shared/DateTimeField";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { toast } from "@/components/ui/sonner";
import { reopenAssignment, type Assignment } from "@/features/assignments/api";
import type { ReopenChoice } from "@/features/assignments/components/ReopenMenu";
import { ApiError } from "@/lib/api/errors";
import { formatMoment, fromDateTimeInput, toDateTimeInput } from "@/lib/i18n/datetime";

const DAY_MS = 24 * 60 * 60 * 1000;

/**
 * The second step of G-09's "Gia hạn cho tất cả": the moment the menu chose
 * (or a picker, for "Chọn thời điểm…") and the reason the audit row keeps.
 * Nothing here is red -- reopening is the reversible side of closing.
 */
export function ReopenDialog({
  assignment,
  choice,
  open,
  onOpenChange,
  onDone,
}: Readonly<{
  assignment: Assignment;
  choice: ReopenChoice;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onDone: () => Promise<void> | void;
}>) {
  const { t } = useTranslation();
  const [picked, setPicked] = useState(() =>
    toDateTimeInput(new Date(Date.now() + DAY_MS)),
  );
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string | null>(null);
  // Mounted fresh on every opening, so "now" is the moment the teacher chose.
  const [now] = useState(() => Date.now());
  const closesAt = reopenMoment(choice, picked, now);

  const reopen = useMutation({
    mutationFn: () =>
      reopenAssignment(assignment.id, {
        closesAt: new Date(closesAt).toISOString(),
        reason: reason.trim(),
      }),
    onSuccess: async () => {
      setReason("");
      setError(null);
      onOpenChange(false);
      toast(t("assignments.detail.reopened"));
      await onDone();
    },
    onError: (cause) =>
      setError(
        cause instanceof ApiError
          ? cause.message
          : t("assignments.detail.reopenFailed"),
      ),
  });

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={(next) => {
        if (!next) setError(null);
        onOpenChange(next);
      }}
      title={t("assignments.detail.reopenTitle", {
        count: assignment.targetCount ?? 0,
      })}
      description={t("assignments.detail.reopenNote")}
      confirmLabel={t("assignments.detail.reopenConfirm")}
      disabled={reason.trim() === "" || closesAt <= now}
      pending={reopen.isPending}
      error={error}
      onConfirm={() => reopen.mutate()}
    >
      <div className="space-y-3">
        {choice === "pick" ? (
          <div>
            <Label htmlFor="reopen-until">{t("assignments.detail.closes")}</Label>
            <DateTimeField
              id="reopen-until"
              label={t("assignments.detail.closes")}
              className="mt-1.5"
              value={picked}
              onChange={setPicked}
            />
          </div>
        ) : null}
        <p className="text-sm">
          {t("assignments.detail.closesAt", {
            when: formatMoment(new Date(closesAt).toISOString()),
          })}
        </p>
        <div>
          <Label htmlFor="reopen-reason">{t("assignments.detail.reopenReason")}</Label>
          <Textarea
            id="reopen-reason"
            className="mt-1.5 min-h-16"
            value={reason}
            onChange={(event) => setReason(event.target.value)}
          />
          <p className="text-muted-foreground mt-1 text-xs">
            {t("assignments.detail.reopenReasonHint")}
          </p>
        </div>
      </div>
    </ConfirmDialog>
  );
}

/** The instant each menu entry stands for, on the clock of the moment it was opened. */
function reopenMoment(choice: ReopenChoice, picked: string, now: number): number {
  switch (choice) {
    case "today": {
      const at = new Date(now);
      at.setHours(21, 0, 0, 0);
      return at.getTime();
    }
    case "day":
      return now + DAY_MS;
    case "threeDays":
      return now + 3 * DAY_MS;
    case "pick":
      return fromDateTimeInput(picked).getTime();
  }
}
