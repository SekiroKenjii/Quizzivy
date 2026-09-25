import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQueryClient } from "@tanstack/react-query";
import type { TFunction } from "i18next";
import { ApiError, failureMessage } from "@/lib/api/errors";
import { formatBytes } from "@/features/media/format";
import {
  createWordImport,
  getWordImport,
  processWordImport,
  uploadImportSource,
  type ImportLimits,
  type ImportSource,
  type ImportSourceRole,
  type WordImport,
} from "./api";
import { refreshAvailability } from "./availability";
import { importCapabilitiesQuery } from "./queries";

/** SOURCE_ROLES orders the upload: the exam first, then its optional key. */
export const SOURCE_ROLES: readonly ImportSourceRole[] = ["exam", "answer_key"];

/** SourceSlot is one role's file as the teacher sees it: chosen, uploading, received, or refused. */
export interface SourceSlot {
  file: File | null;
  phase: "empty" | "chosen" | "uploading" | "uploaded" | "rejected";
  fraction: number;
  error: string | null;
  received: ImportSource | null;
}

type Slots = Record<ImportSourceRole, SourceSlot>;

interface Pinned {
  uploadId: string | null;
  expectedRevision: number | null;
}

const UNPINNED: Pinned = { uploadId: null, expectedRevision: null };

const EMPTY: SourceSlot = {
  file: null,
  phase: "empty",
  fraction: 0,
  error: null,
  received: null,
};

function initialSlots(existing: WordImport | null): Slots {
  const received = (role: ImportSourceRole): SourceSlot => {
    const source = existing?.sources.find((item) => item.role === role) ?? null;
    return source === null ? EMPTY : { ...EMPTY, phase: "uploaded", received: source };
  };
  return { exam: received("exam"), answer_key: received("answer_key") };
}

/** precheckFile applies the server's published limits before a byte is sent, naming the file in the refusal. */
export function precheckFile(
  file: File,
  limits: ImportLimits | undefined,
  t: TFunction,
): string | null {
  if (limits === undefined) return null;
  const name = file.name.toLowerCase();
  if (!limits.formats.some((format) => name.endsWith(`.${format}`)))
    return t("imports.upload.rejectType", {
      name: file.name,
      formats: limits.formats.map((format) => `.${format}`).join(", "),
    });
  if (file.size > limits.maxBytes)
    return t("imports.upload.rejectSize", {
      name: file.name,
      size: formatBytes(file.size),
      max: formatBytes(limits.maxBytes),
    });
  if (file.size === 0) return t("imports.upload.rejectEmpty", { name: file.name });
  return null;
}

const REFUSED: ReadonlySet<number> = new Set([400, 409, 413, 415]);
const FILE_FAULTS: ReadonlySet<number> = new Set([400, 413, 415]);

/**
 * useSourceIntake creates the import when there is none, uploads the exam and
 * then the key against the revision the exam produced, and queues processing.
 * Each step keeps its request identity across retries until its input
 * changes, and a conflict re-reads the import first. Every import an upload
 * or a re-read returns is reported through `onChanged`. With `replacing`,
 * starting requires a newly chosen file. Unmounting aborts an intake in
 * progress without reporting it.
 */
