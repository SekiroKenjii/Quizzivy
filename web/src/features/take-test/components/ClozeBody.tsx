import { useRef, type KeyboardEvent } from "react";
import { useTranslation } from "react-i18next";
import { OptionText } from "@/components/shared/content/OptionText";
import { PAPER_SURFACE } from "@/components/shared/content/paperSurface";
import { QuestionProse } from "@/components/shared/content/QuestionProse";
import { useLargerTestText } from "@/lib/testText";
import { cn } from "@/lib/utils";
import { PromptMedia } from "./PromptMedia";
import { FlagToggle } from "./QuestionCard";
import { answered } from "../answered";
import { usePaper } from "../paper";
import { clozeLine } from "../questionType";
import {
  chosenWord,
  nextEmptyGap,
  sharedPrompt,
  type Cloze,
  type Stop,
} from "../stops";
import { worth } from "../worth";
import type { Answer, StudentQuestion } from "../api";

const OPTION =
  "bg-card text-fg flex min-h-11.5 min-w-0 items-center gap-3 rounded-[11px] border-[1.5px] px-3.5 py-2.5 text-left leading-[1.4] disabled:cursor-default disabled:opacity-60";
const MARKER =
  "grid size-6.5 flex-none place-items-center rounded-full border-[1.5px] text-xs font-semibold";
const ARROW_STEP: Readonly<Record<string, number>> = { ArrowDown: 1, ArrowUp: -1 };
const REPEAT_CLICK_MS = 350;
type ClickAdvance = Readonly<{ position: number; at: number }>;
const CHIP =
  "relative inline-flex h-8 min-h-0 max-w-full min-w-0 items-center gap-1.5 rounded-md border-[1.5px] pr-2.5 pl-[5px] text-sm leading-none font-medium whitespace-nowrap";

/**
 * ClozeBody is a run of a cloze on one screen, as the deck draws it: the line
 * "Questions 23–27 of 34 · Cloze · 5 gaps" with the open gap's flag, which
 * names its question; the prompt every member shares, printed once, when
 * they share one; the hint, which names the cloze's gaps that have screens of
 * their own; a chip per gap of the run; and the card of the open gap, `open`:
 * "Gap 25", how many of the run's gaps are filled, the member's own prompt
 * when there is no shared one, its options and what it is worth. Choosing an
 * option writes that member's answer and opens the next gap with no answer,
 * going round; a chip opens its gap. A click that repeats the one that opened
 * the gap, the second click of a double click or a click on the same option
 * within 350ms of it, chooses nothing, so a double click or a double tap
 * answers one gap; a choice made with a key is never taken for a repeat. A
 * polite live region names the open gap. A locked paper keeps the answers
 * legible and refuses edits.
 */
export function ClozeBody({
  stop,
  cloze,
  open,
  total,
  onOpen,
}: Readonly<{
  stop: Stop;
  cloze: Cloze;
  open: number;
  total: number;
  onOpen: (index: number) => void;
}>) {
  const { t, i18n } = useTranslation();
  const larger = useLargerTestText();
  const { questions, answers, setAnswer, flags, lock } = usePaper();
  const clickAdvance = useRef<ClickAdvance | null>(null);
  const locked = lock !== null;
  const question = questions[open];
  if (question === undefined) return null;
  const shared = sharedPrompt(stop, questions);
  const filled = stop.positions.filter((at) => {
    const member = questions[at];
    return member !== undefined && answered(member, answers[member.id]);
  }).length;
  const elsewhere = cloze.positions
    .filter((at) => !stop.positions.includes(at))
    .map((at) => String(at + 1));
  const list = new Intl.ListFormat(i18n.language, { type: "conjunction" });

  const choose = (optionId: string, position: number, clicks: number) => {
    const now = performance.now();
    const fromPointer = clicks > 0;
    if (fromPointer && repeatsAdvance(clickAdvance.current, position, clicks, now))
      return;
    const answer: Answer = { type: "choice", optionIds: [optionId] };
    setAnswer(question.id, answer);
    const next = nextEmptyGap(stop, open, questions, {
      ...answers,
      [question.id]: answer,
    });
    if (next === null) return;
    clickAdvance.current = fromPointer ? { position, at: now } : null;
    onOpen(next);
  };

  return (
    <div className={cn("flex flex-col gap-4.5", PAPER_SURFACE)}>
      <div className="flex items-center justify-between gap-2.5">
        <p className="text-muted-fg min-w-0 flex-1 text-sm font-medium text-pretty">
          {clozeLine(stop.positions, cloze.positions.length, total, t)}
        </p>
        <FlagToggle questionId={question.id} number={open + 1} disabled={locked} />
      </div>
      {shared !== null && (
        <QuestionProse
          className={cn(
            larger ? "text-[1.1875rem]" : "text-lg",
            "leading-[1.6]! font-medium text-pretty",
          )}
          text={shared.prompt}
          content={shared.promptContent}
        />
      )}
      <div className="flex flex-col gap-1.5">
        <p className="text-muted-fg text-meta leading-normal text-pretty">
          {t("takeTest.cloze.hint")}
          {elsewhere.length > 0 && (
            <>
              {" "}
              <strong className="text-fg font-semibold">
                {t(
                  elsewhere.length === 1
                    ? "takeTest.cloze.elsewhereOne"
                    : "takeTest.cloze.elsewhereMany",
                  { list: list.format(elsewhere) },
                )}
              </strong>
            </>
          )}
        </p>
        <div className="flex flex-wrap items-center gap-1.5">
          {stop.positions.map((at) => {
            const member = questions[at];
            if (member === undefined) return null;
            return (
              <GapStripChip
                key={member.id}
                number={at + 1}
                word={chosenWord(member, answers[member.id], t)}
                open={at === open}
                flagged={flags.has(member.id)}
                onClick={() => onOpen(at)}
              />
            );
          })}
        </div>
      </div>
      <PromptMedia assets={question.promptAssets}>
        <GapCard
          question={question}
          number={open + 1}
          filled={filled}
          size={stop.positions.length}
          ownPrompt={shared === null}
          answer={answers[question.id]}
          larger={larger}
          disabled={locked}
          onChoose={choose}
        />
      </PromptMedia>
      <p className="sr-only" aria-live="polite">
        {t("takeTest.cloze.gap", { n: open + 1 })}
      </p>
    </div>
  );
}

