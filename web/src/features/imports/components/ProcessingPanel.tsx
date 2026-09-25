import { useTranslation } from "react-i18next";
import { Check, Circle, CircleDot } from "lucide-react";
import { useTick } from "@/hooks/useTick";
import { countdown } from "@/lib/i18n/datetime";
import { cn } from "@/lib/utils";
import type { ImportRun } from "../api";
import {
  isWaitingToRetry,
  PROCESSING_STAGES,
  stageStates,
  type StageState,
} from "../status";

const ICONS: Record<StageState, typeof Check> = {
  done: Check,
  current: CircleDot,
  waiting: Circle,
};

/**
 * ProcessingPanel shows the run's current stage, its elapsed time and its
 * attempt, without a percentage. A run queued again after a temporary failure
 * says it will be retried automatically instead of showing a current stage.
 */
export function ProcessingPanel({ run }: Readonly<{ run: ImportRun | undefined }>) {
  const { t } = useTranslation();
  const waiting = isWaitingToRetry(run);
  const now = useTick(!waiting);
  const states = stageStates(run?.stage, waiting);
  const currentIndex = states.indexOf("current");
  const started = run === undefined ? Number.NaN : Date.parse(run.createdAt);
  const elapsed =
    waiting || Number.isNaN(started) ? null : Math.max(0, now * 1000 - started);
  const currentLabel =
    currentIndex === -1
      ? t("imports.processing.waitingToStart")
      : PROCESSING_STAGES.filter((_, index) => states[index] === "current")
          .map((stage) => t(`imports.processing.stage.${stage.key}`))
          .join(" · ");
  const statusLine = waiting
    ? t("imports.processing.retryWaiting")
    : t("imports.processing.current", { stage: currentLabel });

  return (
    <div className="space-y-4">
      <ol className="space-y-2" aria-label={t("imports.processing.stagesLabel")}>
        {PROCESSING_STAGES.map((stage, index) => {
          const state = states[index] ?? "waiting";
          const Icon = ICONS[state];
          return (
            <li
              key={stage.key}
              aria-current={state === "current" ? "step" : undefined}
              className={cn(
                "flex items-center gap-2.5 text-sm",
                state === "waiting" && "text-muted-foreground",
                state === "current" && "font-medium",
              )}
            >
              <Icon className="size-4 shrink-0" aria-hidden="true" />
              <span>{t(`imports.processing.stage.${stage.key}`)}</span>
              <span className="sr-only">{t(`imports.processing.state.${state}`)}</span>
            </li>
          );
        })}
      </ol>
      <div className="text-muted-foreground flex flex-wrap items-center gap-x-4 gap-y-1 text-xs">
        <p role="status" aria-live="polite">
          {statusLine}
        </p>
        {elapsed === null ? null : (
          <p className="tabular-nums">
            {t("imports.processing.elapsed", { time: countdown(elapsed) })}
          </p>
        )}
        {run !== undefined && run.attempt >= 1 && (run.attempt > 1 || waiting) ? (
          <p>
            {t("imports.processing.attempt", {
              attempt: run.attempt,
              max: run.maxAttempts,
            })}
          </p>
        ) : null}
      </div>
    </div>
  );
}