export function useSourceIntake({
  existing,
  limits,
  replacing = false,
  onChanged,
  onStarted,
}: Readonly<{
  existing: WordImport | null;
  limits: ImportLimits | undefined;
  replacing?: boolean;
  onChanged?: ((next: WordImport) => void) | undefined;
  onStarted: (started: WordImport) => void;
}>) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const [slots, setSlots] = useState<Slots>(() => initialSlots(existing));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const current = useRef<WordImport | null>(existing);
  const latest = useRef<WordImport | null>(existing);
  const [importId, setImportId] = useState<string | null>(existing?.id ?? null);
  const pinned = useRef<Record<ImportSourceRole, Pinned>>({
    exam: UNPINNED,
    answer_key: UNPINNED,
  });
  const created = useRef<{ requestId: string; title: string } | null>(null);
  const processed = useRef<{ requestId: string; revision: number } | null>(null);
  const files = useRef<Record<ImportSourceRole, File | null>>({
    exam: null,
    answer_key: null,
  });
  const running = useRef<AbortController | null>(null);

  useEffect(() => () => running.current?.abort(), []);

  useEffect(() => {
    latest.current = existing;
  }, [existing]);

  const patch = useCallback((role: ImportSourceRole, next: Partial<SourceSlot>) => {
    setSlots((previous) => ({ ...previous, [role]: { ...previous[role], ...next } }));
  }, []);

  const choose = useCallback(
    (role: ImportSourceRole, file: File) => {
      files.current[role] = file;
      pinned.current[role] = UNPINNED;
      const refusal = precheckFile(file, limits, t);
      patch(role, {
        file,
        phase: refusal === null ? "chosen" : "rejected",
        fraction: 0,
        error: refusal,
      });
    },
    [limits, patch, t],
  );

  const remove = useCallback((role: ImportSourceRole) => {
    files.current[role] = null;
    pinned.current[role] = UNPINNED;
    setSlots((previous) => ({
      ...previous,
      [role]:
        previous[role].received === null
          ? EMPTY
          : { ...EMPTY, phase: "uploaded", received: previous[role].received },
    }));
  }, []);

  async function refresh(from: WordImport) {
    const fresh = await getWordImport(from.id).catch(() => null);
    if (fresh === null) return;
    current.current = fresh;
    onChanged?.(fresh);
  }

  async function ensureImport(title: string, signal: AbortSignal): Promise<WordImport> {
    const known = latest.current;
    if (
      current.current !== null &&
      known !== null &&
      known.revision > current.current.revision
    )
      current.current = known;
    if (current.current !== null) return current.current;
    if (created.current === null || created.current.title !== title)
      created.current = { requestId: crypto.randomUUID(), title };
    const made = await createWordImport(created.current, signal);
    current.current = made;
    setImportId(made.id);
    return made;
  }

  async function upload(
    role: ImportSourceRole,
    into: WordImport,
    signal: AbortSignal,
  ): Promise<WordImport | null> {
    const file = files.current[role];
    if (file === null) return into;
    const pin = {
      uploadId: pinned.current[role].uploadId ?? crypto.randomUUID(),
      expectedRevision: pinned.current[role].expectedRevision ?? into.revision,
    };
    pinned.current[role] = pin;
    patch(role, { phase: "uploading", fraction: 0, error: null });
    try {
      const receipt = await uploadImportSource(
        into.id,
        file,
        { role, uploadId: pin.uploadId, expectedRevision: pin.expectedRevision },
        { onProgress: (fraction) => patch(role, { fraction }), signal },
      );
      files.current[role] = null;
      current.current = receipt.import;
      onChanged?.(receipt.import);
      patch(role, {
        file: null,
        phase: "uploaded",
        fraction: 1,
        error: null,
        received: receipt.source,
      });
      return receipt.import;
    } catch (cause) {
      if (signal.aborted) return null;
      const status = cause instanceof ApiError ? cause.status : 0;
      if (REFUSED.has(status)) pinned.current[role] = UNPINNED;
      if (status === 409) await refresh(into);
      patch(role, {
        phase: FILE_FAULTS.has(status) ? "rejected" : "chosen",
        error: failureMessage(cause, t("imports.upload.failed")),
      });
      return null;
    }
  }

  async function start(title: string) {
    const controller = new AbortController();
    running.current = controller;
    const { signal } = controller;
    setBusy(true);
    setError(null);
    try {
      const capabilities = await client.query({
        ...importCapabilitiesQuery(),
        staleTime: 0,
      });
      if (!capabilities.processingEnabled || signal.aborted) return;
      let target = await ensureImport(title, signal);
      for (const role of SOURCE_ROLES) {
        const next = await upload(role, target, signal);
        if (next === null || signal.aborted) return;
        target = next;
      }
      if (!target.sources.some((source) => source.role === "exam")) {
        setError(t("imports.upload.examRequired"));
        return;
      }
      if (processed.current?.revision !== target.revision)
        processed.current = {
          requestId: crypto.randomUUID(),
          revision: target.revision,
        };
      const started = await processWordImport(
        target.id,
        { requestId: processed.current.requestId, expectedRevision: target.revision },
        signal,
      );
      current.current = started;
      if (!signal.aborted) onStarted(started);
    } catch (cause) {
      if (signal.aborted) return;
      setError(failureMessage(cause, t("imports.upload.startFailed")));
      refreshAvailability(client, cause);
      if (
        cause instanceof ApiError &&
        cause.status === 409 &&
        current.current !== null
      ) {
        processed.current = null;
        await refresh(current.current);
      }
    } finally {
      if (!signal.aborted) setBusy(false);
    }
  }

  const examReady = slots.exam.phase === "chosen" || slots.exam.phase === "uploaded";
  const keyReady = slots.answer_key.phase !== "rejected";
  const changed = slots.exam.phase === "chosen" || slots.answer_key.phase === "chosen";
  return {
    slots,
    importId,
    busy,
    error,
    choose,
    remove,
    start,
    ready: examReady && keyReady && !busy && (!replacing || changed),
  };
}
