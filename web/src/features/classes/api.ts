import { queryOptions } from "@tanstack/react-query";
import { api } from "@/lib/api/client";
import type { components, operations } from "@/lib/api/schema";

export type Class = components["schemas"]["Class"];
/** ClassListItem is a row of the teacher's class list, its join code readable. */
export type ClassListItem = components["schemas"]["ClassListItem"];
export type MyClass = components["schemas"]["MyClass"];
export type ClassMember = components["schemas"]["ClassMember"];
export type ClassFacets = components["schemas"]["ClassFacets"];
export type ClassStatus = "active" | "joinable" | "archived" | "all";

export interface ListClassesParams {
  q?: string;
  page?: number;
  limit?: number;
  /** Absent means active: pickers never see an archived class (G-08). */
  status?: ClassStatus;
  /** Only the screen that shows join codes asks; a picker never receives one. */
  withCodes?: boolean;
}

export function fetchClasses(params: ListClassesParams = {}, signal?: AbortSignal) {
  const query: Record<string, unknown> = {};
  if (params.q) query["q"] = params.q;
  if (params.page && params.page > 1) query["page"] = params.page;
  if (params.limit) query["limit"] = params.limit;
  if (params.status) query["status"] = params.status;
  if (params.withCodes) query["withCodes"] = true;
  return api("get", "/teacher/classes", signal ? { query, signal } : { query });
}

/** NewClass is the body `createClass` sends. */
export type NewClass =
  operations["createClass"]["requestBody"]["content"]["application/json"];

export function createClass(body: NewClass) {
  return api("post", "/teacher/classes", { body });
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
    "/teacher/classes/{id}",
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
    "/teacher/classes/{id}/members",
    signal ? { path: { id }, query, signal } : { path: { id }, query },
  );
}

export interface ClassEdit {
  name?: string;
  description?: string;
  scheduleLabel?: string | null;
  room?: string | null;
  selfJoinEnabled?: boolean;
  archived?: boolean;
}

export function updateClass(id: string, body: ClassEdit) {
  return api("patch", "/teacher/classes/{id}", { path: { id }, body });
}

/** G-06's expiry and use cap; an absent field means the server's own default, never "unlimited". */
export type JoinCodeOptions = NonNullable<
  operations["rotateJoinCode"]["requestBody"]
>["content"]["application/json"];

/** rotateJoinCode issues a new code and returns it in full; the code is never cached or stored (§13.3). */
export function rotateJoinCode(id: string, options: JoinCodeOptions = {}) {
  return api("post", "/teacher/classes/{id}/join-code", {
    path: { id },
    body: options,
  });
}

export function revokeJoinCode(id: string) {
  return api("delete", "/teacher/classes/{id}/join-code", { path: { id } });
}

export function removeMember(id: string, userId: string) {
  return api("delete", "/teacher/classes/{id}/members/{userId}", {
    path: { id, userId },
  });
}

export function addMember(classId: string, userId: string) {
  return api("post", "/teacher/classes/{id}/members", {
    path: { id: classId },
    body: { userId },
  });
}

/** §9's /app/classes. Never carries a join code; the server blanks it. */
export function fetchMyClasses(signal?: AbortSignal) {
  return api("get", "/app/classes", signal ? { signal } : {});
}

/**
 * myClassesQuery is the student's class list as every screen reads it: Home,
 * Classes and the Join dialog share the one key, so a join refreshes them all.
 */
export const myClassesQuery = queryOptions({
  queryKey: ["my-classes"],
  queryFn: ({ signal }) => fetchMyClasses(signal),
});

export function deleteClass(id: string) {
  return api("delete", "/teacher/classes/{id}", { path: { id } });
}
