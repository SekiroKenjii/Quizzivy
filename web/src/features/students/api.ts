import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";

export type Student = components["schemas"]["StudentRow"];
export type StudentClass = components["schemas"]["StudentClass"];
export type StudentStats = components["schemas"]["StudentStats"];
export type StudentFacets = components["schemas"]["StudentFacets"];

export type StudentStatus = "active" | "disabled" | "all";

export interface ListStudentsParams {
  q?: string;
  classId?: string;
  status?: StudentStatus;
  page?: number;
  limit?: number;
}

export function listStudents(params: ListStudentsParams = {}, signal?: AbortSignal) {
  const query: Record<string, unknown> = {};
  if (params.q) query["q"] = params.q;
  if (params.classId) query["classId"] = params.classId;
  if (params.status) query["status"] = params.status;
  if (params.page && params.page > 1) query["page"] = params.page;
  if (params.limit) query["limit"] = params.limit;
  return api("get", "/admin/students", signal ? { query, signal } : { query });
}

export function getStudent(id: string, signal?: AbortSignal) {
  return api(
    "get",
    "/admin/students/{id}",
    signal ? { path: { id }, signal } : { path: { id } },
  );
}

export function createStudent(body: {
  email: string;
  fullName: string;
  classIds?: string[];
}) {
  return api("post", "/admin/students", { body });
}

export function updateStudent(
  id: string,
  body: { fullName?: string; email?: string; disabled?: boolean },
) {
  return api("patch", "/admin/students/{id}", { path: { id }, body });
}

export function resetStudentPassword(id: string) {
  return api("post", "/admin/students/{id}/reset-password", { path: { id } });
}

/**
 * The percentage G-07 prints, from the (earned, total) pair the server sends.
 *
 * Weighted by construction: the server already summed both sides across the
 * student's best graded attempts, so a 50-point midterm carries five times the
 * weight of a 10-point quiz. Null when nothing is graded — which is the em
 * dash, not a zero.
 */
export function scorePercent(stats: StudentStats): number | null {
  const score = stats.score;
  if (!score || score.total <= 0) return null;
  return Math.round((score.earned / score.total) * 100);
}
