import type { QueryClient } from "@tanstack/react-query";
import { ApiError, failureMessage } from "@/lib/api/errors";
import { AI_STATUS_KEY, AI_USAGE_KEY, type AiMonthUsage, type AiUsage } from "./api";
import type { AiBlock } from "./useAiAvailable";

/**
 * AiFailure is how a failed AI call is drawn: the provider did not answer
 * (the warning callout and "Try again"), the teacher cannot use the
 * assistant now (the block sentence), or the server's own message.
 */
export type AiFailure =
  | { kind: "unavailable" }
  | { kind: "blocked"; block: AiBlock }
  | { kind: "error"; message: string };

/**
 * aiFailureOf reads a failed AI call as an AiFailure. A dropped connection, a
 * 5xx and a failure that is not the server's are all "unavailable"; `402`
 * names whose credits ran out by `details.scope`; anything else shows the
 * server's message, or `fallback` without one.
 */
export function aiFailureOf(cause: unknown, fallback: string): AiFailure {
  if (!(cause instanceof ApiError)) return { kind: "unavailable" };
  if (cause.code === "AI_CREDITS_EXHAUSTED")
    return {
      kind: "blocked",
      block: cause.details?.["scope"] === "pool" ? "pool" : "teacher",
    };
  if (cause.code === "AI_PAUSED") return { kind: "blocked", block: "paused" };
  if (cause.code === "AI_UNAVAILABLE" || cause.status === 0 || cause.status >= 500)
    return { kind: "unavailable" };
  return { kind: "error", message: failureMessage(cause, fallback) };
}

/**
 * refreshAfterAiFailure reads again what a failure says is stale: the month
 * of usage after a block, and the status after `501 AI_OFF`, so the entry
 * points go.
 */
export function refreshAfterAiFailure(
  client: QueryClient,
  cause: unknown,
  failure: AiFailure,
): void {
  if (failure.kind === "blocked")
    void client.invalidateQueries({ queryKey: AI_USAGE_KEY });
  if (cause instanceof ApiError && cause.code === "AI_OFF")
    void client.invalidateQueries({ queryKey: AI_STATUS_KEY });
}

/** settleAiUsage lays what one call cost over the cached month of usage. */
export function settleAiUsage(client: QueryClient, usage: AiUsage): void {
  client.setQueryData<AiMonthUsage>(AI_USAGE_KEY, (month) =>
    month ? { ...month, used: month.used + usage.credits, left: usage.left } : month,
  );
}
