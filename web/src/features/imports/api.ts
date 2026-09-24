import { api, uploadFile, type UploadOptions } from "@/lib/api/client";
import type { components, operations } from "@/lib/api/schema";

export type WordImport = components["schemas"]["WordImport"];
export type ImportSource = components["schemas"]["ImportSource"];
export type ImportStatus = components["schemas"]["ImportStatus"];
export type ImportUploadReceipt = components["schemas"]["ImportUploadReceipt"];
export type ImportHistoryFilter = NonNullable<
  operations["listWordImports"]["parameters"]["query"]
>;
export type ImportUploadIdentity =
  operations["uploadImportSource"]["parameters"]["query"];

export function createWordImport(body: components["schemas"]["CreateWordImport"]) {
  return api("post", "/admin/imports", { body });
}

export function listWordImports(query: ImportHistoryFilter = {}, signal?: AbortSignal) {
  return api("get", "/admin/imports", { query, ...(signal ? { signal } : {}) });
}

export function getWordImport(id: string, signal?: AbortSignal) {
  return api("get", "/admin/imports/{id}", {
    path: { id },
    ...(signal ? { signal } : {}),
  });
}

export function uploadImportSource(
  id: string,
  file: File,
  identity: ImportUploadIdentity,
  options?: UploadOptions,
) {
  const query = new URLSearchParams({
    role: identity.role,
    uploadId: identity.uploadId,
    expectedRevision: String(identity.expectedRevision),
  });
  return uploadFile<ImportUploadReceipt>(
    `/admin/imports/${encodeURIComponent(id)}/sources?${query.toString()}`,
    file,
    options,
  );
}

export function downloadImportSource(id: string, sourceId: string) {
  return api("get", "/admin/imports/{id}/sources/{sourceId}/download", {
    path: { id, sourceId },
  });
}
