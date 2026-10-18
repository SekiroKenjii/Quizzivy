import { keepPreviousData, queryOptions } from "@tanstack/react-query";
import { api, downloadFile } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { saveDownloadedFile } from "@/lib/saveFile";

/** AuditEntry is one row of the audit log as the server words it. */
export type AuditEntry = components["schemas"]["AuditEntry"];
/** AuditCategory is one of the six families the log is filtered by. */
export type AuditCategory = components["schemas"]["AuditCategory"];
/** AuditFacets counts the entries each category holds for the current search and range. */
export type AuditFacets = components["schemas"]["AuditCategoryFacets"];
/** AuditRange is how far back the list reaches. */
export type AuditRange = "today" | "7d" | "30d" | "all" | "custom";
/** AuditPeriod is how far back an export reaches. */
export type AuditPeriod = "7d" | "30d" | "year";
/** AuditFormat is a file an export is written as. */
export type AuditFormat = "csv" | "json";

/** AuditFilters is what the screen asks the list for, as the address holds it. */
export type AuditFilters = Readonly<{
  category: AuditCategory | null;
  q: string;
  range: AuditRange;
  from: string | null;
  to: string | null;
}>;

/** AUDIT_PAGE_SIZE is the rows a page holds, as the deck draws it: no size control. */
export const AUDIT_PAGE_SIZE = 10;

/** AUDIT_KEY is the prefix of every audit read, for invalidation. */
export const AUDIT_KEY = ["audit-log"] as const;

/** auditLogQuery reads one page of the log under `filters`, keeping the page on screen while the next one is read. */
export function auditLogQuery(filters: AuditFilters, page: number) {
  const { category, q, range, from, to } = filters;
  return queryOptions({
    queryKey: [...AUDIT_KEY, { category, q, range, from, to, page }],
    queryFn: ({ signal }) =>
      api("get", "/admin/audit-log", {
        signal,
        query: {
          page,
          limit: AUDIT_PAGE_SIZE,
          range,
          ...(category === null ? {} : { category }),
          ...(q === "" ? {} : { q }),
          ...(range === "custom" && from !== null && to !== null ? { from, to } : {}),
        },
      }),
    placeholderData: keepPreviousData,
  });
}

/** AuditExport is what an export asks the server for: a period, a file format, and optionally the category and search to narrow by. */
export type AuditExport = Readonly<{
  period: AuditPeriod;
  format: AuditFormat;
  category: AuditCategory | null;
  q: string;
}>;

/**
 * exportAuditLog downloads the entries of a period as one file, under the
 * name the server gives it, and hands it to the browser to save. A period
 * over 100,000 entries is refused with `EXPORT_TOO_LARGE`. An aborted `signal`
 * rejects it with an `AbortError` and saves nothing.
 */
export async function exportAuditLog(request: AuditExport, signal?: AbortSignal) {
  const file = await downloadFile(
    "/admin/audit-log/export",
    {
      period: request.period,
      format: request.format,
      ...(request.category === null ? {} : { category: request.category }),
      ...(request.q === "" ? {} : { q: request.q }),
    },
    signal,
  );
  saveDownloadedFile(file, `audit-log.${request.format}`);
}
