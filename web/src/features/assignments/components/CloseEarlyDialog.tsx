import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Timer } from "lucide-react";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { Checkbox } from "@/components/ui/checkbox";
import type { Assignment } from "@/features/assignments/api";
import { formatMoment, formatTime } from "@/lib/i18n/datetime";

/** G-09's "Đóng sớm" confirm: restates S-04's promise and asks for one tick. */
export function CloseEarlyDialog({
  assignment,
  open,
  pending,
  failed,
  onOpenChange,
  onConfirm,
}: Readonly<{
  assignment: Assignment;
  open: boolean;
  pending: boolean;
  failed: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
}>) {
  const { t } = useTranslation();
  const [understood, setUnderstood] = useState(false);

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={(next) => {
        if (!next) setUnderstood(false);
        onOpenChange(next);
      }}
      title={t("assignments.detail.closeNowTitle")}
      description={t("assignments.detail.closeNowBody", {
        now: formatTime(new Date()),
        planned: formatMoment(assignment.window.closesAt),
      })}
      confirmLabel={t("assignments.detail.closeNow")}
      disabled={!understood}
      pending={pending}
      error={failed ? t("assignments.detail.closeFailed") : null}
      onConfirm={onConfirm}
    >
      <div className="bg-muted/40 flex items-start gap-2 rounded-md p-2.5">
        <Timer
          className="text-muted-foreground mt-0.5 size-4 shrink-0"
          aria-hidden="true"
        />
        <p className="text-xs leading-relaxed">
          {t("assignments.detail.closeNowNote", {
            minutes: assignment.durationMinutes,
          })}
        </p>
      </div>
      <label className="flex items-start gap-2 text-sm">
        <Checkbox
          className="mt-0.5"
          checked={understood}
          onChange={(event) => setUnderstood(event.target.checked)}
        />
        {t("assignments.detail.closeNowAck")}
      </label>
    </ConfirmDialog>
  );
}
