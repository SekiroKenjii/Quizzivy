import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { ApiError } from "@/lib/api/errors";
import type { DraftScope } from "@/lib/drafts/store";
import { useAutosave } from "@/features/tests/useAutosave";
import { groupIssue } from "./model";
import type { GroupBundle, StoredGroup } from "./api";
import type { GroupRecovery } from "./recovery";

interface PendingEdit {
  bundle: GroupBundle;
  token: string;
  createdAt: number;
}
interface EditorOptions {
  stored: StoredGroup;
  recovered: GroupRecovery | null;
  createdAt?: number;
  scope: DraftScope | null;
  save: (
    bundle: GroupBundle,
    revision: number,
    testUpdatedAt: string | null | undefined,
  ) => Promise<StoredGroup>;
}

/** useGroupEditor serializes server writes and keeps unacknowledged edits in an account-scoped local outbox. */
export function useGroupEditor({
  stored,
  recovered,
  createdAt,
  scope,
  save,
}: EditorOptions) {
  const { t } = useTranslation();
  const [bundle, setBundle] = useState(recovered?.bundle ?? stored.bundle);
  const [local, setLocal] = useState<"pending" | "stored" | "unavailable" | "empty">(
    recovered ? "stored" : "empty",
  );
  const [dirty, setDirty] = useState(recovered !== null);
  const base = useRef({
    revision: recovered?.revision ?? stored.revision,
    testUpdatedAt: recovered?.testUpdatedAt ?? stored.testUpdatedAt,
  });
  const [recoveryConflict] = useState(
    recovered !== null &&
      (recovered.revision !== stored.revision ||
        (recovered.testUpdatedAt ?? null) !== (stored.testUpdatedAt ?? null)),
  );
  const [initialPending] = useState<PendingEdit | null>(() =>
    recovered
      ? {
          bundle: recovered.bundle,
          token: crypto.randomUUID(),
          createdAt: createdAt ?? Date.now(),
        }
      : null,
  );
  const pending = useRef(initialPending);
  const localQueue = useRef(Promise.resolve());
  const localTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const persist = useCallback(() => {
    const operation = localQueue.current.then(async () => {
      if (!scope) {
        setLocal("unavailable");
        return false;
      }
      const current = pending.current;
      try {
        if (current) {
          await scope.write(current.token, current.createdAt, {
            version: 1,
            ...base.current,
            bundle: current.bundle,
          } satisfies GroupRecovery);
          if (pending.current?.token === current.token) setLocal("stored");
        } else {
          await scope.remove();
          if (pending.current === null) setLocal("empty");
        }
        return true;
      } catch {
        setLocal("unavailable");
        return false;
      }
    });
    localQueue.current = operation.then(() => undefined);
    return operation;
  }, [scope]);

  const autosave = useAutosave<GroupBundle>({
    save: async (value) => {
      const issue = groupIssue(value);
      if (issue)
        throw new ApiError({ code: "UNKNOWN", status: 422, message: t(issue) });
      const result = await save(
        value,
        base.current.revision,
        base.current.testUpdatedAt,
      );
      base.current = { revision: result.revision, testUpdatedAt: result.testUpdatedAt };
      if (pending.current?.bundle === value) {
        pending.current = null;
        setDirty(false);
      }
      void persist();
    },
  });
  const { schedule, status, flush } = autosave;
  const copyRequired = recoveryConflict || status.kind === "stale";

  const change = useCallback(
    (value: GroupBundle) => {
      pending.current = {
        bundle: value,
        token: crypto.randomUUID(),
        createdAt: pending.current?.createdAt ?? Date.now(),
      };
      setBundle(value);
      setDirty(true);
      setLocal("pending");
      if (localTimer.current) clearTimeout(localTimer.current);
      localTimer.current = setTimeout(() => void persist(), 300);
      if (!copyRequired) schedule(value);
    },
    [copyRequired, persist, schedule],
  );

  const saveNow = useCallback(async () => {
    if (copyRequired) throw new Error("Conflict requires an independent copy");
    if (pending.current) schedule(pending.current.bundle);
    await flush();
  }, [copyRequired, schedule, flush]);

  const discardLocal = useCallback(async () => {
    pending.current = null;
    if (localTimer.current) clearTimeout(localTimer.current);
    return persist();
  }, [persist]);

  useEffect(
    () => () => {
      if (localTimer.current) clearTimeout(localTimer.current);
      void persist();
    },
    [persist],
  );

  return {
    bundle,
    change,
    dirty,
    local,
    persist,
    discardLocal,
    saveNow,
    copyRequired,
    status,
  };
}
