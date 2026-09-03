import { api, uploadFile, type UploadOptions } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";

export type MediaAsset = components["schemas"]["MediaAsset"];
export type MediaKind = components["schemas"]["MediaKind"];

/** A library row carries the usage count the list endpoint adds. */
/** The library row: a MediaAsset plus how many published versions use it. */
export type LibraryAsset = components["schemas"]["LibraryAsset"];

export interface ListMediaParams {
  kind?: MediaKind;
  page?: number;
  limit?: number;
}

export function listMedia(params: ListMediaParams = {}, signal?: AbortSignal) {
  const query: Record<string, unknown> = {};
  if (params.kind) query["kind"] = params.kind;
  if (params.page && params.page > 1) query["page"] = params.page;
  if (params.limit) query["limit"] = params.limit;
  return api("get", "/admin/media", signal ? { query, signal } : { query });
}

export function deleteMedia(id: string) {
  return api("delete", "/admin/media/{id}", { path: { id } });
}

export function uploadMedia(file: File, options?: UploadOptions): Promise<MediaAsset> {
  return uploadFile<MediaAsset>("/admin/media", file, options);
}
