import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { Button } from "@/components/ui/button";
import { uploadMedia, type MediaAsset } from "@/features/media/api";
import {
  ACCEPT_ATTRIBUTE,
  MAX_DURATION_MS,
  type Rejection,
} from "@/features/media/limits";
import { precheck } from "@/features/media/probe";
import { ApiError } from "@/lib/api/errors";

interface UploadWidgetProps {
  onUploaded: (asset: MediaAsset) => void;
}

type State =
  | { status: "idle" }
  | { status: "checking"; name: string }
  | { status: "uploading"; name: string; fraction: number }
  | { status: "error"; message: string };

/**
 * Drag-and-drop plus a file picker for §11.1's audio uploads.
 *
 * The pre-check is advisory and the server re-validates everything; its job is
 * that a teacher is not made to wait for 10 MB to be told the file is too long.
 */
export function UploadWidget({ onUploaded }: UploadWidgetProps) {
  const { t, i18n } = useTranslation();
  const inputRef = useRef<HTMLInputElement>(null);
  const abortRef = useRef<AbortController | null>(null);
  const [state, setState] = useState<State>({ status: "idle" });
  const [dragging, setDragging] = useState(false);

  const busy = state.status === "checking" || state.status === "uploading";

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
      // The same file can be chosen again after a failure only if the input is
      // cleared: a repeat selection fires no change event otherwise.
      if (inputRef.current) inputRef.current.value = "";
    }
  }

  function onDrop(event: React.DragEvent) {
    event.preventDefault();
    setDragging(false);
    const file = event.dataTransfer.files[0];
    if (file && !busy) void accept(file);
  }

  return (
    <section className="rounded-lg border p-6" aria-labelledby="upload-heading">
      <h2 id="upload-heading" className="text-base font-semibold">
        {t("media.upload")}
      </h2>

      <button
        type="button"
        onClick={() => inputRef.current?.click()}
        onDragOver={(e) => {
          e.preventDefault();
          setDragging(true);
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={onDrop}
        disabled={busy}
        className={`mt-4 flex w-full flex-col items-center gap-2 rounded-md border border-dashed p-8 text-sm transition-colors ${
          dragging ? "border-primary bg-accent" : "border-input"
        } disabled:opacity-60`}
      >
        <span>{t("media.dropHere")}</span>
        <span className="text-muted-foreground text-xs">{t("media.dropHint")}</span>
      </button>

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
        <p
          className="text-muted-foreground mt-4 text-sm"
          role="status"
          aria-live="polite"
        >
          {state.name}
        </p>
      ) : null}

      {state.status === "uploading" ? (
        <div className="mt-4 flex items-center gap-3">
          <progress
            className="h-2 flex-1"
            value={state.fraction}
            max={1}
            aria-label={t("media.uploading")}
          />
          <span className="text-muted-foreground text-xs tabular-nums">
            {formatPercent(i18n.language, state.fraction)}
          </span>
          <Button variant="outline" size="sm" onClick={() => abortRef.current?.abort()}>
            {t("media.cancel")}
          </Button>
        </div>
      ) : null}

      {state.status === "error" ? (
        <p role="alert" className="text-destructive mt-4 text-sm">
          {state.message}
        </p>
      ) : null}
    </section>
  );
}

function rejectionMessage(t: TFunction, rejection: Rejection): string {
  switch (rejection.reason) {
    case "type":
      return t("media.rejectType");
    case "size":
      return t("media.rejectSize");
    case "duration":
      return t("media.rejectDuration", {
        minutes: Math.ceil((rejection.durationMs ?? MAX_DURATION_MS) / 60000),
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
