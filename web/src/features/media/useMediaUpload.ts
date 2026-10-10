import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import i18n, { type TFunction } from "i18next";
import type { UploadOptions } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import { audioLength } from "@/lib/i18n/datetime";
import { uploadMedia, type MediaAsset, type MediaKind } from "./api";
import { formatOverLimit } from "./format";
import { MAX_DURATION_MS, maxBytes, type Rejection } from "./limits";
import { precheck } from "./probe";

/**
 * UploadState is where one upload stands: checked, sent with its progress,
 * refused or failed (error), or stopped by the user (cancelled), which is not
 * a refusal.
 */
export type UploadState =
  | { status: "idle" }
  | { status: "checking"; name: string; bytes: number; kind: MediaKind }
  | { status: "uploading"; name: string; fraction: number }
  | { status: "error"; message: string }
  | { status: "cancelled" };

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
 * a refusal or failure as a message, and a cancellation. It draws nothing; its host
 * shows the state. `send` defaults to uploadMedia, which is the only sender
 * whose answer is a MediaAsset; a host that names another type passes its own.
 * A new start supersedes the one in flight, and unmounting aborts it.
 * `cancel` ends the check as well as the upload, without waiting for the
 * duration read to settle. With `onRejected`, a pre-check refusal goes to it,
 * with its worded message, and the state returns to idle, for a host that
 * reports refusals itself.
 */
export function useMediaUpload<T = MediaAsset>({
  onUploaded,
  onRejected,
  kind = "audio",
  send,
}: Readonly<{
  onUploaded: (result: T) => void;
  onRejected?: ((rejection: Rejection, message: string) => void) | undefined;
  kind?: MediaKind;
  send?: UploadSender<T>;
}>): MediaUpload<T> {
  const { t } = useTranslation();
  const [state, setState] = useState<UploadState>({ status: "idle" });
  const latest = useRef({ onUploaded, onRejected, kind, send });
  const run = useRef(0);
  const controller = useRef<AbortController | null>(null);

  useEffect(() => {
    latest.current = { onUploaded, onRejected, kind, send };
  }, [onUploaded, onRejected, kind, send]);

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
      setState({
        status: "checking",
        name: file.name,
        bytes: file.size,
        kind: checkedAs,
      });
      try {
        const refusal = await checkFile(file, checkedAs, abort.signal);
        if (!current()) return;
        if (refusal === "cancelled") {
          setState({ status: "cancelled" });
          return;
        }
        if (refusal !== null) {
          const message = rejectionMessage(t, refusal);
          const rejected = latest.current.onRejected;
          if (rejected === undefined) setState({ status: "error", message });
          else {
            setState({ status: "idle" });
            rejected(refusal, message);
          }
          return;
        }
        setState({ status: "uploading", name: file.name, fraction: 0 });
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
        if (current()) setState(uploadFailure(t, cause));
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

async function checkFile(
  file: File,
  kind: MediaKind,
  signal: AbortSignal,
): Promise<Rejection | "cancelled" | null> {
  const rejection = await Promise.race([precheck(file, kind), whenAborted(signal)]);
  if (signal.aborted) return "cancelled";
  return rejection;
}

function uploadFailure(t: TFunction, cause: unknown): UploadState {
  if (cause instanceof DOMException && cause.name === "AbortError")
    return { status: "cancelled" };
  return {
    status: "error",
    message: cause instanceof ApiError ? cause.message : t("media.uploadFailed"),
  };
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
        size: formatOverLimit(
          rejection.bytes,
          maxBytes(rejection.kind ?? "audio"),
          i18n.language,
        ),
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
