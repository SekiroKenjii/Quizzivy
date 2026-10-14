import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api/client";
import { publicStatusQuery } from "@/features/ai/api";
import { useCan } from "@/features/auth/permissions";
import { useIdlePolling, useRefetchOnResume } from "@/hooks/useIdlePolling";
import type { NavFigures } from "@/layouts/shell/nav";

const POLL_MS = 60_000;

/** ADMIN_CREDITS_KEY is the query key of the Admin's month of AI credits, read for the sidebar's badge. */
export const ADMIN_CREDITS_KEY = ["admin-ai-credits"] as const;

/**
 * useAdminNavCounts reads the figures of the Admin sidebar's badges: the AI
 * credit requests waiting for an Admin, once a minute while the user is
 * there (`useIdlePolling`), for a holder of `ai.credits.manage` while the
 * server has an assistant. A failed read draws no badge.
 */
export function useAdminNavCounts(): NavFigures | null {
  const manages = useCan("ai.credits.manage");
  const status = useQuery({
    ...publicStatusQuery(),
    select: (answer) => answer.ai,
    enabled: manages,
  });
  const enabled = manages && status.data === true;
  const refetchInterval = useIdlePolling(POLL_MS, enabled);
  const credits = useQuery({
    queryKey: ADMIN_CREDITS_KEY,
    queryFn: ({ signal }) => api("get", "/admin/ai/credits", { signal }),
    select: (answer) => answer.requests.length,
    enabled,
    staleTime: POLL_MS,
    refetchInterval,
    refetchIntervalInBackground: false,
  });
  useRefetchOnResume(credits.refetch, enabled);
  if (!enabled || credits.isError || credits.data === undefined) return null;
  return { creditRequests: credits.data };
}
