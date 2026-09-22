import { useTranslation } from "react-i18next";
import { X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { Separator } from "@/components/ui/separator";
import { PageAside } from "@/components/shared/PageAside";
import { cn } from "@/lib/utils";
import type { SectionGroup } from "../sections";

/** What one dot needs to know. Computed once by the page, read by every view. */
export interface DotState {
  id: string;
  answered: boolean;
  flagged: boolean;
}

/**
 * S-06's grid: one dot per question, three states that combine. The dot is a
 * button, because jumping is the point, and its label carries the states so a
 * screen reader hears "Câu 4, đã đánh dấu" rather than "4". With more than one
 * section the grid splits under each section's title (S-06, S-08) -- the
 * builder names them "Phần n" by default; one section needs no heading.
 */
export function QuestionDots({
  dots,
  current,
  onJump,
  groups = [],
}: Readonly<{
  dots: DotState[];
  current: number | null;
  onJump: (index: number) => void;
  groups?: SectionGroup[];
}>) {
  if (groups.length < 2) {
    return (
      <Grid>
        {dots.map((dot, i) => (
          <Dot key={dot.id} dot={dot} index={i} current={current} onJump={onJump} />
        ))}
      </Grid>
    );
  }
  return (
    <div className="space-y-4">
      {groups.map((group) => (
        <div key={group.section.id}>
          <p className="text-muted-foreground mb-2 text-xs">{group.section.title}</p>
          <Grid>
            {group.indexes.map((i) => {
              const dot = dots[i];
              return dot === undefined ? null : (
                <Dot
                  key={dot.id}
                  dot={dot}
                  index={i}
                  current={current}
                  onJump={onJump}
                />
              );
            })}
          </Grid>
        </div>
      ))}
    </div>
  );
}

function Grid({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <div className="grid grid-cols-[repeat(auto-fill,minmax(2.75rem,1fr))] gap-1.5 lg:grid-cols-[repeat(auto-fill,minmax(2.25rem,1fr))]">
      {children}
    </div>
  );
}

function Dot({
  dot,
  index,
  current,
  onJump,
}: Readonly<{
  dot: DotState;
  index: number;
  current: number | null;
  onJump: (index: number) => void;
}>) {
  const { t } = useTranslation();
  const states = [
    index === current ? t("takeTest.dotCurrent") : null,
    dot.answered ? t("takeTest.dotAnswered") : null,
    dot.flagged ? t("takeTest.dotFlagged") : null,
  ].filter((s): s is string => s !== null);
  return (
    <button
      type="button"
      aria-current={index === current ? "true" : undefined}
      aria-label={[t("takeTest.dotLabel", { n: index + 1 }), ...states].join(", ")}
      onClick={() => onJump(index)}
      className={cn(
        "bg-background text-muted-foreground grid h-11 place-content-center rounded-md border text-xs tabular-nums lg:h-9",
        dot.answered && "bg-secondary text-foreground font-medium",
        dot.flagged && "border-warning/55",
        index === current &&
          "border-foreground ring-foreground text-foreground font-semibold ring-1 ring-inset",
      )}
    >
      {index + 1}
    </button>
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
  groups,
  onJump,
  onReview,
}: Readonly<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  dots: DotState[];
  current: number;
  groups: SectionGroup[];
  onJump: (index: number) => void;
  onReview: () => void;
}>) {
  const { t } = useTranslation();
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        showCloseButton={false}
        className="student-surface top-auto right-0 bottom-0 left-0 max-h-[85svh] max-w-none translate-x-0 translate-y-0 gap-0 overflow-y-auto rounded-t-lg rounded-b-none border-t p-4 sm:max-w-none"
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
          <QuestionDots dots={dots} current={current} onJump={onJump} groups={groups} />
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

/**
 * From 1024px the same navigator stands beside the paper (S-08). On the
 * review it keeps its dots and loses its button: that page is the button (S-15).
 */
export function NavigatorRail({
  dots,
  current,
  groups,
  onJump,
  onReview,
}: Readonly<{
  dots: DotState[];
  current: number | null;
  groups: SectionGroup[];
  onJump: (index: number) => void;
  onReview?: () => void;
}>) {
  const { t } = useTranslation();
  const answered = dots.filter((d) => d.answered).length;
  const flagged = dots.filter((d) => d.flagged).length;
  return (
    <PageAside
      label={t("takeTest.navTitle")}
      widthKey="studentNavigator"
      hideBelow="lg"
    >
      <QuestionDots dots={dots} current={current} onJump={onJump} groups={groups} />
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
      {onReview !== undefined && (
        <Button className="w-full" onClick={onReview}>
          {t("takeTest.reviewAndSubmit")}
        </Button>
      )}
    </PageAside>
  );
}
