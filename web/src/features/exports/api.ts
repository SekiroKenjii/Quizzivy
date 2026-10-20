import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";

/** DataExport is one request for a copy of the centre's data, as the server reports it. */
export type DataExport = components["schemas"]["DataExport"];

/** DataExportDownload is the single-use address a ready export is fetched from. */
export type DataExportDownload = components["schemas"]["DataExportDownload"];

/** EXPORT_POLL_MS is how often a queued or running export is read again. */
export const EXPORT_POLL_MS = 5000;

/** MAX_DOWNLOADS is how many times a ready export may be signed for download. */
export const MAX_DOWNLOADS = 3;

/** dataExportKey is the query key of `GET /admin/data-exports/{id}`. */
export function dataExportKey(id: string) {
  return ["data-export", id] as const;
}

/** dataExportQuery reads `GET /admin/data-exports/{id}`; another user's export answers 404. */
export function dataExportQuery(id: string) {
  return queryOptions({
    queryKey: dataExportKey(id),
    queryFn: ({ signal }) =>
      api("get", "/admin/data-exports/{id}", { path: { id }, signal }),
    retry: false,
  });
}

/**
 * useRequestDataExport asks for an export of the centre's data. The link is
 * sent to the requester's own address, never one the caller names.
 */
export function useRequestDataExport() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (includeAnswers: boolean) =>
      api("post", "/admin/data-exports", { body: { includeAnswers } }),
    onSuccess: (created) => {
      client.setQueryData(dataExportKey(created.id), created);
    },
  });
}

/**
 * downloadDataExport asks for the address of a ready export. The address is
 * valid for a minute and counts against the export's three fetches.
 */
export function downloadDataExport(id: string) {
  return api("post", "/admin/data-exports/{id}/download", { path: { id } });
}
