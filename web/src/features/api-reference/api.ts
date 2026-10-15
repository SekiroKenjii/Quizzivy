import { queryOptions } from "@tanstack/react-query";
import { api, downloadFile } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { saveDownloadedFile } from "@/lib/saveFile";

/** ApiReferenceSummary is what the server reports about the contract it embeds. */
export type ApiReferenceSummary = components["schemas"]["ApiReferenceSummary"];

/** API_REFERENCE_KEY is the query key of the summary. */
export const API_REFERENCE_KEY = ["api-reference"] as const;

/**
 * apiReferenceQuery reads `GET /admin/api-reference`: the contract's OpenAPI
 * and document versions, its operation count and the count under each tag. It
 * describes the running build, so it is kept for five minutes.
 */
export function apiReferenceQuery() {
  return queryOptions({
    queryKey: API_REFERENCE_KEY,
    queryFn: ({ signal }) => api("get", "/admin/api-reference", { signal }),
    staleTime: 5 * 60_000,
  });
}

/** openDocsSession sets the fifteen-minute cookie that opens the API reference on the API origin; admins only. */
export function openDocsSession() {
  return api("post", "/admin/docs-session");
}

/**
 * downloadOpenApi saves the contract as `quizzivy-openapi.json`, under the
 * name the server gives it.
 */
export async function downloadOpenApi() {
  saveDownloadedFile(
    await downloadFile("/admin/api-reference/openapi.json"),
    "quizzivy-openapi.json",
  );
}
