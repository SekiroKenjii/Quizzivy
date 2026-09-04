import { useImperativeHandle, useRef, useState, type Ref } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { CircleAlert } from "lucide-react";
import { Button } from "@/components/ui/button";
import { uploadMedia, type MediaAsset } from "@/features/media/api";
import {
  ACCEPT_ATTRIBUTE,
  MAX_DURATION_MS,
  type Rejection,
} from "@/features/media/limits";
import { formatBytes } from "@/features/media/format";
import { audioLength } from "@/lib/i18n/datetime";
import { precheck } from "@/features/media/probe";
import { ApiError } from "@/lib/api/errors";

/** Lets the page's header button and its drop target reach this panel. */
export interface UploadHandle {
  choose: () => void;
  accept: (file: File) => void;
  dropped: (files: File[]) => void;
}

interface UploadPanelProps {
  ref?: Ref<UploadHandle>;
  onUploaded: (asset: MediaAsset) => void;
}

type State =
  | { status: "idle" }
  | { status: "checking"; name: string }
  | { status: "uploading"; name: string; fraction: number }
  | { status: "error"; message: string };

/** The drop target for §11.1's audio uploads, and the progress it reports. */
export function UploadPanel({ ref, onUploaded }: UploadPanelProps) {
  const { t, i18n } = useTranslation();
  const inputRef = useRef<HTMLInputElement>(null);
  const abortRef = useRef<AbortController | null>(null);
  const [state, setState] = useState<State>({ status: "idle" });

  useImperativeHandle(ref, () => ({
    choose: () => inputRef.current?.click(),
    accept: (file: File) => void accept(file),
    dropped: (files: File[]) => {
      if (files.length === 0) {
        setState({ status: "error", message: t("media.rejectFolder") });
        return;
      }
      if (files.length > 1) {
        setState({ status: "error", message: t("media.rejectMany") });
        return;
      }
      void accept(files[0]!);
    },
  }));

  async function accept(file: File) {
    setState({ status: "checking", name: file.name });

    const rejection = await precheck(file);
    if (rejection) {
      setState({ status: "error", message: rejectionMessage(t, rejection) });
      return;
    }

    const controller = new AbortController();
    abortRef.current = controller;
    setState({ status: "uploading", name: file.name, fraction: 0 });

    try {
      const asset = await uploadMedia(file, {
        signal: controller.signal,
        onProgress: (fraction) =>
          setState({ status: "uploading", name: file.name, fraction }),
      });
      setState({ status: "idle" });
      onUploaded(asset);
    } catch (cause) {
      if (cause instanceof DOMException && cause.name === "AbortError") {
        setState({ status: "error", message: t("media.cancelled") });
        return;
      }
      setState({
        status: "error",
        message: cause instanceof ApiError ? cause.message : t("media.uploadFailed"),
      });
    } finally {
      abortRef.current = null;
      if (inputRef.current) inputRef.current.value = "";
    }
  }

  return (
    <>
      <input
        ref={inputRef}
        type="file"
        accept={ACCEPT_ATTRIBUTE}
        className="sr-only"
        aria-label={t("media.chooseFile")}
        onChange={(event) => {
          const file = event.target.files?.[0];
          if (file) void accept(file);
        }}
      />

      {state.status === "checking" ? (
        <p className="text-muted-foreground text-sm" role="status" aria-live="polite">
          {state.name}
        </p>
      ) : null}

      {state.status === "uploading" ? (
        <div className="space-y-2 rounded-lg border p-3">
          <p className="truncate text-sm font-medium">{state.name}</p>
          <div className="flex items-center gap-3">
            <progress
              className="h-2 flex-1"
              value={state.fraction}
              max={1}
              aria-label={t("media.uploading")}
            />
            <span className="text-muted-foreground text-xs tabular-nums">
              {formatPercent(i18n.language, state.fraction)}
            </span>
            <Button
              variant="outline"
              size="sm"
              onClick={() => abortRef.current?.abort()}
            >
              {t("media.cancel")}
            </Button>
          </div>
        </div>
      ) : null}

      {state.status === "error" ? (
        <div
          role="alert"
          className="border-destructive/25 bg-destructive/5 flex items-start gap-3 rounded-lg border p-3.5"
        >
          <CircleAlert
            className="text-destructive size-5 shrink-0"
            aria-hidden="true"
          />
          <div className="min-w-0">
            <p className="text-sm font-medium">{t("media.rejectTitle")}</p>
            <p className="text-muted-foreground mt-1 text-xs leading-relaxed">
              {state.message}
            </p>
            <Button
              variant="outline"
              size="sm"
              className="mt-2.5"
              onClick={() => inputRef.current?.click()}
            >
              {t("media.rejectRetry")}
            </Button>
          </div>
        </div>
      ) : null}
    </>
  );
}

// The deck's A-05: "recording.wav dài 6:12" -- a message that gives only the
// reason forces the teacher to guess which of the rules they broke, and which
// of the files they dropped broke it.
function rejectionMessage(t: TFunction, rejection: Rejection): string {
  switch (rejection.reason) {
    case "type":
      return t("media.rejectType", { name: rejection.name });
    case "size":
      return t("media.rejectSize", {
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

// Intl rather than a hardcoded "%": the symbol and its placement are
// locale-dependent, and the lint rule that forbids literals in JSX is what
// catches text that skipped i18n.
function formatPercent(locale: string, fraction: number): string {
  return new Intl.NumberFormat(locale, {
    style: "percent",
    maximumFractionDigits: 0,
  }).format(fraction);
}
