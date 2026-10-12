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
 * answered, whether it is partly answered (a fill-in with some of its blanks
 * typed, which is not answered), and whether the student flagged it. The page
 * computes the list once; the rail, the question sheet and the Submit dialog
 * read it.
 */
export interface DotState {
  id: string;
  answered: boolean;
  partly: boolean;
  flagged: boolean;
}

/**
 * KeysHint is what the footer says after the arrow keys while the rail
 * shows: the long sentence, the short one beside shared content, or how to
 * move only, on a question with no options to choose.
 */
export type KeysHint = "long" | "short" | "move";

/**
 * Squares is what a set of question squares reads: every question's state,
 * the open question, the paper's sections and the move to a question.
 */
export interface Squares {
  dots: DotState[];
  current: number;
  groups: SectionGroup[];
  onJump: (index: number) => void;
}

const RAIL_WIDTH_KEY = "quizzivy.column.studentNavigator";

const STEP =
  "inline-flex h-11 flex-none items-center gap-1.5 rounded-lg text-base leading-none";

const ARROW_KEYS = ["←", "→"] as const;

const HINT_KEYS = {
  long: "takeTest.keysHint",
  short: "takeTest.keysHintShort",
  move: "takeTest.keysHintMove",
} as const;

function forgetRailWidth() {
  try {
    localStorage.removeItem(RAIL_WIDTH_KEY);
  } catch {
    return;
  }
}

/**
 * Navigator is the engine's footer, under the question pane: Previous, a
 * middle and Next, which on the last question is Finish and asks to hand the
 * paper in. While the sections rail shows (`rail`), the middle is the key
 * hint: the arrow keys, then `hint`'s sentence. Otherwise, at every width, it
 * is the grid button, which says which question is open and how many are
 * answered, in the short form when `short`, and opens the question sheet: a
 * bottom sheet below 768 and a floating card from 768 (`wide`), where
 * Previous also shows its label. Without `onFinish` the last question offers
 * no Finish. On mount it deletes the width the old resizable rail remembered
 * in this browser.
 */
export function Navigator({
  wide,
  rail,
  hint,
  short,
  dots,
  current,
  groups,
  onMove,
  onJump,
  onFinish,
}: Readonly<
  Squares & {
    wide: boolean;
    rail: boolean;
    hint: KeysHint;
    short: boolean;
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
        "bg-bg flex flex-none items-center gap-2.5 border-t pt-2.5 pb-2.5",
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
      {rail ? (
        <KeyHint hint={hint} />
      ) : (
        <SheetButton
          wide={wide}
          short={short}
          dots={dots}
          current={current}
          groups={groups}
          onJump={onJump}
        />
      )}
      {last ? (
        onFinish !== undefined && <Step onClick={onFinish}>{t("takeTest.finish")}</Step>
      ) : (
        <Step label={t("takeTest.nextQuestion")} onClick={() => onMove(current + 1)}>
          {t("takeTest.next")}
        </Step>
      )}
    </nav>
  );
}