function repeatsAdvance(
  last: ClickAdvance | null,
  position: number,
  clicks: number,
  now: number,
): boolean {
  if (clicks > 1) return true;
  return last !== null && last.position === position && now - last.at < REPEAT_CLICK_MS;
}

function stripTone(open: boolean, word: string | null): string {
  if (open) return "bg-primary text-primary-fg border-primary";
  return word === null
    ? "bg-card text-fg border-border"
    : "bg-brand-soft text-fg border-border";
}

function GapStripChip({
  number,
  word,
  open,
  flagged,
  onClick,
}: Readonly<{
  number: number;
  word: string | null;
  open: boolean;
  flagged: boolean;
  onClick: () => void;
}>) {
  const { t } = useTranslation();
  return (
    <button
      type="button"
      aria-current={open ? "true" : undefined}
      aria-label={t("takeTest.cloze.chip", {
        n: number,
        word: word ?? t("takeTest.cloze.empty"),
      })}
      className={cn(CHIP, stripTone(open, word))}
      onClick={onClick}
    >
      <span
        aria-hidden="true"
        className="bg-muted text-fg inline-flex size-5 flex-none items-center justify-center rounded-full text-[11px] font-bold tabular-nums"
      >
        {number}
      </span>
      <span aria-hidden="true" className="min-w-0 truncate leading-normal">
        {word ?? "—"}
      </span>
      {flagged && (
        <span
          aria-hidden="true"
          className="bg-warning border-card absolute -top-1 -right-1 size-[9px] rounded-full border-2"
        />
      )}
    </button>
  );
}

function moveFocusInGroup(event: KeyboardEvent<HTMLButtonElement>) {
  const step = ARROW_STEP[event.key];
  const group = event.currentTarget.closest<HTMLElement>('[role="radiogroup"]');
  if (step === undefined || group === null) return;
  if (event.altKey || event.ctrlKey || event.metaKey) return;
  const radios = Array.from(
    group.querySelectorAll<HTMLButtonElement>('[role="radio"]:not(:disabled)'),
  );
  const at = radios.indexOf(event.currentTarget);
  if (at < 0) return;
  event.preventDefault();
  radios[(at + step + radios.length) % radios.length]?.focus();
}

function GapCard({
  question,
  number,
  filled,
  size,
  ownPrompt,
  answer,
  larger,
  disabled,
  onChoose,
}: Readonly<{
  question: StudentQuestion;
  number: number;
  filled: number;
  size: number;
  ownPrompt: boolean;
  answer: Answer | undefined;
  larger: boolean;
  disabled: boolean;
  onChoose: (optionId: string, position: number, clicks: number) => void;
}>) {
  const { t } = useTranslation();
  const title = t("takeTest.cloze.gap", { n: number });
  const chosen = new Set(answer?.type === "choice" ? answer.optionIds : []);
  const options = question.options ?? [];
  const tabStop = Math.max(
    options.findIndex((option) => chosen.has(option.id)),
    0,
  );
  return (
    <div
      role="group"
      aria-label={title}
      className="bg-sidebar flex flex-col gap-2.5 rounded-xl border p-3.5"
    >
      <div className="flex flex-wrap items-center justify-between gap-x-2.5 gap-y-1">
        <span className="text-ui leading-normal font-semibold whitespace-nowrap">
          {title}
        </span>
        <span className="text-muted-fg text-meta leading-normal whitespace-nowrap">
          {t("takeTest.cloze.filled", { filled, count: size })}
        </span>
      </div>
      {ownPrompt && (
        <QuestionProse
          className={cn(
            larger ? "text-lg" : "text-md",
            "leading-normal! font-medium text-pretty",
          )}
          text={question.prompt}
          content={question.promptContent}
        />
      )}
      <div
        role="radiogroup"
        aria-label={t("takeTest.answerOptions")}
        className="grid grid-cols-[repeat(auto-fill,minmax(132px,1fr))] gap-2"
      >
        {options.map((option, index) => {
          const selected = chosen.has(option.id);
          return (
            <button
              key={index}
              type="button"
              role="radio"
              aria-checked={selected}
              tabIndex={index === tabStop ? 0 : -1}
              onKeyDown={moveFocusInGroup}
              disabled={disabled}
              className={cn(
                larger ? "text-lg" : "text-md",
                OPTION,
                selected ? "border-primary" : "border-border",
                !disabled && !selected && "hover:border-ring",
              )}
              onClick={(event) => onChoose(option.id, index, event.detail)}
            >
              <span
                aria-hidden="true"
                className={cn(
                  MARKER,
                  selected
                    ? "border-primary bg-primary text-primary-fg"
                    : "border-ring text-muted-fg",
                )}
              >
                {String.fromCharCode(65 + index)}
              </span>
              <span className="min-w-0 flex-1">
                <OptionText
                  text={option.text}
                  content={option.content}
                  type={question.type}
                />
              </span>
            </button>
          );
        })}
      </div>
      <p className="text-muted-fg text-meta leading-normal">{worth(question, t)}</p>
    </div>
  );
}
