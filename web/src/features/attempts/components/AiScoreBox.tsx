import { useEffect, useMemo, useRef, useState } from "react";
import { Trans, useTranslation } from "react-i18next";
import { useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router";
import { Check, LoaderCircle, Sparkles } from "lucide-react";
import { Callout } from "@/components/shared/Callout";
import { Button } from "@/components/ui/button";
import { aiPreferencesOf } from "@/features/auth/preferences";
import { suggestAiScore, type AiScoreSuggestion } from "@/features/ai/api";
import { AiCreditNote } from "@/features/ai/components/AiCreditNote";
import { credits } from "@/features/ai/credits";
import {
  aiFailureOf,
  refreshAfterAiFailure,
  settleAiUsage,
  type AiFailure,
} from "@/features/ai/outcome";
import {
  AI_BLOCK_KEYS,
  useAiAvailable,
  useAiBlock,
} from "@/features/ai/useAiAvailable";
import { ApiError } from "@/lib/api/errors";
import { useLocale } from "@/lib/i18n/useLocale";
import { notify } from "@/lib/toast";
import { useAuthStore } from "@/stores/auth";
import { cn } from "@/lib/utils";

type Stage =
  | { name: "idle"; failure: AiFailure | null }
  | { name: "running" }
  | { name: "done"; suggestion: AiScoreSuggestion };

const CONFIDENCE: Readonly<Record<AiScoreSuggestion["confidence"], string>> = {
  high: "bg-success-soft text-success-ink",
  medium: "bg-warning-soft text-warning-ink",
  low: "bg-danger-soft text-danger-ink",
};

const ACTION =
  "text-meta in-data-[scale=deck]:text-meta h-7.5 rounded-seg px-2.5 leading-normal font-medium";

/**
 * AiScoreBox is the grading card's "Suggest a score" (V-07) for one short
 * answer, drawn only while the assistant is offered (`useAiAvailable`). With
 * the teacher's "Let AI read student answers" off it is one line that says so
 * and links to Settings › AI assistant. A suggestion saves nothing: "Use score
 * and comment" hands both to `onUse`, which fills the card's own fields and
 * reports whether it took them, and the teacher's Save & next writes the
 * grade.
 */
export function AiScoreBox({
  attemptId,
  questionId,
  max,
  busy,
  onUse,
}: Readonly<{
  attemptId: string;
  questionId: string;
  max: number;
  busy: boolean;
  onUse: (points: number, comment: string) => boolean;
}>) {
  const available = useAiAvailable();
  const reads = useAuthStore(
    (state) => aiPreferencesOf(state.user?.preferences?.ai, "vi").readStudentAnswers,
  );
  const [refused, setRefused] = useState(false);
  if (!available) return null;
  if (!reads || refused) return <SuggestionsOff />;
  return (
    <ScoreSuggestion
      attemptId={attemptId}
      questionId={questionId}
      max={max}
      busy={busy}
      onUse={onUse}
      onRefused={() => setRefused(true)}
    />
  );
}

function SuggestionsOff() {
  return (
    <p className="text-muted-fg text-meta flex items-center gap-1.5 leading-normal">
      <Sparkles aria-hidden="true" className="text-brand-ink size-3.25 flex-none" />
      <span className="min-w-0">
        <Trans
          i18nKey="ai.score.off"
          components={{
            settings: (
              <Link
                to="/teacher/settings/ai"
                className="text-fg rounded-sm font-medium underline underline-offset-2"
              />
            ),
          }}
        />
      </span>
    </p>
  );
}

function ScoreSuggestion({
  attemptId,
  questionId,
  max,
  busy,
  onUse,
  onRefused,
}: Readonly<{
  attemptId: string;
  questionId: string;
  max: number;
  busy: boolean;
  onUse: (points: number, comment: string) => boolean;
  onRefused: () => void;
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const figure = useMemo(() => new Intl.NumberFormat(locale), [locale]);
  const client = useQueryClient();
  const block = useAiBlock(true);
  const [stage, setStage] = useState<Stage>({ name: "idle", failure: null });
  const run = useRef<AbortController | null>(null);

  useEffect(() => () => run.current?.abort(), []);

  const failedBlock =
    stage.name === "idle" && stage.failure?.kind === "blocked"
      ? stage.failure.block
      : null;
  const stop = failedBlock ?? block;

  async function suggest() {
    if (stop !== null) return;
    run.current?.abort();
    const controller = new AbortController();
    run.current = controller;
    setStage({ name: "running" });
    try {
      const suggestion = await suggestAiScore(attemptId, questionId, controller.signal);
      if (controller.signal.aborted) return;
      settleAiUsage(client, suggestion.usage);
      setStage({ name: "done", suggestion });
    } catch (cause) {
      if (controller.signal.aborted) return;
      if (cause instanceof ApiError && cause.code === "AI_READ_OFF") {
        onRefused();
        return;
      }
      const failure = aiFailureOf(cause, t("ai.failed"));
      refreshAfterAiFailure(client, cause, failure);
      setStage({ name: "idle", failure });
    }
  }

  function use(suggestion: AiScoreSuggestion) {
    if (onUse(suggestion.score, suggestion.comment))
      notify.success(t("ai.score.filled"));
  }

  const done = stage.name === "done" ? stage.suggestion : null;
  return (
    <section
      aria-label={t("ai.score.title")}
      className="bg-brand-soft flex flex-col gap-2.5 rounded-[10px] border px-3.5 py-3"
    >
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <h3 className="text-brand-ink text-meta flex items-center gap-1.5 leading-normal font-semibold whitespace-nowrap">
          <Sparkles aria-hidden="true" className="size-3.25" />
          {t("ai.score.title")}
        </h3>
        {stage.name === "idle" && (
          <>
            <p className="text-muted-fg text-meta min-w-0 flex-[1_1_220px] leading-normal [text-wrap:pretty]">
              {t("ai.score.idle")}
            </p>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className={cn(ACTION, "bg-card hover:bg-muted")}
              disabled={stop !== null}
              onClick={() => void suggest()}
            >
              {t("ai.score.run")}
            </Button>
          </>
        )}
        {stage.name === "running" && (
          <p
            role="status"
            className="text-muted-fg text-meta flex items-center gap-2 leading-normal"
          >
            <LoaderCircle
              aria-hidden="true"
              className="size-3.5 animate-spin motion-reduce:animate-none"
            />
            {t("ai.score.running")}
          </p>
        )}
        {done && (
          <p className="ml-auto flex items-center gap-2.5">
            <span
              className={cn(
                "inline-flex h-5.5 items-center rounded-full px-2 text-xs font-medium whitespace-nowrap",
                CONFIDENCE[done.confidence],
              )}
            >
              {t(`ai.score.confidence.${done.confidence}`)}
            </span>
            <span className="text-lg leading-normal font-semibold whitespace-nowrap tabular-nums">
              {figure.format(done.score)}
              <span className="text-muted-fg text-sm font-medium">
                {" "}
                {t("ai.score.outOf", { max: figure.format(max) })}
              </span>
            </span>
          </p>
        )}
      </div>
      {stage.name === "idle" && stop !== null && (
        <div className="flex">
          <AiCreditNote text={t(AI_BLOCK_KEYS[stop])} warning />
        </div>
      )}
      {stage.name === "idle" && stage.failure?.kind === "unavailable" && (
        <Callout
          tone="warning"
          size="md"
          announce
          actions={
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="bg-card"
              disabled={stop !== null}
              onClick={() => void suggest()}
            >
              {t("ai.tryAgain")}
            </Button>
          }
        >
          {t("ai.unavailable")}
        </Callout>
      )}
      {stage.name === "idle" && stage.failure?.kind === "error" && (
        <Callout tone="danger" size="md" announce>
          {stage.failure.message}
        </Callout>
      )}
      {done && (
        <>
          {done.reasons.length > 0 && (
            <ul className="m-0 flex list-disc flex-col gap-0.75 pl-4.5 text-sm leading-normal">
              {done.reasons.map((reason, index) => (
                <li key={index}>{reason}</li>
              ))}
            </ul>
          )}
          {done.comment.trim() !== "" && (
            <p className="bg-card rounded-md border px-2.5 py-2 text-sm leading-[1.55] [text-wrap:pretty]">
              <span className="text-muted-fg">{t("ai.score.comment")}</span>{" "}
              {done.comment}
            </p>
          )}
          <div className="flex flex-wrap items-center gap-1.5">
            <Button
              type="button"
              size="sm"
              className={cn(ACTION, "px-3")}
              disabled={busy}
              onClick={() => use(done)}
            >
              <Check aria-hidden="true" className="size-3.25" />
              {t("ai.score.use")}
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className={cn(ACTION, "bg-card hover:bg-muted")}
              disabled={block !== null}
              onClick={() => void suggest()}
            >
              {t("ai.tryAgain")}
            </Button>
            <AiCreditNote
              text={t("ai.credits.used", {
                credits: credits(done.usage.credits, locale),
                left: credits(done.usage.left, locale),
              })}
              warning={false}
            />
          </div>
        </>
      )}
    </section>
  );
}
