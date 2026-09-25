import { useCallback, useRef, useState, type RefObject } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { ApiError } from "@/lib/api/errors";
import { commitWordImport, type ImportCommitResult } from "./api";
import { storeImport } from "./queries";

/** CommitState is the commit's visible lifecycle; "lost" means no answer arrived and a retry replays the same request. */
export type CommitState =
  | { phase: "idle" }
  | { phase: "pending" }
  | { phase: "unsaved" }
  | { phase: "lost" }
  | { phase: "refused"; code: string; message: string }
  | { phase: "done"; result: ImportCommitResult };

/**
 * useImportCommit flushes the autosave and creates the draft test from the
 * saved revision. One request ID is bound to each revision and reused after a
 * lost response. A call while a commit is pending does nothing.
 */
export function useImportCommit(
  importId: string,
  flush: () => Promise<void>,
  revision: RefObject<number>,
) {
  const client = useQueryClient();
  const [state, setState] = useState<CommitState>({ phase: "idle" });
  const inFlight = useRef(false);
  const attempt = useRef<{ requestId: string; revision: number } | null>(null);

  const commit = useCallback(async () => {
    if (inFlight.current) return;
    inFlight.current = true;
    setState({ phase: "pending" });
    try {
      try {
        await flush();
      } catch {
        setState({ phase: "unsaved" });
        return;
      }
      const draftRevision = revision.current;
      if (attempt.current?.revision !== draftRevision)
        attempt.current = { requestId: crypto.randomUUID(), revision: draftRevision };
      const result = await commitWordImport(importId, {
        requestId: attempt.current.requestId,
        draftRevision,
      });
      await storeImport(client, result.import);
      setState({ phase: "done", result });
    } catch (cause) {
      if (cause instanceof ApiError && cause.status >= 400 && cause.status < 500) {
        setState({ phase: "refused", code: cause.code, message: cause.message });
        if (cause.code === "IMPORT_CONFLICT")
          void client.invalidateQueries({ queryKey: ["word-import", importId] });
      } else {
        setState({ phase: "lost" });
      }
    } finally {
      inFlight.current = false;
    }
  }, [client, flush, importId, revision]);

  return { state, commit };
}
