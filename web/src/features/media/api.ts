import { api, uploadFile, type UploadOptions } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";

export type MediaAsset = components["schemas"]["MediaAsset"];
export type MediaKind = components["schemas"]["MediaKind"];

/** The library row: a MediaAsset plus its name, play limit, size and usage. */
export type LibraryAsset = components["schemas"]["LibraryAsset"];

/** MediaReplacement is what replaceMedia answers: the new file and the rows it reached. */
export type MediaReplacement = components["schemas"]["MediaReplacement"];

/** MediaUsage is the bytes a library holds, by kind, beside the owner's quota. */
export type MediaUsage = components["schemas"]["MediaUsage"];

/** MediaUpdate is the body of updateMedia: a new name, a new default play limit, or both. */
export type MediaUpdate = components["schemas"]["MediaUpdate"];

/** ListMediaParams narrows listMedia: a kind, the unused files, a search, a page. */
export interface ListMediaParams {
  kind?: MediaKind;
  unused?: boolean;
  q?: string;
  page?: number;
  limit?: number;
}

/** listMedia reads a page of the caller's library with its facets and storage usage. */
export function listMedia(params: ListMediaParams = {}, signal?: AbortSignal) {
  const query: Record<string, unknown> = {};
  if (params.kind) query["kind"] = params.kind;
  if (params.unused) query["unused"] = true;
  if (params.q) query["q"] = params.q;
  if (params.page && params.page > 1) query["page"] = params.page;
  if (params.limit) query["limit"] = params.limit;
  return api("get", "/teacher/media", signal ? { query, signal } : { query });
}

/** deleteMedia removes a file from the library; a file in use is refused with 409. */
export function deleteMedia(id: string) {
  return api("delete", "/teacher/media/{id}", { path: { id } });
}

/** updateMedia renames a library file or sets its default play limit. */
export function updateMedia(id: string, body: MediaUpdate) {
  return api("patch", "/teacher/media/{id}", { path: { id }, body });
}

/** UploadMediaOptions adds the default play limit an audio upload stores. */
export interface UploadMediaOptions extends UploadOptions {
  defaultMaxPlays?: number | undefined;
}

/** uploadMedia adds a file to the caller's library, reporting progress as it sends. */
export function uploadMedia(
  file: File,
  { defaultMaxPlays, ...options }: UploadMediaOptions = {},
): Promise<MediaAsset> {
  const path =
    defaultMaxPlays === undefined
      ? "/teacher/media"
      : `/teacher/media?${new URLSearchParams({ defaultMaxPlays: String(defaultMaxPlays) }).toString()}`;
  return uploadFile<MediaAsset>(path, file, options);
}

/**
 * replaceMedia replaces a library file with one of the same kind. It is not
 * idempotent: a failure may still have committed, so a caller re-reads the
 * library rather than retrying.
 */
export function replaceMedia(
  id: string,
  file: File,
  options?: UploadOptions,
): Promise<MediaReplacement> {
  return uploadFile<MediaReplacement>(
    `/teacher/media/${encodeURIComponent(id)}/replace`,
    file,
    options,
  );
}
