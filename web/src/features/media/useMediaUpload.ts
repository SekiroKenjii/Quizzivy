import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import type { UploadOptions } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import { audioLength } from "@/lib/i18n/datetime";
import { uploadMedia, type MediaAsset, type MediaKind } from "./api";
import { formatBytes } from "./format";
import { MAX_DURATION_MS, type Rejection } from "./limits";
import { precheck } from "./probe";

/** UploadState is where one upload stands: checked, sent with its progress, or refused. */
export type UploadState =
  | { status: "idle" }
  | { status: "checking"; name: string }
  | { status: "uploading"; name: string; fraction: number }
  | { status: "error"; message: string };

/** UploadSender sends one checked file and resolves with what the server answered. */
export type UploadSender<T> = (file: File, options: UploadOptions) => Promise<T>;

/** UploadRequest overrides, for one upload, the kind it is checked as and where it is sent. */
export interface UploadRequest<T> {
  kind?: MediaKind;
  send?: UploadSender<T>;
}

/** MediaUpload is what useMediaUpload hands its host: the state and the controls over it. */
export interface MediaUpload<T> {
  state: UploadState;
  busy: boolean;
  start: (file: File, request?: UploadRequest<T>) => Promise<void>;
  dropped: (files: readonly File[], request?: UploadRequest<T>) => void;
  cancel: () => void;
  reset: () => void;
}

/**
 * useMediaUpload runs §11.1's upload: the client pre-check from `limits.ts`
 * (type, size, then an audio file's duration), the upload with its progress,
 * and a failure or a cancellation as a message. It draws nothing; its host
 * shows the state. `send` defaults to uploadMedia, which is the only sender
 * whose answer is a MediaAsset; a host that names another type passes its own.
 * A new start supersedes the one in flight, and unmounting aborts it.
 * `cancel` ends the check as well as the upload, without waiting for the
 * duration read to settle.
 */
export function useMediaUpload<T = MediaAsset>({
  onUploaded,
  kind = "audio",
  send,
}: Readonly<{
  onUploaded: (result: T) => void;
  kind?: MediaKind;
  send?: UploadSender<T>;
}>): MediaUpload<T> {
  const { t } = useTranslation();
  const [state, setState] = useState<UploadState>({ status: "idle" });
  const latest = useRef({ onUploaded, kind, send });
  const run = useRef(0);
  const controller = useRef<AbortController | null>(null);

  useEffect(() => {
    latest.current = { onUploaded, kind, send };
  }, [onUploaded, kind, send]);

  useEffect(
    () => () => {
      run.current += 1;
      controller.current?.abort();
    },
    [],
  );

  const start = useCallback(
    async (file: File, request?: UploadRequest<T>) => {
      controller.current?.abort();
      run.current += 1;
      const id = run.current;
      const current = () => id === run.current;
      const checkedAs = request?.kind ?? latest.current.kind;
      const sender =
        request?.send ??
        latest.current.send ??
        (uploadMedia as unknown as UploadSender<T>);

      const abort = new AbortController();
      controller.current = abort;
      const cancelled = () =>
        setState({ status: "error", message: t("media.cancelled") });
      setState({ status: "checking", name: file.name });
      const rejection = await Promise.race([
        precheck(file, checkedAs),
        whenAborted(abort.signal),
      ]);
      if (!current()) return;
      if (abort.signal.aborted) {
        if (controller.current === abort) controller.current = null;
        cancelled();
        return;
      }
      if (rejection) {
        if (controller.current === abort) controller.current = null;
        setState({ status: "error", message: rejectionMessage(t, rejection) });
        return;
      }

      setState({ status: "uploading", name: file.name, fraction: 0 });
      try {
        const result = await sender(file, {
          signal: abort.signal,
          onProgress: (fraction) => {
            if (current()) setState({ status: "uploading", name: file.name, fraction });
          },
        });
        if (!current()) return;
        setState({ status: "idle" });
        latest.current.onUploaded(result);
      } catch (cause) {
        if (!current()) return;
        if (cause instanceof DOMException && cause.name === "AbortError") {
          cancelled();
          return;
        }
        setState({
          status: "error",
          message: cause instanceof ApiError ? cause.message : t("media.uploadFailed"),
        });
      } finally {
        if (controller.current === abort) controller.current = null;
      }
    },
    [t],
  );

  const dropped = useCallback(
    (files: readonly File[], request?: UploadRequest<T>) => {
      if (files.length === 0) {
        setState({ status: "error", message: t("media.rejectFolder") });
        return;
      }
      if (files.length > 1) {
        setState({ status: "error", message: t("media.rejectMany") });
        return;
      }
      void start(files[0]!, request);
    },
    [start, t],
  );

  const cancel = useCallback(() => controller.current?.abort(), []);

  const reset = useCallback(() => {
    run.current += 1;
    controller.current?.abort();
    setState({ status: "idle" });
  }, []);

  const busy = state.status === "checking" || state.status === "uploading";
  return { state, busy, start, dropped, cancel, reset };
}

function whenAborted(signal: AbortSignal): Promise<null> {
  return new Promise((resolve) => {
    if (signal.aborted) resolve(null);
    else signal.addEventListener("abort", () => resolve(null), { once: true });
  });
}

/** rejectionMessage words a pre-check refusal: the file's name and the rule of its kind it broke. */
export function rejectionMessage(t: TFunction, rejection: Rejection): string {
  const image = rejection.kind === "image";
  switch (rejection.reason) {
    case "type":
      return t(image ? "media.rejectImageType" : "media.rejectType", {
        name: rejection.name,
      });
    case "size":
      return t(image ? "media.rejectImageSize" : "media.rejectSize", {
        name: rejection.name,
        size: formatBytes(rejection.bytes),
      });
    case "duration":
      return t("media.rejectDuration", {
        name: rejection.name,
        duration: audioLength(rejection.durationMs ?? MAX_DURATION_MS),
      });
    default:
      return t("media.uploadFailed");
  }
}
