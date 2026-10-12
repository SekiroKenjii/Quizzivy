import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  getNotificationPreferences,
  notificationKeys,
  updateNotificationPreferences,
} from "./api";

/** useNotificationPreferences reads all five switches and saves the server's canonical answer in their cache. */
export function useNotificationPreferences() {
  const client = useQueryClient();
  const query = useQuery({
    queryKey: notificationKeys.preferences,
    queryFn: ({ signal }) => getNotificationPreferences(signal),
  });
  const save = useMutation({
    mutationFn: updateNotificationPreferences,
    onSuccess: (rows) => client.setQueryData(notificationKeys.preferences, rows),
  });
  return { query, save };
}
