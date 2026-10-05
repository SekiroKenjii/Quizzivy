import { useQuery } from "@tanstack/react-query";
import { getDashboard } from "@/features/dashboard/api";
import { useIdlePolling, useRefetchOnResume } from "@/hooks/useIdlePolling";

/**
 * NavCounts are the two figures the teacher sidebar draws beside a
 * destination: the caller's live assignments and the papers waiting for a
 * mark.
 */
export interface NavCounts {
  liveAssignments: number;
  toGrade: number;
}

const POLL_MS = 60_000;

/**
 * useNavCounts reads the sidebar's two figures, and answers null while they
 * load and whenever the last request failed. It shares the dashboard's query
 * key, so a write that invalidates the dashboard refreshes the sidebar too.
 * It asks again every minute while the tab is visible and the user is
 * active, and once when the user comes back.
 */
export function useNavCounts(): NavCounts | null {
  const refetchInterval = useIdlePolling(POLL_MS);
  const query = useQuery({
    queryKey: ["admin-dashboard"],
    queryFn: ({ signal }) => getDashboard(signal),
    staleTime: POLL_MS,
    refetchInterval,
    refetchIntervalInBackground: false,
  });
  useRefetchOnResume(query.refetch);
  if (query.isError || query.data === undefined) return null;
  return {
    liveAssignments: query.data.openAssignments,
    toGrade: query.data.awaitingGrading,
  };
}
