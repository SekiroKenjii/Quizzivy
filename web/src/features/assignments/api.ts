import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";

export type Assignment = components["schemas"]["Assignment"];
export type AssignmentInput = components["schemas"]["AssignmentInput"];
export type AssignmentStatus = components["schemas"]["AssignmentStatus"];
export type ReviewPolicy = components["schemas"]["ReviewPolicy"];
export type IntegrityPolicy = components["schemas"]["IntegrityPolicy"];
export type Student = components["schemas"]["User"];

export interface ListAssignmentsParams {
  status?: AssignmentStatus;
  cursor?: string;
  limit?: number;
}

export function listAssignments(
  params: ListAssignmentsParams = {},
  signal?: AbortSignal,
) {
  const query: Record<string, unknown> = {};
  if (params.status) query["status"] = params.status;
  if (params.cursor) query["cursor"] = params.cursor;
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

export interface ListStudentsParams {
  q?: string;
  classId?: string;
  cursor?: string;
  limit?: number;
}

export function listStudents(params: ListStudentsParams = {}, signal?: AbortSignal) {
  const query: Record<string, unknown> = {};
  if (params.q) query["q"] = params.q;
  if (params.classId) query["classId"] = params.classId;
  if (params.cursor) query["cursor"] = params.cursor;
  if (params.limit) query["limit"] = params.limit;
  return api("get", "/admin/students", signal ? { query, signal } : { query });
}
