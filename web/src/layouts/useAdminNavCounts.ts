import { useQuery } from "@tanstack/react-query";
import { adminSummaryQuery } from "@/features/admin-overview/api";
import { adminCreditsQuery } from "@/features/ai-credits/api";
import { publicStatusQuery } from "@/features/ai/api";
import { useCan } from "@/features/auth/permissions";
import { useIdlePolling, useRefetchOnResume } from "@/hooks/useIdlePolling";
import type { NavFigures } from "@/layouts/shell/nav";

const POLL_MS = 60_000;

/**
 * useAdminNavCounts reads the figures of the Admin sidebar's badges, once a
 * minute while the user is there (`useIdlePolling`): the accounts that must
 * change their password, for a holder of `people.users.manage`, and the AI
 * credit requests waiting for an Admin, for a holder of `ai.credits.manage`
 * while the server has an assistant. A failed read draws no badge.
 */
export function useAdminNavCounts(): NavFigures | null {
  const manages = useCan("ai.credits.manage");
  const manageUsers = useCan("people.users.manage");
  const status = useQuery({
    ...publicStatusQuery(),
    select: (answer) => answer.ai,
    enabled: manages,
  });
  const enabled = manages && status.data === true;
  const refetchInterval = useIdlePolling(POLL_MS, enabled);
  const credits = useQuery({
    ...adminCreditsQuery(),
    select: (answer) => answer.requests.length,
    enabled,
    staleTime: POLL_MS,
    refetchInterval,
    refetchIntervalInBackground: false,
  });
  useRefetchOnResume(credits.refetch, enabled);
  const summaryInterval = useIdlePolling(POLL_MS, manageUsers);
  const summary = useQuery({
    ...adminSummaryQuery(),
    select: (answer) => answer.mustChangePassword,
    enabled: manageUsers,
    staleTime: POLL_MS,
    refetchInterval: summaryInterval,
    refetchIntervalInBackground: false,
  });
  useRefetchOnResume(summary.refetch, manageUsers);
  const figures: Record<string, number> = {};
  if (enabled && !credits.isError && credits.data !== undefined) {
    figures.creditRequests = credits.data;
  }
  if (manageUsers && !summary.isError && typeof summary.data === "number") {
    figures.mustChangePassword = summary.data;
  }
  return Object.keys(figures).length === 0 ? null : figures;
}
