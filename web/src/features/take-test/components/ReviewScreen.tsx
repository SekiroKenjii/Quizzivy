import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ChevronLeft, CircleCheck, CircleHelp, Flag } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
import { Separator } from "@/components/ui/separator";
import { Clock } from "./Clock";
import { QuestionDots, type DotState } from "./Navigator";
import { useTakeTestStore } from "../store";

/**
 * S-06's review: what is still empty, what was flagged, and the one button
 * that ends the attempt.
 *
 * The clock keeps running up here -- this is a view of the same attempt, not
 * a pause -- and every dot is a way back to the question it names. The
 * confirm dialog says the unanswered count out loud, because that is the
 * number a student regrets not having seen.
 */
export function ReviewScreen({
  dots,
  onBack,
  onJump,
}: {
  dots: DotState[];
  onBack: () => void;
  onJump: (index: number) => void;
}) {
  const { t } = useTranslation();
  const submit = useTakeTestStore((s) => s.submit);
  const submitState = useTakeTestStore((s) => s.submitState);
  const [confirming, setConfirming] = useState(false);
  const [failed, setFailed] = useState(false);

  const answered = dots.filter((d) => d.answered).length;
  const unanswered = dots
    .map((d, i) => ({ ...d, index: i }))
    .filter((d) => !d.answered);
  const flagged = dots.map((d, i) => ({ ...d, index: i })).filter((d) => d.flagged);
  const busy = submitState === "inFlight";

  const confirm = async () => {
    setFailed(false);
    await submit("manual");
    // The store settles back to idle when the request did not land and the
    // attempt is still open; anything else is the page's to navigate away from.
    if (useTakeTestStore.getState().submitState === "idle") {
      setFailed(true);
      setConfirming(false);
    }
  };

  return (
    <div className="flex flex-1 flex-col">
      <header className="border-b">
        <div className="mx-auto flex h-12 w-full max-w-[720px] items-center gap-3 px-4">
          <Button
            variant="ghost"
            size="xs"
            className="text-muted-foreground px-1"
            onClick={onBack}
          >
            <ChevronLeft aria-hidden="true" />
            {t("takeTest.backToPaper")}
          </Button>
          <span className="ml-auto">
            <Clock />
          </span>
        </div>
      </header>

      <main className="mx-auto w-full max-w-[720px] flex-1 space-y-4 px-4 py-4">
        <div>
          <h1 className="text-lg font-semibold tracking-tight">
            {t("takeTest.reviewTitle")}
          </h1>
          <p className="text-muted-foreground mt-1 text-sm">
            {t("takeTest.reviewAnswered", { answered, total: dots.length })}
          </p>
        </div>

        <Card className="gap-0 p-4">
          <div className="space-y-3">
            {unanswered.length === 0 ? (
              <div className="flex items-center gap-2">
                <CircleCheck className="text-success size-4" aria-hidden="true" />
                <p className="text-sm font-medium">{t("takeTest.allAnswered")}</p>
              </div>
            ) : (
              <>
                <div className="flex items-center gap-2">
                  <CircleHelp
                    className="text-muted-foreground size-4"
                    aria-hidden="true"
                  />
                  <p className="text-sm font-medium">
                    {t("takeTest.unanswered", { count: unanswered.length })}
                  </p>
                </div>
                <Dots items={unanswered} onJump={onJump} />
              </>
            )}
            {flagged.length > 0 && (
              <>
                <Separator />
                <div className="flex items-center gap-2">
                  <Flag className="text-muted-foreground size-4" aria-hidden="true" />
                  <p className="text-sm font-medium">
                    {t("takeTest.flagged", { count: flagged.length })}
                  </p>
                </div>
                <Dots items={flagged} onJump={onJump} />
              </>
            )}
          </div>
        </Card>

        {failed && (
          <p role="alert" className="text-sm">
            {t("takeTest.submitFailed")}
          </p>
        )}
        <div className="space-y-2">
          <Button variant="outline" size="lg" className="w-full" onClick={onBack}>
            {t("takeTest.keepWorking")}
          </Button>
          <Button
            size="lg"
            className="w-full"
            disabled={busy}
            onClick={() => setConfirming(true)}
          >
            {busy ? t("takeTest.submitting") : t("takeTest.submit")}
          </Button>
        </div>
        <p className="text-muted-foreground text-center text-xs leading-relaxed">
          {t("takeTest.submitNote")}
        </p>
      </main>

      <Dialog open={confirming} onOpenChange={setConfirming}>
        <DialogContent className="gap-0 p-5 sm:max-w-md" showCloseButton={false}>
          <DialogTitle className="text-base leading-normal">
            {t("takeTest.confirmTitle")}
          </DialogTitle>
          <DialogDescription className="mt-2 text-sm leading-relaxed">
            {unanswered.length === 0
              ? t("takeTest.confirmAll")
              : t("takeTest.confirmUnanswered", { count: unanswered.length })}
          </DialogDescription>
          <div className="mt-5 flex gap-2">
            <Button
              variant="outline"
              className="flex-1"
              onClick={() => setConfirming(false)}
            >
              {t("takeTest.confirmBack")}
            </Button>
            <Button className="flex-1" disabled={busy} onClick={() => void confirm()}>
              {t("takeTest.submit")}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}

/** A subset of the dots, keeping their real numbers. */
function Dots({
  items,
  onJump,
}: {
  items: (DotState & { index: number })[];
  onJump: (index: number) => void;
}) {
  const { t } = useTranslation();
  return (
    <div className="flex flex-wrap gap-1.5">
      {items.map((d) => (
        <button
          key={d.id}
          type="button"
          aria-label={t("takeTest.dotLabel", { n: d.index + 1 })}
          onClick={() => onJump(d.index)}
          className="bg-background grid h-9 w-9 place-content-center rounded-md border text-xs tabular-nums"
        >
          {d.index + 1}
        </button>
      ))}
    </div>
  );
}

export { QuestionDots };
