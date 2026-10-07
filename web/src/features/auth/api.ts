import type { AccountTransition } from "@/lib/api/authTransition";
import { api } from "@/lib/api/client";
import type { components, operations } from "@/lib/api/schema";

export type User = components["schemas"]["CurrentUser"];

/**
 * The session calls, in one place so the store and the pages agree on what a
 * session is. Everything here is typed against `api/openapi.yaml`: a contract
 * change that breaks one of these fails `tsc` rather than a request at runtime.
 */

/** §5.4: called on every app load to decide between the app and `/login`. */
export function fetchCurrentUser(signal?: AbortSignal): Promise<User> {
  return api("get", "/auth/me", signal ? { signal } : {});
}

/** updateProfile sends a typed profile patch while preserving the existing legal-name shorthand. */
export function updateProfile(input: string | ProfilePatch) {
  return api("patch", "/auth/me", {
    body: typeof input === "string" ? { fullName: input } : input,
  });
}

export function login(email: string, password: string, transition?: AccountTransition) {
  return api("post", "/auth/login", {
    body: { email, password },
    ...(transition ? { transition } : {}),
  });
}

/**
 * §5.4. The refresh cookie is what identifies the family to revoke, and it
 * reaches the endpoint on its own -- `Path=/auth`, and the client sends
 * credentials. Nothing is passed here because nothing needs to be.
 */
export function logout(transition?: AccountTransition) {
  return api("post", "/auth/logout", transition ? { transition } : {});
}

/**
 * Omits `currentPassword` when it is blank rather than sending "".
 *
 * The contract makes it optional exactly while `mustChangePassword` is set, and
 * an empty string is not the same request as an absent field: it would be
 * compared against the stored hash and fail.
 */
export function changePassword(currentPassword: string, newPassword: string) {
  return api("post", "/auth/change-password", {
    body: {
      newPassword,
      ...(currentPassword === "" ? {} : { currentPassword }),
    },
  });
}

/** openDocsSession sets the fifteen-minute cookie that opens the API reference on the API origin; admins only. */
export function openDocsSession() {
  return api("post", "/admin/docs-session");
}

/** ProfilePatch preserves omission and explicit nullable clearing for all five account fields. */
export type ProfilePatch =
  operations["updateCurrentUser"]["requestBody"]["content"]["application/json"];

/** PreferencesPatch contains only the preference keys deliberately changed by the caller. */
export type PreferencesPatch = components["schemas"]["UserPreferences"];

/** updatePreferences returns the complete merged stored preferences acknowledged by the server. */
export function updatePreferences(body: PreferencesPatch) {
  return api("patch", "/me/preferences", { body });
}
