import { useId } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  AI_STATUS_KEY,
  AI_USAGE_KEY,
  explainAssignmentResults,
  type AiInsightsResult,
  type AiMonthUsage,
} from "@/features/ai/api";
import type { AiBlock } from "@/features/ai/useAiAvailable";
import { ApiError, failureMessage } from "@/lib/api/errors";

/**
 * InsightsFailure is why the last explanation did not come back: the
 * assistant did not answer, the teacher is blocked, reading answers is off,
 * nothing is handed in yet, or the server's own message.
 */
export type InsightsFailure =
  | { kind: "unavailable" }
  | { kind: "blocked"; block: AiBlock }
  | { kind: "readOff" }
  | { kind: "nothingHandedIn" }
  | { kind: "error"; message: string };

/**
 * AssignmentInsights is the state of one assignment's "Explain results with
 * AI" for the visit: whether it is reading, what it answered or why it
 * failed, `run` and `hide`, and the id of the button that opened it, which
 * `hide` focuses again.
 */
export interface AssignmentInsights {
  running: boolean;
  result: AiInsightsResult | null;
  failure: InsightsFailure | null;
  run: () => void;
  hide: () => void;
  openerId: string;
}

function failureOf(cause: unknown, fallback: string): InsightsFailure {
  if (!(cause instanceof ApiError)) return { kind: "unavailable" };
  if (cause.code === "AI_CREDITS_EXHAUSTED")
    return {
      kind: "blocked",
      block: cause.details?.["scope"] === "pool" ? "pool" : "teacher",
    };
  if (cause.code === "AI_PAUSED") return { kind: "blocked", block: "paused" };
  if (cause.code === "AI_READ_OFF") return { kind: "readOff" };
  if (cause.status === 422) return { kind: "nothingHandedIn" };
  if (cause.code === "AI_UNAVAILABLE" || cause.status === 0 || cause.status >= 500)
    return { kind: "unavailable" };
  return { kind: "error", message: failureMessage(cause, fallback) };
}

/**
 * useAssignmentInsights holds "Explain results with AI" for the assignment
 * `assignmentId` while the page stays open, so a tab change keeps what was
 * read. The call writes nothing; its answer's `usage` updates the teacher's
 * cached month, a refusal for credits reads the month again, and `AI_OFF`
 * reads the status again so the entry point goes.
 */
export function useAssignmentInsights(assignmentId: string): AssignmentInsights {
  const { t } = useTranslation();
  const client = useQueryClient();
  const openerId = useId();
  const explain = useMutation({
    mutationFn: (id: string) => explainAssignmentResults(id),
    onSuccess: ({ usage }) =>
      client.setQueryData<AiMonthUsage>(AI_USAGE_KEY, (month) =>
        month
          ? { ...month, used: month.used + usage.credits, left: usage.left }
          : month,
      ),
    onError: (cause) => {
      if (!(cause instanceof ApiError)) return;
      if (cause.code === "AI_CREDITS_EXHAUSTED" || cause.code === "AI_PAUSED")
        void client.invalidateQueries({ queryKey: AI_USAGE_KEY });
      if (cause.code === "AI_OFF")
        void client.invalidateQueries({ queryKey: AI_STATUS_KEY });
    },
  });
  const current = explain.variables === assignmentId;
  return {
    running: current && explain.isPending,
    result: current && explain.isSuccess ? explain.data : null,
    failure:
      current && explain.isError ? failureOf(explain.error, t("ai.failed")) : null,
    run: () => explain.mutate(assignmentId),
    hide: () => {
      explain.reset();
      document.getElementById(openerId)?.focus();
    },
    openerId,
  };
}
