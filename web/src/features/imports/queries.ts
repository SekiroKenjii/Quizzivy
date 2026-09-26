import { queryOptions, type QueryClient } from "@tanstack/react-query";
import {
  getWordImportCapabilities,
  getWordImportSource,
  type ImportSourceRole,
  type WordImport,
} from "./api";

/** importCapabilitiesQuery reads what this deployment can do with Word imports; it changes only with a redeploy. */
export function importCapabilitiesQuery() {
  return queryOptions({
    queryKey: ["word-import-capabilities"],
    queryFn: ({ signal }) => getWordImportCapabilities(signal),
    staleTime: 5 * 60_000,
  });
}

/** sourceViewQuery reads the extracted text of one source role; it never goes stale for a given source revision. */
export function sourceViewQuery(
  importId: string,
  role: ImportSourceRole,
  sourceRevision: number,
) {
  return queryOptions({
    queryKey: ["word-import-source", importId, role, sourceRevision],
    queryFn: ({ signal }) => getWordImportSource(importId, role, signal),
    staleTime: Infinity,
  });
}

/**
 * storeImport records an import a mutation returned: it cancels reads of that
 * import still in flight, stores the new value, and marks the history stale.
 */
export async function storeImport(client: QueryClient, next: WordImport) {
  await client.cancelQueries({ queryKey: ["word-import", next.id] });
  client.setQueryData(["word-import", next.id], next);
  void client.invalidateQueries({ queryKey: ["word-imports"] });
}
