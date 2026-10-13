import {
  queryOptions,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { api } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { components } from "@/lib/api/schema";

/** AiMonthUsage is the caller's AI month, as `GET /teacher/ai/usage` answers it. */
export type AiMonthUsage = components["schemas"]["AiMonthUsage"];

/** AiCreditRequest is one request for more credits, as the server records it. */
export type AiCreditRequest = components["schemas"]["AiCreditRequest"];

/** AiCreditRequestInput is the body of `POST /teacher/ai/credit-requests`. */
export type AiCreditRequestInput = components["schemas"]["AiCreditRequestInput"];

/** AiCreditCategory is one row of the usage card. */
export type AiCreditCategory = components["schemas"]["AiCreditCategory"];

/** AI_USAGE_KEY is the query key of the caller's AI month. */
export const AI_USAGE_KEY = ["ai-usage"] as const;

/** PUBLIC_STATUS_KEY is the query key of `GET /public/status`. */
export const PUBLIC_STATUS_KEY = ["public-status"] as const;

/** aiUsageQuery reads the caller's AI month. */
export function aiUsageQuery() {
  return queryOptions({
    queryKey: AI_USAGE_KEY,
    queryFn: ({ signal }) => api("get", "/teacher/ai/usage", { signal }),
  });
}

/** requestAiCredits asks the caller's admin for more credits this month. */
export function requestAiCredits(body: AiCreditRequestInput): Promise<AiCreditRequest> {
  return api("post", "/teacher/ai/credit-requests", { body });
}

const aiOf = (status: components["schemas"]["PublicStatus"]) => status.ai;

/**
 * useAiConfigured reads whether the server has an AI assistant, from
 * `GET /public/status`, while `enabled`. The route lets a browser keep the
 * answer for thirty seconds, and so does the query.
 */
export function useAiConfigured(enabled: boolean) {
  return useQuery({
    queryKey: PUBLIC_STATUS_KEY,
    queryFn: ({ signal }) => api("get", "/public/status", { signal }),
    select: aiOf,
    staleTime: 30_000,
    enabled,
  });
}

/** isAiOff reports whether a failure is the server's `501 AI_OFF`, the assistant not configured. */
export function isAiOff(cause: unknown): boolean {
  return cause instanceof ApiError && (cause.code === "AI_OFF" || cause.status === 501);
}

/**
 * useRequestAiCredits sends a request for more credits. A recorded request
 * becomes the month's pending one in the cached usage; a 409 (a request
 * already waiting, or a centre that takes none) rereads the usage, which
 * then says so.
 */
export function useRequestAiCredits() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: requestAiCredits,
    onSuccess: (request) => {
      client.setQueryData<AiMonthUsage>(AI_USAGE_KEY, (usage) =>
        usage === undefined ? usage : { ...usage, pending: request },
      );
    },
    onError: (cause) => {
      if (cause instanceof ApiError && cause.status === 409)
        void client.invalidateQueries({ queryKey: AI_USAGE_KEY });
    },
  });
}
