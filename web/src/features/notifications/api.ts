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
/**
 * NotificationAudience is the console a bell belongs to; with it the server
 * counts, lists and marks only that console's kinds (DG-170).
 */
export type NotificationAudience = components["schemas"]["NotificationAudience"];
/** MySummary supplies the shell's unread notification count. */
export type MySummary = components["schemas"]["MySummary"];

/**
 * notificationKeys groups list, summary and preference caches for settled
 * invalidation; the list and the summary are kept per audience, so the two
 * consoles' bells never share an entry.
 */
export const notificationKeys = {
  all: ["notifications"],
  list: (audience: NotificationAudience) =>
    ["notifications", "list", audience] as const,
  summary: (audience: NotificationAudience) =>
    ["notifications", "summary", audience] as const,
  preferences: ["notifications", "preferences"],
} as const;

/** listNotifications reads a cursor page of one audience's kinds, with optional bounds and cancellation. */
export function listNotifications(
  params: Readonly<{ audience: NotificationAudience; before?: string; limit?: number }>,
  signal?: AbortSignal,
): Promise<NotificationPage> {
  return api(
    "get",
    "/me/notifications",
    signal ? { query: params, signal } : { query: params },
  );
}

/**
 * markNotificationsRead marks one audience's notifications: all of them when
 * ids are absent; otherwise it skips an empty list and sends larger lists
 * sequentially in batches of at most one hundred, each with the audience.
 */
export async function markNotificationsRead(
  audience: NotificationAudience,
  ids?: readonly string[],
): Promise<void> {
  if (ids === undefined) {
    await api("post", "/me/notifications/read", { body: { audience } });
    return;
  }
  for (let start = 0; start < ids.length; start += 100) {
    await api("post", "/me/notifications/read", {
      body: { audience, ids: ids.slice(start, start + 100) },
    });
  }
}

/** getMySummary reads the caller's current unread count of one audience's kinds. */
export function getMySummary(
  audience: NotificationAudience,
  signal?: AbortSignal,
): Promise<MySummary> {
  const query = { audience };
  return api("get", "/me/summary", signal ? { query, signal } : { query });
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
