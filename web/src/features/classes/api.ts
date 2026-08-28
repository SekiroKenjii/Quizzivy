import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";

export type Class = components["schemas"]["Class"];
export type ClassMember = components["schemas"]["ClassMember"];

export function fetchClass(id: string, signal?: AbortSignal): Promise<Class> {
  return api(
    "get",
    "/admin/classes/{id}",
    signal ? { path: { id }, signal } : { path: { id } },
  );
}

export function fetchMembers(id: string, signal?: AbortSignal) {
  return api(
    "get",
    "/admin/classes/{id}/members",
    signal ? { path: { id }, signal } : { path: { id } },
  );
}

/**
 * The ONE call that ever returns a plaintext code (§13.3). Its result is held
 * in component state and never written anywhere -- not to the query cache,
 * which persists across navigations, and not to storage.
 */
export function rotateJoinCode(id: string) {
  return api("post", "/admin/classes/{id}/join-code", { path: { id }, body: {} });
}

export function revokeJoinCode(id: string) {
  return api("delete", "/admin/classes/{id}/join-code", { path: { id } });
}

export function removeMember(id: string, userId: string) {
  return api("delete", "/admin/classes/{id}/members/{userId}", {
    path: { id, userId },
  });
}
