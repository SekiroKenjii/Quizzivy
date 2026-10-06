import { useQuery } from "@tanstack/react-query";
import { useWorkspace } from "@/features/auth/permissions";
import { getTeacherSummary } from "@/features/dashboard/api";
import { dashboardKeys } from "@/features/dashboard/keys";
import { useIdlePolling, useRefetchOnResume } from "@/hooks/useIdlePolling";

/** NavCounts preserves unavailable figures as null rather than claiming no work. */
export interface NavCounts {
  liveAssignments: number | null;
  toGrade: number | null;
}

const POLL_MS = 60_000;

/** useNavCounts reads compatible summary figures while the Teacher workspace is active and hides failed badges. */
export function useNavCounts(): NavCounts | null {
  const enabled = useWorkspace("teacher");
  const refetchInterval = useIdlePolling(POLL_MS, enabled);
  const query = useQuery({
    queryKey: dashboardKeys.summary,
    queryFn: ({ signal }) => getTeacherSummary(signal),
    enabled,
    staleTime: POLL_MS,
    refetchInterval,
    refetchIntervalInBackground: false,
  });
  useRefetchOnResume(query.refetch, enabled);
  if (!enabled || query.isError || query.data === undefined) return null;
  return {
    liveAssignments: query.data.liveAssignments,
    toGrade: query.data.answersToGrade,
  };
}
