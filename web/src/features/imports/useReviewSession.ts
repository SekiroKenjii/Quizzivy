import { useCallback, useRef, useState, type RefObject } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useAutosave } from "@/features/tests/useAutosave";
import { ApiError } from "@/lib/api/errors";
import {
  saveWordImportReview,
  type ImportDraftQuestion,
  type ImportReview,
} from "./api";
import { updateQuestion, workingFrom, type ReviewWorking } from "./draft";

function superseded(cause: unknown): boolean {
  return (
    cause instanceof ApiError &&
    (cause.code === "STALE_WRITE" || cause.code === "IMPORT_CONFLICT")
  );
}

/**
 * useReviewSession holds the review's local draft and saves it through the
 * shared autosave, one write at a time, each sending the revision the previous
 * save returned. STALE_WRITE or IMPORT_CONFLICT stops saving and refreshes the
 * import. While `locked`, edits are ignored. Findings, summary and readiness
 * are the server's answer for the last saved revision.
 */
export function useReviewSession(
  importId: string,
  initial: ImportReview,
  locked: RefObject<boolean>,
) {
  const client = useQueryClient();
  const [working, setWorking] = useState<ReviewWorking>(() => workingFrom(initial));
  const [server, setServer] = useState<ImportReview>(initial);
  const latest = useRef<ReviewWorking>(working);
  const revision = useRef(initial.revision);

  const save = useCallback(
    async (value: ReviewWorking) => {
      const saved = await saveWordImportReview(importId, {
        expectedRevision: revision.current,
        title: value.title,
        sections: value.sections,
        acknowledged: value.acknowledged,
      }).catch((cause: unknown) => {
        if (superseded(cause))
          void client.invalidateQueries({ queryKey: ["word-import", importId] });
        throw cause;
      });
      revision.current = saved.revision;
      setServer(saved);
      client.setQueryData(["word-import-review", importId], saved);
    },
    [client, importId],
  );
  const autosave = useAutosave<ReviewWorking>({ save, isStale: superseded });
  const { schedule } = autosave;

  const apply = useCallback(
    (next: ReviewWorking) => {
      if (locked.current) return;
      latest.current = next;
      setWorking(next);
      schedule(next);
    },
    [locked, schedule],
  );

  const editQuestion = useCallback(
    (
      questionId: string,
      update: (question: ImportDraftQuestion) => ImportDraftQuestion,
    ) =>
      apply({
        ...latest.current,
        sections: updateQuestion(latest.current.sections, questionId, update),
      }),
    [apply],
  );

  const setTitle = useCallback(
    (title: string) => apply({ ...latest.current, title }),
    [apply],
  );

  const acknowledge = useCallback(
    (findingId: string, on: boolean) => {
      const next = new Set(latest.current.acknowledged);
      if (on) next.add(findingId);
      else next.delete(findingId);
      apply({ ...latest.current, acknowledged: [...next] });
    },
    [apply],
  );

  return {
    working,
    server,
    revision,
    autosave,
    editQuestion,
    setTitle,
    acknowledge,
  };
}
