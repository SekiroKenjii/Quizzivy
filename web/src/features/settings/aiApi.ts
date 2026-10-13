import { useMutation, useQueryClient } from "@tanstack/react-query";
import { AI_USAGE_KEY, requestAiCredits, type AiMonthUsage } from "@/features/ai/api";
import { ApiError } from "@/lib/api/errors";

/**
 * useRequestAiCredits sends a request for more credits. A recorded request
 * becomes the month's pending one in the shared cached usage; a 409 (a
 * request already waiting, or a centre that takes none) rereads the usage,
 * which then says so.
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
