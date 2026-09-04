import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";

export type Class = components["schemas"]["Class"];
export type ClassMember = components["schemas"]["ClassMember"];
export type ClassFacets = components["schemas"]["ClassFacets"];
export type ClassStatus = "active" | "joinable" | "archived" | "all";

export interface ListClassesParams {
  q?: string;
  page?: number;
  limit?: number;
  /** Absent means active: pickers never see an archived class (G-08). */
  status?: ClassStatus;
}

export function fetchClasses(params: ListClassesParams = {}, signal?: AbortSignal) {
  const query: Record<string, unknown> = {};
  if (params.q) query["q"] = params.q;
  if (params.page && params.page > 1) query["page"] = params.page;
  if (params.limit) query["limit"] = params.limit;
  if (params.status) query["status"] = params.status;
  return api("get", "/admin/classes", signal ? { query, signal } : { query });
}

export function createClass(body: {
  name: string;
  description: string | null;
  selfJoinEnabled: boolean;
}) {
  return api("post", "/admin/classes", { body });
}

/** G-08's "Đang mở" badge: live, self-join on, and a code that still admits. */
export function isJoinOpen(klass: Class, now = Date.now()): boolean {
  const code = klass.joinCode;
  return (
    klass.archivedAt === null &&
    klass.selfJoinEnabled &&
    code != null &&
    Date.parse(code.expiresAt) > now &&
    (code.maxUses === null || code.usesCount < code.maxUses)
  );
}

export function fetchClass(id: string, signal?: AbortSignal): Promise<Class> {
  return api(
    "get",
    "/admin/classes/{id}",
    signal ? { path: { id }, signal } : { path: { id } },
  );
}

export function fetchMembers(
  id: string,
  params: ListClassesParams = {},
  signal?: AbortSignal,
) {
  const query: Record<string, unknown> = {};
  if (params.q) query["q"] = params.q;
  if (params.page && params.page > 1) query["page"] = params.page;
  if (params.limit) query["limit"] = params.limit;
  return api(
    "get",
    "/admin/classes/{id}/members",
    signal ? { path: { id }, query, signal } : { path: { id }, query },
  );
}

/**
 * The ONE call that ever returns a plaintext code (§13.3). Its result is held
 * in component state and never written anywhere -- not to the query cache,
 * which persists across navigations, and not to storage.
 */
export interface ClassEdit {
  name?: string;
  description?: string;
  selfJoinEnabled?: boolean;
  archived?: boolean;
}

export function updateClass(id: string, body: ClassEdit) {
  return api("patch", "/admin/classes/{id}", { path: { id }, body });
}

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

export function addMember(classId: string, userId: string) {
  return api("post", "/admin/classes/{id}/members", {
    path: { id: classId },
    body: { userId },
  });
}

/** §9's /app/classes. Never carries a join code; the server blanks it. */
export function fetchMyClasses(signal?: AbortSignal) {
  return api("get", "/app/classes", signal ? { signal } : {});
}
