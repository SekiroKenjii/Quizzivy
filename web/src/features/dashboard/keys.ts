import type { DashboardRange } from "./view";

/** dashboardKeys preserves dashboard-prefix invalidation with distinct summary and range readings. */
export const dashboardKeys = {
  summary: ["admin-dashboard", "summary"] as const,
  home: (range: DashboardRange) => ["admin-dashboard", "home", range] as const,
  live: ["admin-assignments", "dashboard", "open"] as const,
};
