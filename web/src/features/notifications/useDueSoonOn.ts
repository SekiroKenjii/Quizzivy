import { useQuery } from "@tanstack/react-query";
import { getNotificationPreferences, notificationKeys } from "./api";

/** useDueSoonOn reports whether the caller's "Test due soon" switch is on, which also governs the notice that a test opened; it is false while the switches are unread, when they cannot be read, and when the switch is off. */
export function useDueSoonOn(): boolean {
  const { data } = useQuery({
    queryKey: notificationKeys.preferences,
    queryFn: ({ signal }) => getNotificationPreferences(signal),
    select: (rows) => rows.find((row) => row.event === "assignment.due_soon")?.inApp,
  });
  return data === true;
}
