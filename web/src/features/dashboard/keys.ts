import type { DashboardRange } from "./view";

/** dashboardKeys preserves dashboard-prefix invalidation with distinct summary and range readings. */
export const dashboardKeys = {
  summary: ["teacher-dashboard", "summary"] as const,
  home: (range: DashboardRange) => ["teacher-dashboard", "home", range] as const,
  live: ["teacher-assignments", "dashboard", "open"] as const,
};
