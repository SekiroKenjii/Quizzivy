import { useMutation, useQueryClient } from "@tanstack/react-query";
import { dashboardKeys } from "@/features/dashboard/keys";
import { markNotificationsRead, notificationKeys } from "./api";

/**
 * useMarkNotificationsRead invalidates every notification query and the
 * teacher shell's summary, which carries its unread count, after either
 * outcome, without optimistic cache writes. The invalidation runs even when
 * the component that marked has unmounted.
 */
export function useMarkNotificationsRead() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: markNotificationsRead,
    onSettled: () =>
      Promise.all([
        client.invalidateQueries({ queryKey: notificationKeys.all }),
        client.invalidateQueries({ queryKey: dashboardKeys.summary }),
      ]),
  });
}
