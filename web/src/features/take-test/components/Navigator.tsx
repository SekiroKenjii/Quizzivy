import { useEffect, useRef, useState, type ReactNode, type Ref } from "react";
import { useTranslation } from "react-i18next";
import { ArrowLeft, ArrowRight, LayoutGrid } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
import { cn } from "@/lib/utils";
import type { SectionGroup } from "../sections";

/**
 * DotState is what one question's square shows: whether the question is
 * answered and whether the student flagged it. The page computes the list
 * once; the strip, the question sheet and the Submit dialog read it.
 */
export interface DotState {
  id: string;
  answered: boolean;
  flagged: boolean;
}

const RAIL_WIDTH_KEY = "quizzivy.column.studentNavigator";

const STEP =
  "inline-flex h-11 flex-none items-center gap-1.5 rounded-lg text-base leading-none";

function forgetRailWidth() {
  try {
    localStorage.removeItem(RAIL_WIDTH_KEY);
  } catch {
    return;
  }
}

interface Squares {
  dots: DotState[];
  current: number;
  groups: SectionGroup[];
  onJump: (index: number) => void;
}

/**
 * Navigator is the engine's footer, under the question pane: Previous, the
 * questions, and Next, which on the last question is Finish and asks to hand
 * the paper in. From 768 the questions are a strip of numbered squares that
 * wraps and, past four rows, scrolls inside itself. Below 768 one button
 * says which question is open and how many are answered, and opens the same
 * squares in a bottom sheet. A square is filled when its question is
 * answered, has the accent border when it is the one on screen and an amber
 * dot when it is flagged. On a paper of several parts the strip leaves a gap
 * between parts, the sheet names each part, and a square's name includes its
 * part. Without `onFinish` the last question offers no Finish. On mount it
 * deletes the width the retired navigator rail remembered in this browser.
 */
export function Navigator({
  wide,
  dots,
  current,
  groups,
  onMove,
  onJump,
  onFinish,
}: Readonly<
  Squares & {
    wide: boolean;
    onMove: (index: number) => void;
    onFinish?: (() => void) | undefined;
  }
>) {
  const { t } = useTranslation();
  useEffect(forgetRailWidth, []);
  const last = current >= dots.length - 1;

  return (
    <nav
      aria-label={t("takeTest.navTitle")}
      className={cn(
        "bg-bg flex flex-none items-center gap-2.5 border-t pt-2.5",
        wide ? "px-6" : "px-3.5",
      )}
      style={{ paddingBottom: "max(0.625rem, env(safe-area-inset-bottom))" }}
    >
      <button
        type="button"
        aria-label={t("takeTest.previousQuestion")}
        disabled={current === 0}
        className={cn(
          STEP,
          "bg-card hover:bg-muted border px-3.5 font-medium disabled:pointer-events-none disabled:opacity-40",
        )}
        onClick={() => onMove(Math.max(0, current - 1))}
      >
        <ArrowLeft aria-hidden="true" className="size-4" />
        {wide && t("takeTest.previous")}
      </button>
      {wide ? (
        <Strip dots={dots} current={current} groups={groups} onJump={onJump} />
      ) : (
        <SheetButton dots={dots} current={current} groups={groups} onJump={onJump} />
      )}
      {last ? (
        onFinish !== undefined && <Step onClick={onFinish}>{t("takeTest.finish")}</Step>
      ) : (
        <Step onClick={() => onMove(current + 1)}>{t("takeTest.next")}</Step>
      )}
    </nav>
  );
}

function Step({
  onClick,
  children,
}: Readonly<{ onClick: () => void; children: ReactNode }>) {
  return (
    <button
      type="button"
      className={cn(
        STEP,
        "bg-primary text-primary-fg px-4 font-semibold whitespace-nowrap hover:opacity-90",
      )}
      onClick={onClick}
    >
      {children}
      <ArrowRight aria-hidden="true" className="size-4" />
    </button>
  );
}

function Strip({ dots, current, groups, onJump }: Readonly<Squares>) {
  const now = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    now.current?.scrollIntoView?.({ block: "nearest" });
  }, [current]);

  return (
    <div className="-m-1 flex max-h-40 min-w-0 flex-1 flex-wrap justify-center gap-[5px] overflow-y-auto p-1">
      {partsOf(groups, dots).flatMap((part, at) =>
        part.squares.map(({ dot, index }, place) => (
          <Square
            key={dot.id}
            ref={index === current ? now : undefined}
            dot={dot}
            index={index}
            current={current}
            part={part.title}
            className={cn(
              "size-8.5 min-h-0 rounded-md text-sm leading-none tabular-nums",
              at > 0 && place === 0 && "ml-3",
            )}
            flagRing="border-bg"
            onJump={onJump}
          />
        )),
      )}
    </div>
  );
}

