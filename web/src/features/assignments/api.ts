import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";

export type Assignment = components["schemas"]["Assignment"];
export type AssignmentInput = components["schemas"]["AssignmentInput"];
export type AssignmentStatus = components["schemas"]["AssignmentStatus"];
export type ReviewPolicy = components["schemas"]["ReviewPolicy"];
export type IntegrityPolicy = components["schemas"]["IntegrityPolicy"];

export interface ListAssignmentsParams {
  status?: AssignmentStatus;
  page?: number;
  limit?: number;
}

export function listAssignments(
  params: ListAssignmentsParams = {},
  signal?: AbortSignal,
) {
  const query: Record<string, unknown> = {};
  if (params.status) query["status"] = params.status;
  if (params.page && params.page > 1) query["page"] = params.page;
  if (params.limit) query["limit"] = params.limit;
  return api("get", "/admin/assignments", signal ? { query, signal } : { query });
}

export function getAssignment(id: string, signal?: AbortSignal) {
  return api(
    "get",
    "/admin/assignments/{id}",
    signal ? { path: { id }, signal } : { path: { id } },
  );
}

export function createAssignment(body: AssignmentInput) {
  return api("post", "/admin/assignments", { body });
}

export function updateAssignment(id: string, body: AssignmentInput) {
  return api("patch", "/admin/assignments/{id}", { path: { id }, body });
}

export type AssignmentFacets = components["schemas"]["AssignmentStatusFacets"];

/** G-09's "Gia hạn cho tất cả": a later close, and the early close lifted. */
export function reopenAssignment(
  id: string,
  body: { closesAt: string; reason: string },
) {
  return api("post", "/admin/assignments/{id}/reopen", { path: { id }, body });
}

export type StudentAssignmentCard = components["schemas"]["StudentAssignmentCard"];
export type StudentAssignmentDetail = components["schemas"]["StudentAssignmentDetail"];

export function listMyAssignments(signal?: AbortSignal) {
  return api("get", "/app/assignments", signal ? { signal } : {});
}

export function getMyAssignment(id: string, signal?: AbortSignal) {
  return api(
    "get",
    "/app/assignments/{id}",
    signal ? { path: { id }, signal } : { path: { id } },
  );
}
