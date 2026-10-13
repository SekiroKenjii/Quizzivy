import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { Check, LoaderCircle, Sparkles, X } from "lucide-react";
import { useTick } from "@/hooks/useTick";
import { countdown } from "@/lib/i18n/datetime";
import { cn } from "@/lib/utils";
import type { ImportRun, ImportSource } from "../api";
import {
  isWaitingToRetry,
  PROCESSING_STAGES,
  stageStates,
  type StageState,
} from "../status";

type Stage = (typeof PROCESSING_STAGES)[number];
type RowState = StageState | "failed";

function stageLabel(stage: Stage, text: boolean, t: TFunction): string {
  return t(
    text && stage.key === "validate"
      ? "imports.processing.textCheck"
      : `imports.processing.stage.${stage.key}`,
  );
}

function rowStates(run: ImportRun | undefined, failed: boolean): RowState[] {
  const waiting = isWaitingToRetry(run);
  const states: RowState[] = stageStates(run?.stage, waiting || failed);
  if (!failed) return states;
  const stopped = stageStates(run?.stage).indexOf("current");
  if (stopped !== -1) states[stopped] = "failed";
  return states;
}

/**
 * ProcessingPanel is the stage list of an import's status pane: each stage
 * done, current or waiting as the run places it, with the line under it that
 * names the current stage, the time elapsed and the attempt while the run is
 * queued or running. Without a run every stage waits, as before a start.
 * With `failed`, the stage the run stopped at is marked failed and nothing
 * spins. With `readWithAi`, the reading stage says that scans are read with
 * AI. It shows no percentage.
 */
export function ProcessingPanel({
  run,
  exam,
  readWithAi = false,
  failed = false,
}: Readonly<{
  run: ImportRun | undefined;
  exam?: ImportSource | undefined;
  readWithAi?: boolean;
  failed?: boolean;
}>) {
  const { t } = useTranslation();
  const text = exam?.role === "exam" && exam.format === "text";
  const states = rowStates(run, failed);
  const live = run !== undefined && !failed && isLive(run);
  return (
    <div
      role={live ? "status" : undefined}
      tabIndex={live ? 0 : undefined}
      className="group"
    >
      <ol
        className="divide-border divide-y"
        aria-label={t("imports.processing.stagesLabel")}
      >
        {PROCESSING_STAGES.map((stage, index) => (
          <StageRow
            key={stage.key}
            stage={stage}
            index={index}
            state={states[index] ?? "waiting"}
            exam={text ? exam : undefined}
            readWithAi={readWithAi && !text}
          />
        ))}
      </ol>
      {live ? <RunLine run={run} states={states} text={text} /> : null}
    </div>
  );
}

function isLive(run: ImportRun): boolean {
  return run.status === "queued" || run.status === "running";
}

function StageRow({
  stage,
  index,
  state,
  exam,
  readWithAi,
}: Readonly<{
  stage: Stage;
  index: number;
  state: RowState;
  exam: ImportSource | undefined;
  readWithAi: boolean;
}>) {
  const { t } = useTranslation();
  const marked = state === "current" || state === "failed";
  return (
    <li
      aria-current={marked ? "step" : undefined}
      className={cn(
        "flex flex-wrap items-center gap-3 px-4.5 py-3.25 text-sm",
        state === "waiting" && "text-muted-foreground",
        marked && "font-medium",
        state === "failed" && "text-danger-ink",
      )}
    >
      <StageMark state={state} index={index} />
      <span className="min-w-0 flex-1">{stageLabel(stage, exam !== undefined, t)}</span>
      {state === "done" ? <TextReceipt stage={stage} exam={exam} /> : null}
      {readWithAi && stage.key === "read" ? (
        <span className="text-muted-fg inline-flex items-center gap-1 text-right text-xs font-normal">
          <Sparkles aria-hidden="true" className="text-brand-ink size-3.5" />
          {t("imports.processing.withAi")}
        </span>
      ) : null}
      <span className="sr-only">{t(`imports.processing.state.${state}`)}</span>
    </li>
  );
}

const MARK_TONES: Record<RowState, string> = {
  done: "bg-success-soft text-success-ink",
  waiting: "bg-muted text-muted-fg",
  failed: "bg-danger-soft text-danger-ink",
  current: "",
};

function StageMark({ state, index }: Readonly<{ state: RowState; index: number }>) {
  return (
    <span
      className={cn(
        "grid size-6 shrink-0 place-items-center rounded-full",
        MARK_TONES[state],
      )}
    >
      <MarkIcon state={state} index={index} />
    </span>
  );
}

function MarkIcon({ state, index }: Readonly<{ state: RowState; index: number }>) {
  switch (state) {
    case "done":
      return <Check className="size-3.5" aria-hidden="true" />;
    case "failed":
      return <X className="size-3.5" aria-hidden="true" />;
    case "current":
      return (
        <LoaderCircle
          className="size-4 group-focus-within:[animation-play-state:paused] group-hover:[animation-play-state:paused] motion-safe:animate-spin motion-safe:[animation-duration:800ms]"
          aria-hidden="true"
        />
      );
    case "waiting":
      return (
        <span className="text-xs" aria-hidden="true">
          {index + 1}
        </span>
      );
  }
}

function TextReceipt({
  stage,
  exam,
}: Readonly<{ stage: Stage; exam: ImportSource | undefined }>) {
  const { t } = useTranslation();
  if (exam === undefined) return null;
  if (stage.key === "validate" && exam.characters !== undefined)
    return (
      <span className="text-muted-fg text-right text-xs">
        {t("imports.processing.charactersChecked", { count: exam.characters })}
      </span>
    );
  if (stage.key === "read")
    return (
      <span className="text-muted-fg text-right text-xs">
        {t("imports.processing.plainText")}
      </span>
    );
  return null;
}

function RunLine({
  run,
  states,
  text,
}: Readonly<{ run: ImportRun; states: readonly RowState[]; text: boolean }>) {
  const { t } = useTranslation();
  const waiting = isWaitingToRetry(run);
  const now = useTick(!waiting);
  const started = Date.parse(run.createdAt);
  const elapsed =
    waiting || Number.isNaN(started) ? null : Math.max(0, now * 1000 - started);
  const current = PROCESSING_STAGES.filter((_, index) => states[index] === "current");
  const currentLabel =
    current.length === 0
      ? t("imports.processing.waitingToStart")
      : current.map((stage) => stageLabel(stage, text, t)).join(" · ");
  return (
    <div className="text-muted-fg flex flex-wrap items-center gap-x-4 gap-y-1 border-t px-4.5 py-3 text-xs">
      <p role="status" aria-live="polite">
        {waiting
          ? t("imports.processing.retryWaiting")
          : t("imports.processing.current", { stage: currentLabel })}
      </p>
      {elapsed === null ? null : (
        <p className="tabular-nums">
          {t("imports.processing.elapsed", { time: countdown(elapsed) })}
        </p>
      )}
      {run.attempt >= 1 ? (
        <p>
          {t("imports.processing.attempt", {
            attempt: run.attempt,
            max: run.maxAttempts,
          })}
        </p>
      ) : null}
    </div>
  );
}
