import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation } from "@tanstack/react-query";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { toast } from "@/components/ui/sonner";
import { reopenAssignment, type Assignment } from "@/features/assignments/api";
import { ApiError } from "@/lib/api/errors";
import { formatMoment, fromDateTimeInput, toDateTimeInput } from "@/lib/i18n/datetime";
import { cn } from "@/lib/utils";

type Choice = "today" | "day" | "threeDays" | "pick";
const DAY_MS = 24 * 60 * 60 * 1000;

/**
 * G-09's "Gia hạn cho tất cả": three quick moments or a picked one, then the
 * reason the audit row keeps. Nothing here is red -- reopening is the
 * reversible side of closing.
 */
export function ReopenDialog({
  assignment,
  open,
  onOpenChange,
  onDone,
}: {
  assignment: Assignment;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onDone: () => Promise<void> | void;
}) {
  const { t } = useTranslation();
  const [choice, setChoice] = useState<Choice>("day");
  const [picked, setPicked] = useState(() =>
    toDateTimeInput(new Date(Date.now() + DAY_MS)),
  );
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string | null>(null);
  // Mounted fresh on every opening, so "now" is the moment the teacher opened it.
  const [now] = useState(() => Date.now());
  const closesAt = momentFor(choice, picked, now);
  const todayPossible = momentFor("today", picked, now) > now;

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

  const choices: { key: Choice; label: string; disabled?: boolean }[] = [
    {
      key: "today",
      label: t("assignments.detail.reopenUntilToday"),
      disabled: !todayPossible,
    },
    { key: "day", label: t("assignments.detail.reopenPlusDay") },
    { key: "threeDays", label: t("assignments.detail.reopenPlusThreeDays") },
    { key: "pick", label: t("assignments.detail.reopenPick") },
  ];

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
        <div
          className="flex flex-wrap gap-1.5"
          role="radiogroup"
          aria-label={t("assignments.detail.closes")}
        >
          {choices.map((c) => (
            <button
              key={c.key}
              type="button"
              role="radio"
              aria-checked={choice === c.key}
              disabled={c.disabled}
              className={cn(
                "rounded-md border px-2.5 py-1 text-sm disabled:opacity-50",
                choice === c.key ? "bg-foreground text-background" : "hover:bg-accent",
              )}
              onClick={() => setChoice(c.key)}
            >
              {c.label}
            </button>
          ))}
        </div>
        {choice === "pick" ? (
          <Input
            type="datetime-local"
            aria-label={t("assignments.detail.closes")}
            value={picked}
            onChange={(event) => setPicked(event.target.value)}
          />
        ) : null}
        <p className="text-muted-foreground text-xs">
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

function momentFor(choice: Choice, picked: string, now: number): number {
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
