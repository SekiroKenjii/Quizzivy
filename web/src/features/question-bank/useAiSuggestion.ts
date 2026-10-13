import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { useQueryClient } from "@tanstack/react-query";
import type { AiUsage } from "@/features/ai/api";
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
  type AiBlock,
} from "@/features/ai/useAiAvailable";
import { notify } from "@/lib/toast";
import { aiGuardOf, type QuestionAi } from "./questionAi";
import type { QuestionValues } from "./questionSchema";

/** AiSuggestionStage is where one of the editor's suggestions is: not asked, being written, answered, or failed. */
export type AiSuggestionStage<R> =
  | { name: "idle" }
  | { name: "running" }
  | { name: "done"; result: R }
  | { name: "failed"; failure: AiFailure };

/** aiFailureText is the sentence a failed suggestion shows in its panel. */
export function aiFailureText(failure: AiFailure, t: TFunction): string {
  switch (failure.kind) {
    case "unavailable":
      return t("ai.unavailable");
    case "blocked":
      return t(AI_BLOCK_KEYS[failure.block]);
    case "error":
      return failure.message;
  }
}

/**
 * useAiSuggestion runs one of the question editor's inline suggestions.
 * `offered` is whether the assistant is drawn at all, and `block` why it
 * cannot be used now (the month's usage, or the last refusal). `ask` checks
 * the deck's two guards against `values` and toasts the first that fails, has
 * the page ready the stored question (`QuestionAi.prepare`) and toasts its
 * refusal, then calls; a newer ask, `dismiss` and unmounting abort an older
 * one. It writes nothing: what the teacher keeps goes through the editor's own
 * change handler.
 */
export function useAiSuggestion<R extends { usage: AiUsage }>(
  ai: QuestionAi,
  values: QuestionValues,
) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const offered = useAiAvailable();
  const usageBlock = useAiBlock(offered);
  const [stage, setStage] = useState<AiSuggestionStage<R>>({ name: "idle" });
  const run = useRef<AbortController | null>(null);

  useEffect(() => () => run.current?.abort(), []);

  const failedBlock: AiBlock | null =
    stage.name === "failed" && stage.failure.kind === "blocked"
      ? stage.failure.block
      : null;
  const block = failedBlock ?? usageBlock;

  async function ask(call: (id: string, signal: AbortSignal) => Promise<R>) {
    if (block !== null || stage.name === "running") return;
    const guard = aiGuardOf(values);
    if (guard !== null) {
      notify.info(t(guard));
      return;
    }
    run.current?.abort();
    const controller = new AbortController();
    run.current = controller;
    setStage({ name: "running" });
    const ready = await ai.prepare();
    if (controller.signal.aborted) return;
    if ("refusal" in ready) {
      setStage({ name: "idle" });
      notify.warning(ready.refusal);
      return;
    }
    try {
      const result = await call(ready.id, controller.signal);
      if (controller.signal.aborted) return;
      settleAiUsage(client, result.usage);
      setStage({ name: "done", result });
    } catch (cause) {
      if (controller.signal.aborted) return;
      const failure = aiFailureOf(cause, t("ai.failed"));
      refreshAfterAiFailure(client, cause, failure);
      setStage({ name: "failed", failure });
    }
  }

  function dismiss() {
    run.current?.abort();
    setStage({ name: "idle" });
  }

  return { offered, block, stage, ask, dismiss };
}
