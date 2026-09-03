import { useState } from "react";
import { Trans, useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
import type { StrikeState } from "../strikes";

/**
 * S-07's first dialog: what happened, what is left, what happens at zero.
 *
 * It opens on every counted episode while there is a limit, because each one
 * moves the consequence closer and the deck's reason for these dialogs is "to
 * make a consequence known while it can still be avoided". With no limit
 * there is nothing new to say after the first, so it opens once a sitting.
 *
 * Non-dismissible in the deck's sense: no close button and the scrim does not
 * close it, so it cannot be waved away unread. Escape still acknowledges it --
 * T-3.14 forbids swallowing that key, and a student pressing it is a student
 * who has seen the text. The timer runs underneath the whole time, which the
 * dialog says out loud.
 */
export function StrikeDialog({
  state,
  strikes,
  lastAwayMs,
}: {
  state: StrikeState;
  /** This sitting's count, which is what a new dialog is keyed to. */
  strikes: number;
  lastAwayMs: number | null;
}) {
  const { t } = useTranslation();
  const [acknowledged, setAcknowledged] = useState(0);

  const unseen = strikes > acknowledged;
  const open = unseen && (state.limit !== null || acknowledged === 0);
  const seconds = Math.max(1, Math.round((lastAwayMs ?? 0) / 1000));

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) setAcknowledged(strikes);
      }}
    >
      <DialogContent
        className="gap-0 p-5 sm:max-w-md"
        showCloseButton={false}
        onPointerDownOutside={(event) => event.preventDefault()}
        onInteractOutside={(event) => event.preventDefault()}
      >
        <DialogTitle className="text-base leading-normal">
          {t("integrity.leftTitle")}
        </DialogTitle>
        <DialogDescription asChild>
          <div className="mt-2 space-y-2 text-sm leading-relaxed">
            <p>{t("integrity.leftBody", { seconds })}</p>
            <p>
              <Consequence state={state} />
            </p>
            <p>{t("integrity.timerRunning")}</p>
            {state.exceeded && <p className="text-xs">{t("integrity.honestLimits")}</p>}
          </div>
        </DialogDescription>
        <Button className="mt-5 w-full" onClick={() => setAcknowledged(strikes)}>
          {t("integrity.continue")}
        </Button>
      </DialogContent>
    </Dialog>
  );
}

/** The one sentence that changes between the dialog's states. */
function Consequence({ state }: { state: StrikeState }) {
  const { t } = useTranslation();
  const flag = state.consequence === "flag";

  if (state.limit === null) return t("integrity.unlimited");
  if (state.exceeded) {
    return t(flag ? "integrity.exceededFlag" : "integrity.exceededWarn");
  }
  if (state.remaining === 0) {
    return t(flag ? "integrity.spentFlag" : "integrity.spentWarn");
  }
  return (
    <Trans
      i18nKey={flag ? "integrity.remainingFlag" : "integrity.remainingWarn"}
      values={{ count: state.remaining }}
      components={{ strong: <strong className="text-foreground" /> }}
    />
  );
}
