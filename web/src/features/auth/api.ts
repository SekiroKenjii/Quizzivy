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
