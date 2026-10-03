import { useTranslation } from "react-i18next";
import { LoaderCircle } from "lucide-react";
import {
  DeckDialog,
  DeckDialogActions,
  DeckDialogCancel,
} from "@/components/shared/DeckDialog";
import { Button } from "@/components/ui/button";
import type { Leave } from "../useLeave";

/**
 * LeaveDialog asks "Leave the test?" in the deck's dialog, with Stay and
 * Leave. While an answer is unsaved it says the answer will be saved first
 * rather than that it is saved, and Leave shows the save in progress. When
 * that save fails it says the answers are not saved and offers Stay and "Try
 * saving again"; it never offers to leave without them. Esc and the backdrop
 * mean Stay.
 */
export function LeaveDialog({ leave }: Readonly<{ leave: Leave }>) {
  const { t } = useTranslation();
  const { phase, pending } = leave;
  const failed = phase === "failed";
  const saving = phase === "saving";
  const body = pending ? "takeTest.leavePending" : "takeTest.leaveBody";

  return (
    <DeckDialog
      open={phase !== "idle"}
      onOpenChange={(open) => {
        if (!open) leave.stay();
      }}
      title={t(failed ? "takeTest.leaveUnsavedTitle" : "takeTest.leaveTitle")}
      description={t(failed ? "takeTest.leaveUnsavedDescription" : body)}
    >
      <DeckDialogActions>
        <DeckDialogCancel onClick={leave.stay}>
          {t("takeTest.leaveStay")}
        </DeckDialogCancel>
        <Button
          size="lg"
          aria-busy={saving || undefined}
          aria-disabled={saving || undefined}
          onClick={() => {
            if (!saving) leave.confirm();
          }}
        >
          {saving && (
            <LoaderCircle aria-hidden="true" className="size-[17px] animate-spin" />
          )}
          {t(failed ? "takeTest.retrySave" : "takeTest.leaveConfirm")}
        </Button>
      </DeckDialogActions>
    </DeckDialog>
  );
}
