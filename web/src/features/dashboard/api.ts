import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";

export type Dashboard = components["schemas"]["Dashboard"];
export type Assignment = components["schemas"]["Assignment"];
export type AssignmentStatus = components["schemas"]["AssignmentStatus"];

export function getDashboard(signal?: AbortSignal) {
  return api("get", "/admin/dashboard", signal ? { signal } : {});
}

export function listAssignments(
  params: { status?: AssignmentStatus; limit?: number } = {},
  signal?: AbortSignal,
) {
  const query: Record<string, unknown> = {};
  if (params.status) query["status"] = params.status;
  if (params.limit) query["limit"] = params.limit;
  return api("get", "/admin/assignments", signal ? { query, signal } : { query });
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
