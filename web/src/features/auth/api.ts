import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";

export type User = components["schemas"]["User"];

/**
 * The session calls, in one place so the store and the pages agree on what a
 * session is. Everything here is typed against `api/openapi.yaml`: a contract
 * change that breaks one of these fails `tsc` rather than a request at runtime.
 */

/** §5.4: called on every app load to decide between the app and `/login`. */
export function fetchCurrentUser(signal?: AbortSignal): Promise<User> {
  // `exactOptionalPropertyTypes` is on: passing `signal: undefined` is not the
  // same as omitting it, so the property is only added when there is one.
  return api("get", "/auth/me", signal ? { signal } : {});
}

export function login(email: string, password: string) {
  return api("post", "/auth/login", { body: { email, password } });
}

/**
 * §5.4. The refresh cookie is what identifies the family to revoke, and it
 * reaches the endpoint on its own -- `Path=/auth`, and the client sends
 * credentials. Nothing is passed here because nothing needs to be.
 */
export function logout() {
  return api("post", "/auth/logout");
}

export function changePassword(currentPassword: string, newPassword: string) {
  return api("post", "/auth/change-password", {
    body: { currentPassword, newPassword },
  });
}
