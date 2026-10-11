import { api, uploadFile } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import type { User } from "@/features/auth/api";

/** Session is one live sign-in of the caller, as `GET /auth/sessions` lists it. */
export type Session = components["schemas"]["Session"];

/** SESSIONS_KEY is the query key of the caller's signed-in devices. */
export const SESSIONS_KEY = ["auth-sessions"] as const;

/** AVATAR_MAX_BYTES is the largest photo `PUT /me/avatar` accepts: 2 MiB. */
export const AVATAR_MAX_BYTES = 2 * 1024 * 1024;

/** AVATAR_TYPES are the image types `PUT /me/avatar` accepts. */
export const AVATAR_TYPES = ["image/png", "image/jpeg"] as const;

/** listSessions returns the caller's live sessions, the calling one first. */
export async function listSessions(signal?: AbortSignal): Promise<Session[]> {
  const page = await api("get", "/auth/sessions", signal ? { signal } : {});
  return page.items;
}

/**
 * revokeSession ends one of the caller's other sessions. It moves the
 * caller's session epoch, so the next request answers 401 once and recovers
 * through the client's single-flight refresh.
 */
export function revokeSession(familyId: string): Promise<void> {
  return api("delete", "/auth/sessions/{familyId}", { path: { familyId } });
}

/** revokeOtherSessions ends every session but the calling one and says how many ended. */
export async function revokeOtherSessions(): Promise<number> {
  const result = await api("post", "/auth/sessions/revoke-others");
  return result.revoked;
}

/** setAvatar uploads a new profile photo and returns the caller with its address. */
export function setAvatar(file: File): Promise<User> {
  return uploadFile<User>("/me/avatar", file, { method: "PUT" });
}

/** deleteAvatar clears the profile photo and returns the caller without one. */
export function deleteAvatar(): Promise<User> {
  return api("delete", "/me/avatar");
}
