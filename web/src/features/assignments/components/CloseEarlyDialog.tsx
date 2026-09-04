import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Timer } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
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
}: {
  assignment: Assignment;
  open: boolean;
  pending: boolean;
  failed: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
}) {
  const { t } = useTranslation();
  const [understood, setUnderstood] = useState(false);

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) setUnderstood(false);
        onOpenChange(next);
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("assignments.detail.closeNowTitle")}</DialogTitle>
          <DialogDescription>
            {t("assignments.detail.closeNowBody", {
              now: formatTime(new Date()),
              planned: formatMoment(assignment.window.closesAt),
            })}
          </DialogDescription>
        </DialogHeader>
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
        {failed && (
          <p role="alert" className="text-destructive text-sm">
            {t("assignments.detail.closeFailed")}
          </p>
        )}
        <DialogFooter>
          <Button
            variant="outline"
            className="flex-1"
            onClick={() => onOpenChange(false)}
          >
            {t("common.cancel")}
          </Button>
          <Button
            className="flex-1"
            disabled={!understood || pending}
            onClick={onConfirm}
          >
            {t("assignments.detail.closeNow")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