function Step({
  label,
  onClick,
  children,
}: Readonly<{ label?: string; onClick: () => void; children: ReactNode }>) {
  return (
    <button
      type="button"
      aria-label={label}
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

function KeyHint({ hint }: Readonly<{ hint: KeysHint }>) {
  const { t } = useTranslation();
  return (
    <p className="text-muted-fg text-meta flex min-w-0 flex-1 items-center justify-center gap-2 overflow-hidden leading-normal whitespace-nowrap">
      <span className="inline-flex flex-none gap-[3px]">
        {ARROW_KEYS.map((key) => (
          <Key key={key}>{key}</Key>
        ))}
      </span>
      <span className="truncate">{t(HINT_KEYS[hint])}</span>
    </p>
  );
}

function Key({ children }: Readonly<{ children: string }>) {
  return (
    <kbd className="bg-card text-fg inline-flex h-5.5 min-w-5.5 items-center justify-center rounded-[5px] border border-b-2 px-[5px] font-mono">
      {children}
    </kbd>
  );
}

function SheetButton({
  wide,
  short,
  dots,
  current,
  groups,
  onJump,
}: Readonly<Squares & { wide: boolean; short: boolean }>) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const opener = useRef<HTMLButtonElement>(null);
  const now = useRef<HTMLButtonElement>(null);
  const counts = {
    no: current + 1,
    answered: dots.filter((dot) => dot.answered).length,
    total: dots.length,
  };
  const label = t("takeTest.gridLabel", counts);

  return (
    <>
      <button
        ref={opener}
        type="button"
        aria-label={label}
        aria-haspopup="dialog"
        aria-expanded={open}
        className="bg-card hover:bg-muted inline-flex h-11 min-w-0 flex-1 items-center justify-center gap-2 rounded-lg border px-2 text-base leading-none font-medium whitespace-nowrap"
        onClick={() => setOpen(true)}
      >
        <LayoutGrid aria-hidden="true" className="size-4 flex-none" />
        <span className="min-w-0 truncate leading-normal">
          {short ? t("takeTest.gridLabelShort", counts) : label}
        </span>
      </button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent
          showCloseButton={false}
          className={cn(
            "bg-card shadow-float top-auto flex max-h-[80%] max-w-none translate-x-0 translate-y-0 flex-col gap-4 overflow-y-auto border px-4 pt-3 pb-5 sm:max-w-none",
            wide
              ? "right-[calc(50%-280px)] bottom-[76px] left-[calc(50%-280px)] w-auto rounded-2xl"
              : "right-0 bottom-0 left-0 rounded-t-3xl rounded-b-none",
          )}
          style={
            wide
              ? undefined
              : { paddingBottom: "max(1.25rem, env(safe-area-inset-bottom))" }
          }
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
          <DialogTitle className="sr-only">{t("takeTest.navTitle")}</DialogTitle>
          <DialogDescription className="sr-only">{label}</DialogDescription>
          <SectionBlocks
            shape="sheet"
            dots={dots}
            current={current}
            groups={groups}
            currentRef={now}
            onJump={(next) => {
              setOpen(false);
              onJump(next);
            }}
          />
          <Legend />
        </DialogContent>
      </Dialog>
    </>
  );
}

/**
 * SectionBlocks draws one block per section of the paper: the section's
 * title, beside how many of its questions are answered, over a grid of its
 * questions' squares, five columns of 34px squares in the rail and six in
 * the sheet. A section with no title has no heading. On a paper of several
 * sections a square's name includes its section. `currentRef` receives the
 * open question's square.
 */
export function SectionBlocks({
  shape,
  dots,
  current,
  groups,
  currentRef,
  onJump,
}: Readonly<
  Squares & {
    shape: "rail" | "sheet";
    currentRef: Ref<HTMLButtonElement>;
  }
>) {
  const { t } = useTranslation();
  const named = groups.length > 1;
  return partsOf(groups, dots).map((part) => {
    const on = part.squares.some(({ index }) => index === current);
    return (
      <div key={part.id} className="flex flex-col gap-2">
        {part.title !== "" && (
          <div className="flex items-start justify-between gap-2">
            <h3
              className={cn(
                "text-meta min-w-0 flex-1 leading-[1.35]",
                on ? "text-fg font-semibold" : "text-muted-fg font-medium",
              )}
            >
              {part.title}
            </h3>
            <span className="text-caption text-muted-fg leading-normal whitespace-nowrap tabular-nums">
              {t("takeTest.railDone", {
                done: part.squares.filter(({ dot }) => dot.answered).length,
                total: part.squares.length,
              })}
            </span>
          </div>
        )}
        <div
          className={cn(
            "grid gap-[5px]",
            shape === "rail" ? "grid-cols-5" : "grid-cols-6",
          )}
        >
          {part.squares.map(({ dot, index }) => (
            <Square
              key={dot.id}
              ref={index === current ? currentRef : undefined}
              dot={dot}
              index={index}
              current={current}
              part={named ? part.title : ""}
              className={shape === "rail" ? "h-8.5 min-h-0" : "h-11 lg:h-10.5"}
              onJump={onJump}
            />
          ))}
        </div>
      </div>
    );
  });
}

/** Legend names the three marks a square can carry: answered, partly answered and flagged. */
export function Legend({ className }: Readonly<{ className?: string }>) {
  const { t } = useTranslation();
  return (
    <div
      className={cn(
        "text-muted-fg text-caption flex flex-wrap gap-x-3.5 gap-y-1.5 leading-normal",
        className,
      )}
    >
      <span className="flex items-center gap-1.5">
        <span
          aria-hidden="true"
          className="bg-primary size-[11px] flex-none rounded-[3px]"
        />
        {t("takeTest.legendAnswered")}
      </span>
      <span className="flex items-center gap-1.5">
        <span
          aria-hidden="true"
          className="bg-brand-soft border-primary size-[11px] flex-none rounded-[3px] border"
        />
        {t("takeTest.legendPartly")}
      </span>
      <span className="flex items-center gap-1.5">
        <span
          aria-hidden="true"
          className="bg-warning size-[9px] flex-none rounded-full"
        />
        {t("takeTest.legendFlagged")}
      </span>
    </div>
  );
}

interface Part {
  id: string;
  title: string;
  squares: { dot: DotState; index: number }[];
}

function partsOf(groups: SectionGroup[], dots: DotState[]): Part[] {
  const squares = dots.map((dot, index) => ({ dot, index }));
  if (groups.length === 0) return [{ id: "paper", title: "", squares }];
  return groups.map((group) => ({
    id: group.section.id,
    title: group.section.title,
    squares: group.indexes.flatMap((index) => {
      const square = squares[index];
      return square === undefined ? [] : [square];
    }),
  }));
}

function fillOf(dot: DotState): string {
  if (dot.answered) return "bg-primary text-primary-fg";
  if (dot.partly) return "bg-brand-soft text-fg";
  return "bg-card text-fg";
}

function toneOf(dot: DotState, on: boolean): string {
  if (on) return `${fillOf(dot)} border-brand ring-2 ring-brand-soft`;
  const marked = dot.answered || dot.partly;
  return `${fillOf(dot)} ${marked ? "border-primary" : "border-border"}`;
}

function Square({
  ref,
  dot,
  index,
  current,
  part,
  className,
  onJump,
}: Readonly<{
  ref: Ref<HTMLButtonElement> | undefined;
  dot: DotState;
  index: number;
  current: number;
  part: string;
  className: string;
  onJump: (index: number) => void;
}>) {
  const { t } = useTranslation();
  const on = index === current;
  const name = [
    t("takeTest.dotLabel", { n: index + 1 }),
    part,
    on ? t("takeTest.dotCurrent") : "",
    dot.answered ? t("takeTest.dotAnswered") : "",
    dot.partly ? t("takeTest.dotPartly") : "",
    dot.flagged ? t("takeTest.dotFlagged") : "",
  ].filter((piece) => piece !== "");

  return (
    <button
      ref={ref}
      type="button"
      aria-current={on ? "true" : undefined}
      aria-label={name.join(", ")}
      className={cn(
        "text-meta relative min-w-0 rounded-md border-[1.5px] p-0 leading-none font-semibold tabular-nums",
        toneOf(dot, on),
        className,
      )}
      onClick={() => onJump(index)}
    >
      {index + 1}
      {dot.flagged && (
        <span
          aria-hidden="true"
          className="bg-warning border-card absolute -top-1 -right-1 size-[9px] rounded-full border-2"
        />
      )}
    </button>
  );
}
