import { useTranslation } from "react-i18next";
import { X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { Separator } from "@/components/ui/separator";
import { PageAside } from "@/components/shared/PageAside";
import { cn } from "@/lib/utils";

/** What one dot needs to know. Computed once by the page, read by every view. */
export interface DotState {
  id: string;
  answered: boolean;
  flagged: boolean;
}

/**
 * S-06's grid: one dot per question, three states that combine. The dot is a
 * button, because jumping is the point, and its label carries the states so a
 * screen reader hears "Câu 4, đã đánh dấu" rather than "4".
 */
export function QuestionDots({
  dots,
  current,
  onJump,
}: {
  dots: DotState[];
  current: number | null;
  onJump: (index: number) => void;
}) {
  const { t } = useTranslation();
  return (
    <div className="grid grid-cols-[repeat(auto-fill,minmax(2.25rem,1fr))] gap-1.5">
      {dots.map((dot, i) => {
        const states = [
          i === current ? t("takeTest.dotCurrent") : null,
          dot.answered ? t("takeTest.dotAnswered") : null,
          dot.flagged ? t("takeTest.dotFlagged") : null,
        ].filter((s): s is string => s !== null);
        return (
          <button
            key={dot.id}
            type="button"
            aria-current={i === current ? "true" : undefined}
            aria-label={[t("takeTest.dotLabel", { n: i + 1 }), ...states].join(", ")}
            onClick={() => onJump(i)}
            className={cn(
              "bg-background grid h-9 place-content-center rounded-md border text-xs tabular-nums",
              dot.answered && "bg-secondary text-foreground font-medium",
              dot.flagged && "border-warning/55",
              i === current &&
                "border-foreground ring-foreground text-foreground font-semibold ring-1 ring-inset",
            )}
          >
            {i + 1}
          </button>
        );
      })}
    </div>
  );
}

function Legend() {
  const { t } = useTranslation();
  const sample = "inline-block size-4 rounded-sm border";
  return (
    <div className="text-muted-foreground flex flex-wrap items-center gap-3 text-xs">
      <span className="flex items-center gap-1.5">
        <span className={cn(sample, "bg-secondary")} aria-hidden="true" />
        {t("takeTest.legendAnswered")}
      </span>
      <span className="flex items-center gap-1.5">
        <span className={cn(sample, "border-warning/55")} aria-hidden="true" />
        {t("takeTest.legendFlagged")}
      </span>
      <span className="flex items-center gap-1.5">
        <span className={cn(sample, "bg-background")} aria-hidden="true" />
        {t("takeTest.legendUnanswered")}
      </span>
    </div>
  );
}

/**
 * The phone's navigator: a bottom sheet in thumb range (S-06). The same
 * dialog primitive as everything else, pinned to the bottom edge -- and, unlike
 * the integrity dialog, dismissible every way, because nothing here needs to
 * be read before it goes away.
 */
export function NavigatorSheet({
  open,
  onOpenChange,
  dots,
  current,
  onJump,
  onReview,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  dots: DotState[];
  current: number;
  onJump: (index: number) => void;
  onReview: () => void;
}) {
  const { t } = useTranslation();
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        showCloseButton={false}
        className="top-auto right-0 bottom-0 left-0 max-w-none translate-x-0 translate-y-0 gap-0 rounded-t-lg rounded-b-none border-t p-4 sm:max-w-none"
        style={{ paddingBottom: "max(1rem, env(safe-area-inset-bottom))" }}
      >
        <div className="flex items-center justify-between">
          <DialogTitle className="text-sm font-semibold">
            {t("takeTest.navTitle")}
          </DialogTitle>
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={t("takeTest.close")}
            onClick={() => onOpenChange(false)}
          >
            <X aria-hidden="true" />
          </Button>
        </div>
        <div className="mt-4">
          <QuestionDots dots={dots} current={current} onJump={onJump} />
        </div>
        <div className="mt-4">
          <Legend />
        </div>
        <Button className="mt-4 w-full" onClick={onReview}>
          {t("takeTest.reviewAndSubmit")}
        </Button>
      </DialogContent>
    </Dialog>
  );
}

/** From 1024px the same navigator stands beside the paper (S-08). */
export function NavigatorRail({
  dots,
  current,
  onJump,
  onReview,
}: {
  dots: DotState[];
  current: number;
  onJump: (index: number) => void;
  onReview: () => void;
}) {
  const { t } = useTranslation();
  const answered = dots.filter((d) => d.answered).length;
  const flagged = dots.filter((d) => d.flagged).length;
  return (
    <PageAside label={t("takeTest.navTitle")} hideBelow="lg">
      <QuestionDots dots={dots} current={current} onJump={onJump} />
      <Separator />
      <div className="text-muted-foreground space-y-1.5 text-xs">
        <p>
          {t("takeTest.railAnswered")}{" "}
          <span className="text-foreground font-medium">
            {t("takeTest.railCount", { answered, total: dots.length })}
          </span>
        </p>
        <p>
          {t("takeTest.railFlagged")}{" "}
          <span className="text-foreground font-medium">{flagged}</span>
        </p>
      </div>
      <Button className="w-full" onClick={onReview}>
        {t("takeTest.reviewAndSubmit")}
      </Button>
    </PageAside>
  );
}
