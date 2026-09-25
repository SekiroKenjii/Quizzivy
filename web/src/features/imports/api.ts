import { api, uploadFile, type UploadOptions } from "@/lib/api/client";
import type { components, operations } from "@/lib/api/schema";

type Schemas = components["schemas"];

export type WordImport = Schemas["WordImport"];
export type ImportSource = Schemas["ImportSource"];
export type ImportSourceRole = Schemas["ImportSourceRole"];
export type ImportStatus = Schemas["ImportStatus"];
export type ImportRun = Schemas["ImportRun"];
export type ImportUploadReceipt = Schemas["ImportUploadReceipt"];
export type ImportLimits = Schemas["ImportLimits"];
export type ImportCapabilities = Schemas["ImportCapabilities"];
export type ImportReview = Schemas["ImportReview"];
export type ImportReviewSummary = Schemas["ImportReviewSummary"];
export type ImportDraft = Schemas["ImportDraft"];
export type ImportDraftSection = Schemas["ImportDraftSection"];
export type ImportDraftItem = Schemas["ImportDraftItem"];
export type ImportDraftGroup = Schemas["ImportDraftGroup"];
export type ImportDraftQuestion = Schemas["ImportDraftQuestion"];
export type ImportDraftOption = Schemas["ImportDraftOption"];
export type ImportDraftBlank = Schemas["ImportDraftBlank"];
export type ImportDraftAnswer = Schemas["ImportDraftAnswer"];
export type ImportKeyValue = Schemas["ImportKeyValue"];
export type ImportFieldOrigins = Schemas["ImportFieldOrigins"];
export type ImportOrigin = Schemas["ImportOrigin"];
export type ImportFinding = Schemas["ImportFinding"];
export type ImportSourceRef = Schemas["ImportSourceRef"];
export type ImportSourceView = Schemas["ImportSourceView"];
export type ImportSourceBlock = Schemas["ImportSourceBlock"];
export type ImportSourceSpan = Schemas["ImportSourceSpan"];
export type SaveImportReview = Schemas["SaveImportReview"];
export type ImportCommitResult = Schemas["ImportCommitResult"];
export type ImportHistoryFilter = NonNullable<
  operations["listWordImports"]["parameters"]["query"]
>;
export type ImportUploadIdentity =
  operations["uploadImportSource"]["parameters"]["query"];

export function createWordImport(
  body: Schemas["CreateWordImport"],
  signal?: AbortSignal,
) {
  return api("post", "/admin/imports", { body, ...(signal ? { signal } : {}) });
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

export function getWordImportCapabilities(signal?: AbortSignal) {
  return api("get", "/admin/imports/capabilities", signal ? { signal } : {});
}

export function getWordImportLimits(signal?: AbortSignal) {
  return api("get", "/admin/imports/limits", signal ? { signal } : {});
}

export function processWordImport(
  id: string,
  body: Schemas["ProcessWordImport"],
  signal?: AbortSignal,
) {
  return api("post", "/admin/imports/{id}/process", {
    path: { id },
    body,
    ...(signal ? { signal } : {}),
  });
}

export function cancelWordImport(id: string, body: Schemas["CancelWordImport"]) {
  return api("post", "/admin/imports/{id}/cancel", { path: { id }, body });
}

export function getWordImportReview(id: string, signal?: AbortSignal) {
  return api("get", "/admin/imports/{id}/review", {
    path: { id },
    ...(signal ? { signal } : {}),
  });
}

export function saveWordImportReview(id: string, body: SaveImportReview) {
  return api("put", "/admin/imports/{id}/review", { path: { id }, body });
}

export function adoptWordImportReprocessed(
  id: string,
  body: Schemas["CancelWordImport"],
) {
  return api("post", "/admin/imports/{id}/review/adopt", { path: { id }, body });
}

export function getWordImportSource(
  id: string,
  role: ImportSourceRole,
  signal?: AbortSignal,
) {
  return api("get", "/admin/imports/{id}/source", {
    path: { id },
    query: { role },
    ...(signal ? { signal } : {}),
  });
}

export function commitWordImport(id: string, body: Schemas["CommitWordImport"]) {
  return api("post", "/admin/imports/{id}/commit", { path: { id }, body });
}
