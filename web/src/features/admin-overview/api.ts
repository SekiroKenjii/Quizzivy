import { queryOptions } from "@tanstack/react-query";
import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";

/** AdminOverview is the Admin console's Overview as the server sends it. */
export type AdminOverview = components["schemas"]["AdminOverview"];
/** AdminAttention is one thing that needs doing. */
export type AdminAttention = components["schemas"]["AdminAttention"];
/** AdminDailyActive is who was active on each of the last fourteen days. */
export type AdminDailyActive = components["schemas"]["AdminDailyActive"];

/** ADMIN_OVERVIEW_KEY is the key of the Overview read, for invalidation. */
export const ADMIN_OVERVIEW_KEY = ["admin", "overview"] as const;

/** ADMIN_SUMMARY_KEY is the key of the sidebar's counts. */
export const ADMIN_SUMMARY_KEY = ["admin", "summary"] as const;

/** adminOverviewQuery reads `GET /admin/overview`. */
export function adminOverviewQuery() {
  return queryOptions({
    queryKey: ADMIN_OVERVIEW_KEY,
    queryFn: ({ signal }) => api("get", "/admin/overview", { signal }),
    refetchOnMount: "always",
  });
}

/** adminSummaryQuery reads `GET /admin/summary`, the one count the sidebar draws. */
export function adminSummaryQuery() {
  return queryOptions({
    queryKey: ADMIN_SUMMARY_KEY,
    queryFn: ({ signal }) => api("get", "/admin/summary", { signal }),
  });
}
