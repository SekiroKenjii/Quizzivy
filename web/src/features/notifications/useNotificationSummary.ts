import { useQuery } from "@tanstack/react-query";
import { useIdlePolling, useRefetchOnResume } from "@/hooks/useIdlePolling";
import { getMySummary, notificationKeys } from "./api";

/** SUMMARY_POLL_MS is the active reader's sixty-second notification interval. */
export const SUMMARY_POLL_MS = 60_000;

/**
 * useNotificationSummary returns the unread count of `/me/summary`, polled
 * idle-aware with an immediate read on resume. It keeps the last count while
 * a later read is in flight, so a bell's dot does not blink on each poll, and
 * is zero while disabled, before the first read answers, and after a read
 * fails.
 */
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
    unread: enabled && query.isSuccess ? query.data.unreadNotifications : 0,
  };
}
