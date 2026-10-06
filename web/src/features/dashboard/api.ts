import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import type { DashboardRange } from "./view";

export type Dashboard = components["schemas"]["Dashboard"];
export type Assignment = components["schemas"]["Assignment"];
export type AssignmentStatus = components["schemas"]["AssignmentStatus"];

/** getDashboard preserves the legacy default request and optionally selects the submission range. */
export function getDashboard(signal?: AbortSignal, range?: DashboardRange) {
  const query = range === undefined ? {} : { range };
  return api("get", "/teacher/dashboard", signal ? { query, signal } : { query });
}

export function listAssignments(
  params: { status?: AssignmentStatus; limit?: number } = {},
  signal?: AbortSignal,
) {
  const query: Record<string, unknown> = {};
  if (params.status) query["status"] = params.status;
  if (params.limit) query["limit"] = params.limit;
  return api("get", "/teacher/assignments", signal ? { query, signal } : { query });
}

/** listDashboardAssignments filters each status on the server before pagination. */
export async function listDashboardAssignments(signal?: AbortSignal) {
  const pages = await Promise.all(
    ["open", "scheduled"].map((status) =>
      listAssignments({ status: status as AssignmentStatus, limit: 10 }, signal),
    ),
  );
  return {
    items: pages
      .flatMap((page) => page.items)
      .sort((a, b) => a.window.closesAt.localeCompare(b.window.closesAt))
      .slice(0, 10),
  };
}

/** TeacherSummary supplies nullable permission-aware navigation counts. */
export type TeacherSummary = components["schemas"]["TeacherSummary"];

/** getTeacherSummary reads the own-teaching navigation figures without fetching the home payload. */
export function getTeacherSummary(signal?: AbortSignal) {
  return api("get", "/teacher/summary", signal ? { signal } : {});
}
