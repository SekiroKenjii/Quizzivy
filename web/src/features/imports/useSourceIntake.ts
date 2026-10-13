import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQueryClient, type QueryClient } from "@tanstack/react-query";
import type { TFunction } from "i18next";
import { ApiError, failureMessage } from "@/lib/api/errors";
import { AI_STATUS_KEY, AI_USAGE_KEY } from "@/features/ai/api";
import { formatBytes } from "@/features/media/format";
import type { components } from "@/lib/api/schema";
import {
  createWordImport,
  getWordImport,
  pasteImportSource,
  processWordImport,
  uploadImportSource,
  type ImportLimits,
  type ImportSource,
  type ImportSourceRole,
  type WordImport,
} from "./api";
import { refreshAvailability } from "./availability";
import { listFormats } from "./formats";
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

type Schemas = components["schemas"];

interface Pinned {
  uploadId: string | null;
  expectedRevision: number | null;
}

const UNPINNED: Pinned = { uploadId: null, expectedRevision: null };

interface PinnedText {
  text: string;
  uploadId: string;
  expectedRevision: number;
}

type Send = (into: WordImport, signal: AbortSignal) => Promise<WordImport | null>;

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
      formats: listFormats(limits.formats, "extension", "or", t),
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

function refusesReadWithAi(cause: unknown): boolean {
  return (
    cause instanceof ApiError &&
    cause.status === 422 &&
    cause.details !== undefined &&
    "readWithAi" in cause.details
  );
}

function refreshAi(client: QueryClient) {
  void client.invalidateQueries({ queryKey: AI_STATUS_KEY });
  void client.invalidateQueries({ queryKey: AI_USAGE_KEY });
}

const REFUSED: ReadonlySet<number> = new Set([400, 409, 413, 415]);
const FILE_FAULTS: ReadonlySet<number> = new Set([400, 413, 415]);

/**
 * useSourceIntake creates the import when there is none, uploads the exam and
 * then the key against the revision the exam produced, or with `startText`
 * sends pasted text as the exam, and queues processing. Each step keeps its
 * request identity across retries until its input changes, and a conflict
 * re-reads the import first. `start` asks for the files to be read with AI
 * when its `readWithAi` is true, which only a new import takes; a refusal of
 * it reads the assistant's status and usage again. The returned `readWithAi`
 * is the import's own choice once it exists, and null before. Every import
 * an upload or a re-read returns is reported through `onChanged`. With
 * `replacing`, starting requires a newly chosen file. Unmounting aborts an
 * intake in progress without reporting it.
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
  const [readWithAi, setReadWithAi] = useState<boolean | null>(
    existing === null ? null : existing.readWithAi === true,
  );
  const pinned = useRef<Record<ImportSourceRole, Pinned>>({
    exam: UNPINNED,
    answer_key: UNPINNED,
  });
  const created = useRef<Schemas["CreateWordImport"] | null>(null);
  const processed = useRef<{ requestId: string; revision: number } | null>(null);
  const files = useRef<Record<ImportSourceRole, File | null>>({
    exam: null,
    answer_key: null,
  });
  const running = useRef<AbortController | null>(null);
  const pinnedText = useRef<PinnedText | null>(null);
  const sentText = useRef<string | null>(null);

  useLayoutEffect(() => () => running.current?.abort(), []);

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

  async function ensureImport(
    title: string,
    readWithAi: boolean,
    signal: AbortSignal,
  ): Promise<WordImport> {
    const known = latest.current;
    if (
      current.current !== null &&
      known !== null &&
      known.revision > current.current.revision
    )
      current.current = known;
    if (current.current !== null) return current.current;
    if (
      created.current === null ||
      created.current.title !== title ||
      (created.current.readWithAi === true) !== readWithAi
    )
      created.current = {
        requestId: crypto.randomUUID(),
        title,
        ...(readWithAi ? { readWithAi: true } : {}),
      };
    const made = await createWordImport(created.current, signal);
    current.current = made;
    setImportId(made.id);
    setReadWithAi(made.readWithAi === true);
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

  async function sendFiles(
    into: WordImport,
    signal: AbortSignal,
  ): Promise<WordImport | null> {
    let target = into;
    for (const role of SOURCE_ROLES) {
      const next = await upload(role, target, signal);
      if (next === null || signal.aborted) return null;
      target = next;
    }
    return target;
  }

  async function sendText(
    text: string,
    into: WordImport,
    signal: AbortSignal,
  ): Promise<WordImport | null> {
    if (sentText.current === text && into.sources.some((item) => item.role === "exam"))
      return into;
    if (pinnedText.current?.text !== text)
      pinnedText.current = {
        text,
        uploadId: crypto.randomUUID(),
        expectedRevision: into.revision,
      };
    const { uploadId, expectedRevision } = pinnedText.current;
    try {
      const receipt = await pasteImportSource(
        into.id,
        { uploadId, expectedRevision, text },
        signal,
      );
      sentText.current = text;
      current.current = receipt.import;
      onChanged?.(receipt.import);
      return receipt.import;
    } catch (cause) {
      if (signal.aborted) return null;
      const status = cause instanceof ApiError ? cause.status : 0;
      if (REFUSED.has(status)) pinnedText.current = null;
      if (status === 409) await refresh(into);
      setError(failureMessage(cause, t("imports.paste.failed")));
      refreshAvailability(client, cause);
      return null;
    }
  }

  async function run(title: string, readWithAi: boolean, send: Send) {
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
      const target = await send(await ensureImport(title, readWithAi, signal), signal);
      if (target === null || signal.aborted) return;
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
      if (refusesReadWithAi(cause)) refreshAi(client);
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
    readWithAi,
    busy,
    error,
    choose,
    remove,
    start: (title: string, readWithAi = false) => run(title, readWithAi, sendFiles),
    startText: (title: string, text: string) =>
      run(title, false, (into, signal) => sendText(text, into, signal)),
    ready: examReady && keyReady && !busy && (!replacing || changed),
  };
}
