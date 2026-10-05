import { useMutation, useQueryClient } from "@tanstack/react-query";
import { markNotificationsRead, notificationKeys } from "./api";

/** useMarkNotificationsRead invalidates every notification query after either outcome without optimistic cache writes. */
export function useMarkNotificationsRead() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: markNotificationsRead,
    onSettled: () => client.invalidateQueries({ queryKey: notificationKeys.all }),
  });
}
