import { useState } from "react";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { EyeOff } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
import type { StrikeState } from "../strikes";

const FRAME =
  "bg-card shadow-float flex max-h-[calc(100%-2rem)] w-[min(26.25rem,calc(100%-1.5rem))] max-w-none flex-col items-start gap-3 overflow-y-auto rounded-2xl p-5.5 sm:max-w-none";

function sentence(state: StrikeState, t: TFunction): string {
  const { count: n, limit, consequence } = state;
  if (limit === null) return t("integrity.left.recorded");
  if (state.exceeded && consequence === "auto_submit") {
    return t("integrity.autoSubmitBody");
  }
  if (limit === 0) {
    return t(
      consequence === "flag" ? "integrity.left.noneFlag" : "integrity.left.noneWarn",
    );
  }
  if (state.exceeded) {
    return t(
      consequence === "flag" ? "integrity.left.overFlag" : "integrity.left.overWarn",
      { n, limit },
    );
  }
  if (consequence === "auto_submit") {
    return t(
      state.remaining === 0
        ? "integrity.left.lastSubmit"
        : "integrity.left.countedSubmit",
      { n, count: limit },
    );
  }
  return t("integrity.left.counted", { n, count: limit });
}

/**
 * StrikeDialog is the deck's "You left the test" alert, shown when the student
 * comes back from a counted absence: the eye-off icon on a warning tile, one
 * body that says where the student stands, and "Back to the test". Within the
 * allowance it reads "That counts as n of m times allowed"; past it under
 * `flag`, "Your teacher has been told. Your answers are safe." The states the
 * deck does not draw use the same frame and say only what the server does: a
 * `warn` policy never names the teacher, an assignment that allows no absence
 * states no number, an `auto_submit` policy says the test is submitted next,
 * and with no limit it says the absence is recorded, once a sitting. It opens
 * again on each counted absence, keyed to this sitting's `strikes`. It cannot
 * be closed from the backdrop and has no close button, so it is not waved
 * away unread, but Escape acknowledges it as the button does: the student is
 * never trapped.
 */
export function StrikeDialog({
  state,
  strikes,
}: Readonly<{
  state: StrikeState;
  strikes: number;
}>) {
  const { t } = useTranslation();
  const [acknowledged, setAcknowledged] = useState(0);

  const unseen = strikes > acknowledged;
  const open = unseen && (state.limit !== null || acknowledged === 0);

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) setAcknowledged(strikes);
      }}
    >
      <DialogContent
        role="alertdialog"
        className={FRAME}
        showCloseButton={false}
        onPointerDownOutside={(event) => event.preventDefault()}
        onInteractOutside={(event) => event.preventDefault()}
      >
        <span
          aria-hidden="true"
          className="bg-warning-soft text-warning-ink grid size-10 flex-none place-items-center rounded-lg"
        >
          <EyeOff className="size-5" />
        </span>
        <DialogTitle className="text-lg leading-normal font-semibold">
          {t("integrity.left.title")}
        </DialogTitle>
        <DialogDescription className="text-muted-fg text-base leading-[1.55] text-pretty">
          {sentence(state, t)}
        </DialogDescription>
        <Button
          size="xl"
          className="self-stretch"
          onClick={() => setAcknowledged(strikes)}
        >
          {t("integrity.left.back")}
        </Button>
      </DialogContent>
    </Dialog>
  );
}
