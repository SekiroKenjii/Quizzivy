import { api } from "@/lib/api/client";
import type { components, operations } from "@/lib/api/schema";

/** Notification is one caller-owned notice from the contract. */
export type Notification = components["schemas"]["Notification"];
/** NotificationKind names the nine supported notification sentences. */
export type NotificationKind = components["schemas"]["NotificationKind"];
/** NotificationEvent names one stored preference switch. */
export type NotificationEvent = components["schemas"]["NotificationEvent"];
/** NotificationParams carries plain values for a notification sentence. */
export type NotificationParams = components["schemas"]["NotificationParams"];
/** NotificationTarget names a local destination without supplying a URL. */
export type NotificationTarget = components["schemas"]["NotificationTarget"];
/** NotificationPreference stores both channels for one switch. */
export type NotificationPreference = components["schemas"]["NotificationPreference"];
/** NotificationPage is the list operation's cursor envelope. */
export type NotificationPage =
  operations["listNotifications"]["responses"][200]["content"]["application/json"];
/** MySummary supplies the shell's unread notification count. */
export type MySummary = components["schemas"]["MySummary"];

/** notificationKeys groups list, summary and preference caches for settled invalidation. */
export const notificationKeys = {
  all: ["notifications"],
  list: ["notifications", "list"],
  summary: ["notifications", "summary"],
  preferences: ["notifications", "preferences"],
} as const;

/** listNotifications reads a cursor page with optional bounds and cancellation. */
export function listNotifications(
  params: Readonly<{ before?: string; limit?: number }> = {},
  signal?: AbortSignal,
): Promise<NotificationPage> {
  return api(
    "get",
    "/me/notifications",
    signal ? { query: params, signal } : { query: params },
  );
}

/** markNotificationsRead marks all when ids are absent, skips an empty list, and sends larger lists sequentially in batches of at most one hundred. */
export async function markNotificationsRead(ids?: readonly string[]): Promise<void> {
  if (ids === undefined) {
    await api("post", "/me/notifications/read", { body: {} });
    return;
  }
  for (let start = 0; start < ids.length; start += 100) {
    await api("post", "/me/notifications/read", {
      body: { ids: ids.slice(start, start + 100) },
    });
  }
}

/** getMySummary reads the caller's current unread count. */
export function getMySummary(signal?: AbortSignal): Promise<MySummary> {
  return api("get", "/me/summary", signal ? { signal } : {});
}

/** getNotificationPreferences reads every switch in contract order. */
export function getNotificationPreferences(
  signal?: AbortSignal,
): Promise<NotificationPreference[]> {
  return api("get", "/me/notification-preferences", signal ? { signal } : {});
}

/** updateNotificationPreferences replaces all five switches and returns the server's canonical rows. */
export function updateNotificationPreferences(
  all: readonly NotificationPreference[],
): Promise<NotificationPreference[]> {
  return api("put", "/me/notification-preferences", { body: [...all] });
}
