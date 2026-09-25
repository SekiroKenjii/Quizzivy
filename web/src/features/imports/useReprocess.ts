import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { useQueryClient } from "@tanstack/react-query";
import { failureMessage } from "@/lib/api/errors";
import { getWordImport, processWordImport, type WordImport } from "./api";
import { refreshAvailability } from "./availability";
import { storeImport } from "./queries";
import { isActiveStatus } from "./status";

/**
 * useReprocess processes an import under review again with the answer-key
 * paper the teacher chose, after flushing the autosave. One request ID is kept
 * per paper and revision, and a retry that finds the import already processing
 * treats the earlier request as accepted. On success the cached review is
 * marked stale and the teacher is taken to the progress page. Unmounting
 * aborts a reprocess in progress without navigating.
 */
export function useReprocess(importId: string, flush: () => Promise<void>) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const navigate = useNavigate();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const running = useRef<AbortController | null>(null);
  const attempt = useRef<{ requestId: string; paper: number; revision: number } | null>(
    null,
  );

  useEffect(() => () => running.current?.abort(), []);

  const reprocess = useCallback(
    async (paper: number) => {
      if (running.current !== null && !running.current.signal.aborted) return;
      const controller = new AbortController();
      running.current = controller;
      const { signal } = controller;
      setPending(true);
      setError(null);
      const leave = async (next: WordImport) => {
        await storeImport(client, next);
        void client.invalidateQueries({
          queryKey: ["word-import-review", importId],
          refetchType: "none",
        });
        if (!signal.aborted) void navigate(`/admin/imports/${importId}`);
      };
      try {
        try {
          await flush();
        } catch {
          setError(t("imports.review.saveBeforeReprocess"));
          return;
        }
        const fresh = await getWordImport(importId, signal);
        if (isActiveStatus(fresh.status)) {
          await leave(fresh);
          return;
        }
        const previous = attempt.current;
        const current =
          previous?.paper === paper && previous.revision === fresh.revision
            ? previous
            : { requestId: crypto.randomUUID(), paper, revision: fresh.revision };
        attempt.current = current;
        const started = await processWordImport(
          importId,
          {
            requestId: current.requestId,
            expectedRevision: current.revision,
            keyPaper: paper,
          },
          signal,
        );
        await leave(started);
      } catch (cause) {
        if (signal.aborted) return;
        setError(failureMessage(cause, t("imports.review.reprocessFailed")));
        refreshAvailability(client, cause);
        void client.invalidateQueries({ queryKey: ["word-import", importId] });
      } finally {
        running.current = null;
        if (!signal.aborted) setPending(false);
      }
    },
    [client, flush, importId, navigate, t],
  );

  return { reprocess, pending, error };
}
