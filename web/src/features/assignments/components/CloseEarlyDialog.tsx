import { useState, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import { Timer } from "lucide-react";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { Checkbox } from "@/components/ui/checkbox";
import type { Assignment } from "@/features/assignments/api";
import { formatMoment, formatTime, useDisplayTimeZone } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";

/**
 * CloseEarlyDialog is "Close early": the deck's title and button, and the
 * true promise that students mid-test keep their time (DG-161), in the
 * reader's language, with one tick to confirm. `keepTime`, given where the
 * roster is known, counts the students whose own extension reaches past now;
 * they keep it, and none draws no sentence. Focus returns to `returnFocus` when it closes.
 */
export function CloseEarlyDialog({
  assignment,
  open,
  pending,
  failed,
  keepTime = 0,
  returnFocus,
  onOpenChange,
  onConfirm,
}: Readonly<{
  assignment: Assignment;
  open: boolean;
  pending: boolean;
  failed: boolean;
  keepTime?: number | undefined;
  returnFocus?: RefObject<HTMLElement | null> | undefined;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
}>) {
  useDisplayTimeZone();
  const { t } = useTranslation();
  const locale = useLocale();
  const [understood, setUnderstood] = useState(false);

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={(next) => {
        if (!next) setUnderstood(false);
        onOpenChange(next);
      }}
      title={t("assignments.detail.closeNowTitle", { title: assignment.testTitle })}
      description={t("assignments.detail.closeNowBody", {
        now: formatTime(new Date()),
        planned: formatMoment(assignment.window.closesAt, locale),
      })}
      confirmLabel={t("assignments.detail.closeNow")}
      disabled={!understood}
      pending={pending}
      {...(returnFocus === undefined ? {} : { returnFocus })}
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
          {keepTime > 0 &&
            ` ${t("assignments.detail.closeNowKeepTime", { count: keepTime })}`}
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
