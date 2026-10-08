import { useTranslation } from "react-i18next";
import { Check, LoaderCircle } from "lucide-react";
import { Card } from "@/components/ui/card";
import { useTick } from "@/hooks/useTick";
import { countdown } from "@/lib/i18n/datetime";
import { cn } from "@/lib/utils";
import type { ImportRun, ImportSource } from "../api";
import { isWaitingToRetry, PROCESSING_STAGES, stageStates } from "../status";

/** ProcessingPanel shows actual run progress and completed text-source receipts without a percentage. */
export function ProcessingPanel({
  run,
  exam,
}: Readonly<{ run: ImportRun | undefined; exam?: ImportSource | undefined }>) {
  const { t } = useTranslation();
  const text = exam?.role === "exam" && exam.format === "text";
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
          .map((stage) =>
            t(
              text && stage.key === "validate"
                ? "imports.processing.textCheck"
                : `imports.processing.stage.${stage.key}`,
            ),
          )
          .join(" · ");
  const statusLine = waiting
    ? t("imports.processing.retryWaiting")
    : t("imports.processing.current", { stage: currentLabel });

  return (
    <Card role="status" tabIndex={0} className="group gap-0 overflow-hidden py-0">
      <ol
        className="divide-border divide-y"
        aria-label={t("imports.processing.stagesLabel")}
      >
        {PROCESSING_STAGES.map((stage, index) => {
          const state = states[index] ?? "waiting";
          return (
            <li
              key={stage.key}
              aria-current={state === "current" ? "step" : undefined}
              className={cn(
                "flex flex-wrap items-center gap-3 px-4.5 py-3.25 text-sm",
                state === "waiting" && "text-muted-foreground",
                state === "current" && "font-medium",
              )}
            >
              <span
                className={cn(
                  "grid size-6 shrink-0 place-items-center rounded-full",
                  state === "done" && "bg-success-soft text-success-ink",
                  state === "waiting" && "bg-muted text-muted-fg",
                )}
              >
                {state === "done" ? (
                  <Check className="size-3.5" aria-hidden="true" />
                ) : null}
                {state === "current" ? (
                  <LoaderCircle
                    className="size-4 group-focus-within:[animation-play-state:paused] group-hover:[animation-play-state:paused] motion-safe:animate-spin motion-safe:[animation-duration:800ms]"
                    aria-hidden="true"
                  />
                ) : null}
                {state === "waiting" ? (
                  <span className="text-xs" aria-hidden="true">
                    {index + 1}
                  </span>
                ) : null}
              </span>
              <span className="min-w-0 flex-1">
                {t(
                  text && stage.key === "validate"
                    ? "imports.processing.textCheck"
                    : `imports.processing.stage.${stage.key}`,
                )}
              </span>
              {text &&
              state === "done" &&
              stage.key === "validate" &&
              exam.characters !== undefined ? (
                <span className="text-muted-fg text-right text-xs">
                  {t("imports.processing.charactersChecked", {
                    count: exam.characters,
                  })}
                </span>
              ) : null}
              {text && state === "done" && stage.key === "read" ? (
                <span className="text-muted-fg text-right text-xs">
                  {t("imports.processing.plainText")}
                </span>
              ) : null}
              <span className="sr-only">{t(`imports.processing.state.${state}`)}</span>
            </li>
          );
        })}
      </ol>
      <div className="text-muted-fg flex flex-wrap items-center gap-x-4 gap-y-1 border-t px-4.5 py-3 text-xs">
        <p role="status" aria-live="polite">
          {statusLine}
        </p>
        {elapsed === null ? null : (
          <p className="tabular-nums">
            {t("imports.processing.elapsed", { time: countdown(elapsed) })}
          </p>
        )}
        {run !== undefined && run.attempt >= 1 ? (
          <p>
            {t("imports.processing.attempt", {
              attempt: run.attempt,
              max: run.maxAttempts,
            })}
          </p>
        ) : null}
      </div>
    </Card>
  );
}
