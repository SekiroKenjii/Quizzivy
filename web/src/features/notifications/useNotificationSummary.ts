import { useQuery } from "@tanstack/react-query";
import { useIdlePolling, useRefetchOnResume } from "@/hooks/useIdlePolling";
import { getMySummary, notificationKeys } from "./api";

/** SUMMARY_POLL_MS is the active reader's sixty-second notification interval. */
export const SUMMARY_POLL_MS = 60_000;

/** useNotificationSummary returns zero unless an enabled read has a known successful count, with idle-aware polling and immediate resume. */
export function useNotificationSummary(enabled = true): { unread: number } {
  const refetchInterval = useIdlePolling(SUMMARY_POLL_MS, enabled);
  const query = useQuery({
    queryKey: notificationKeys.summary,
    queryFn: ({ signal }) => getMySummary(signal),
    enabled,
    refetchInterval,
    refetchIntervalInBackground: false,
  });
  useRefetchOnResume(query.refetch, enabled);
  return {
    unread:
      enabled && query.isSuccess && !query.isFetching
        ? query.data.unreadNotifications
        : 0,
  };
}