function SheetButton({ dots, current, groups, onJump }: Readonly<Squares>) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const opener = useRef<HTMLButtonElement>(null);
  const now = useRef<HTMLButtonElement>(null);
  const answered = dots.filter((dot) => dot.answered).length;

  return (
    <>
      <button
        ref={opener}
        type="button"
        aria-haspopup="dialog"
        aria-expanded={open}
        className="bg-card hover:bg-muted inline-flex h-11 min-w-0 flex-1 items-center justify-center gap-2 overflow-hidden rounded-lg border text-base leading-[1.2] font-medium"
        onClick={() => setOpen(true)}
      >
        <LayoutGrid aria-hidden="true" className="size-4 flex-none" />
        <span className="sr-only">{t("takeTest.navTitle")}:</span>{" "}
        <span className="flex min-w-0 flex-wrap justify-center gap-x-[0.23em]">
          <span className="whitespace-nowrap">
            {t("takeTest.navPosition", { n: current + 1, total: dots.length })}
          </span>{" "}
          <span className="whitespace-nowrap">
            {t("takeTest.navAnswered", { answered })}
          </span>
        </span>
      </button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent
          showCloseButton={false}
          className="bg-card shadow-float top-auto right-0 bottom-0 left-0 flex max-h-[85svh] max-w-none translate-x-0 translate-y-0 flex-col gap-3.5 rounded-t-3xl rounded-b-none border-0 px-4 pt-2.5 pb-5 sm:max-w-none"
          style={{ paddingBottom: "max(1.25rem, env(safe-area-inset-bottom))" }}
          onOpenAutoFocus={(event) => {
            if (now.current === null) return;
            event.preventDefault();
            now.current.focus();
          }}
          onCloseAutoFocus={(event) => {
            event.preventDefault();
            const active = document.activeElement;
            if (active === null || active === document.body) opener.current?.focus();
          }}
        >
          <span
            aria-hidden="true"
            className="bg-border h-1 w-10 flex-none self-center rounded-full"
          />
          <div className="flex items-center justify-between gap-3">
            <DialogTitle className="text-md leading-normal font-semibold">
              {t("takeTest.navTitle")}
            </DialogTitle>
            <DialogDescription className="text-muted-fg text-meta leading-normal">
              {t("takeTest.navCount", { answered, total: dots.length })}
            </DialogDescription>
          </div>
          <div className="-m-1 flex min-h-0 flex-col gap-3.5 overflow-y-auto p-1">
            {partsOf(groups, dots).map((part) => (
              <div key={part.id} className="flex flex-col gap-2">
                {part.title ? (
                  <h3 className="text-muted-fg text-meta leading-normal font-semibold tracking-[0.02em] uppercase">
                    {part.title}
                  </h3>
                ) : null}
                <div className="grid grid-cols-6 gap-2">
                  {part.squares.map(({ dot, index }) => (
                    <Square
                      key={dot.id}
                      ref={index === current ? now : undefined}
                      dot={dot}
                      index={index}
                      current={current}
                      part={part.title}
                      className="rounded-ctl h-11 text-base leading-none"
                      flagRing="border-card"
                      onJump={(next) => {
                        setOpen(false);
                        onJump(next);
                      }}
                    />
                  ))}
                </div>
              </div>
            ))}
          </div>
          <div className="text-muted-fg flex flex-wrap gap-x-3.5 gap-y-1 text-xs leading-normal">
            <span className="flex items-center gap-1.5">
              <span
                aria-hidden="true"
                className="bg-primary size-3 flex-none rounded-[3px]"
              />
              {t("takeTest.legendAnswered")}
            </span>
            <span className="flex items-center gap-1.5">
              <span
                aria-hidden="true"
                className="bg-warning size-2.5 flex-none rounded-full"
              />
              {t("takeTest.legendFlagged")}
            </span>
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}

interface Part {
  id: string;
  title: string;
  squares: { dot: DotState; index: number }[];
}

function partsOf(groups: SectionGroup[], dots: DotState[]): Part[] {
  const squares = dots.map((dot, index) => ({ dot, index }));
  if (groups.length < 2) return [{ id: "paper", title: "", squares }];
  return groups.map((group) => ({
    id: group.section.id,
    title: group.section.title,
    squares: group.indexes.flatMap((index) => {
      const square = squares[index];
      return square === undefined ? [] : [square];
    }),
  }));
}

function toneOf(dot: DotState, on: boolean): string {
  const fill = dot.answered ? "bg-primary text-primary-fg" : "bg-card text-fg";
  if (on) return `${fill} border-brand`;
  return dot.answered ? `${fill} border-primary` : `${fill} border-border`;
}

function Square({
  ref,
  dot,
  index,
  current,
  part,
  className,
  flagRing,
  onJump,
}: Readonly<{
  ref: Ref<HTMLButtonElement> | undefined;
  dot: DotState;
  index: number;
  current: number;
  part: string;
  className: string;
  flagRing: string;
  onJump: (index: number) => void;
}>) {
  const { t } = useTranslation();
  const on = index === current;
  const name = [
    t("takeTest.dotLabel", { n: index + 1 }),
    part,
    on ? t("takeTest.dotCurrent") : "",
    dot.answered ? t("takeTest.dotAnswered") : "",
    dot.flagged ? t("takeTest.dotFlagged") : "",
  ].filter((piece) => piece !== "");

  return (
    <button
      ref={ref}
      type="button"
      aria-current={on ? "true" : undefined}
      aria-label={name.join(", ")}
      className={cn(
        "relative min-w-0 flex-none border-[1.5px] font-semibold",
        toneOf(dot, on),
        className,
      )}
      onClick={() => onJump(index)}
    >
      {index + 1}
      {dot.flagged && (
        <span
          aria-hidden="true"
          className={cn(
            "bg-warning absolute -top-1 -right-1 size-2.5 rounded-full border-2",
            flagRing,
          )}
        />
      )}
    </button>
  );
}
