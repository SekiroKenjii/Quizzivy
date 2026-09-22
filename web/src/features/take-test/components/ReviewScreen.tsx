import { useState, type ReactNode } from "react";
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
import { cn } from "@/lib/utils";
import { SaveStrip } from "./SaveState";
import { EngineHeader } from "./EngineHeader";
import { NavigatorRail, QuestionDots, type DotState } from "./Navigator";
import { useTakeTestStore } from "../store";
import type { SectionGroup } from "../sections";

/**
 * S-06's review: what is still empty, what was flagged, and the one button
 * that ends the attempt. From 1024px it is S-15: the same header row as the
 * paper, the rail beside it without its button, and the two actions side by
 * side at their own width.
 */
export function ReviewScreen({
  wide,
  dots,
  groups,
  status,
  onBack,
  onJump,
}: Readonly<{
  wide: boolean;
  dots: DotState[];
  groups: SectionGroup[];
  status: ReactNode;
  onBack: () => void;
  onJump: (index: number) => void;
}>) {
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
  const lock = useTakeTestStore((s) => s.lock);
  const remainingAttempts = useTakeTestStore((s) => s.remainingAttempts);

  const confirm = async () => {
    setFailed(false);
    await submit("manual");
    if (useTakeTestStore.getState().submitState === "idle") {
      setFailed(true);
      setConfirming(false);
    }
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <EngineHeader
        wide={wide}
        progress={1}
        status={status}
        leading={
          <Button
            variant="ghost"
            size="xs"
            className="text-muted-foreground h-11 px-1 lg:h-7"
            onClick={onBack}
          >
            <ChevronLeft aria-hidden="true" />
            {t("takeTest.backToPaper")}
          </Button>
        }
      />

      <SaveStrip wide={wide} indicator={status} />
      <div data-columns className="flex min-h-0 flex-1">
        <main
          data-resize-middle
          className={cn("min-w-0 flex-1 overflow-y-auto", wide ? "p-8" : "px-4 py-4")}
        >
          <div className="mx-auto w-full max-w-[720px] space-y-4">
            <div>
              <h1 className="text-lg font-semibold tracking-tight lg:text-xl">
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
                      <Flag
                        className="text-muted-foreground size-4"
                        aria-hidden="true"
                      />
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
            <div className={wide ? "flex items-center gap-2" : "space-y-2"}>
              <Button
                variant="outline"
                size="lg"
                className={wide ? undefined : "w-full"}
                onClick={onBack}
              >
                {t("takeTest.keepWorking")}
              </Button>
              <Button
                size="lg"
                className={wide ? undefined : "w-full"}
                disabled={busy || lock === "superseded" || lock === "closed"}
                onClick={() => setConfirming(true)}
              >
                {busy ? t("takeTest.submitting") : t("takeTest.submit")}
              </Button>
            </div>
            <p
              className={cn(
                "text-muted-foreground text-xs leading-relaxed",
                !wide && "text-center",
              )}
            >
              {t("takeTest.submitNote")}{" "}
              {remainingAttempts > 0 &&
                t("takeTest.retakeNote", { count: remainingAttempts })}
            </p>
          </div>
        </main>
        {wide && (
          <NavigatorRail dots={dots} current={null} groups={groups} onJump={onJump} />
        )}
      </div>

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
            <Button
              className="flex-1"
              disabled={busy || lock === "superseded" || lock === "closed"}
              onClick={() => void confirm()}
            >
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
}: Readonly<{
  items: (DotState & { index: number })[];
  onJump: (index: number) => void;
}>) {
  const { t } = useTranslation();
  return (
    <div className="flex flex-wrap gap-1.5">
      {items.map((d) => (
        <button
          key={d.id}
          type="button"
          aria-label={t("takeTest.dotLabel", { n: d.index + 1 })}
          onClick={() => onJump(d.index)}
          className="bg-background text-muted-foreground grid h-11 w-11 place-content-center rounded-md border text-xs tabular-nums lg:h-9 lg:w-9"
        >
          {d.index + 1}
        </button>
      ))}
    </div>
  );
}

export { QuestionDots };
