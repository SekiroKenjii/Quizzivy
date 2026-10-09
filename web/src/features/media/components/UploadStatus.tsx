import { useTranslation } from "react-i18next";
import { CircleAlert, FileAudio, FileImage, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { MediaKind } from "@/features/media/api";
import type { UploadState } from "@/features/media/useMediaUpload";

/**
 * UploadStatus draws useMediaUpload's state: the file being checked and the
 * upload's progress, each with a cancel; a refusal or failure as an alert;
 * and a cancellation as a plain status line, which is not a refusal. Idle
 * draws nothing. `onRetry` adds "Choose another file" to the alert and to the
 * cancellation.
 */
export function UploadStatus({
  state,
  kind = "audio",
  onCancel,
  onRetry,
}: Readonly<{
  state: UploadState;
  kind?: MediaKind;
  onCancel: () => void;
  onRetry?: (() => void) | undefined;
}>) {
  const { t, i18n } = useTranslation();
  const FileIcon = kind === "image" ? FileImage : FileAudio;
  const cancel = (
    <Button
      type="button"
      variant="ghost"
      size="icon-xs"
      aria-label={t("media.cancel")}
      onClick={onCancel}
    >
      <X aria-hidden="true" />
    </Button>
  );

  if (state.status === "checking")
    return (
      <div className="flex items-center gap-3">
        <p
          className="text-muted-fg min-w-0 flex-1 truncate text-sm"
          role="status"
          aria-live="polite"
        >
          {state.name}
        </p>
        {cancel}
      </div>
    );

  if (state.status === "uploading")
    return (
      <div className="space-y-2 rounded-lg border p-3">
        <p className="flex items-center gap-2 text-sm font-medium">
          <FileIcon className="text-muted-fg size-4 shrink-0" aria-hidden="true" />
          <span className="truncate">{state.name}</span>
        </p>
        <div className="flex items-center gap-3">
          <progress
            className="h-2 min-w-0 flex-1"
            value={state.fraction}
            max={1}
            aria-label={t("media.uploading")}
          />
          <span className="text-muted-fg text-xs tabular-nums">
            {t("media.uploading")} · {formatPercent(i18n.language, state.fraction)}
          </span>
          {cancel}
        </div>
      </div>
    );

  if (state.status === "error")
    return (
      <div
        role="alert"
        className="border-danger/25 bg-danger-soft flex items-start gap-3 rounded-lg border p-3.5"
      >
        <CircleAlert className="text-danger-ink size-5 shrink-0" aria-hidden="true" />
        <div className="min-w-0">
          <p className="text-sm font-medium">{t("media.rejectTitle")}</p>
          <p className="text-muted-fg mt-1 text-xs leading-relaxed break-words">
            {state.message}
          </p>
          {onRetry === undefined ? null : (
            <Button
              type="button"
              variant="outline"
              size="xs"
              className="mt-2.5"
              onClick={onRetry}
            >
              {t("media.rejectRetry")}
            </Button>
          )}
        </div>
      </div>
    );

  if (state.status === "cancelled")
    return (
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <p className="text-muted-fg text-sm" role="status" aria-live="polite">
          {t("media.cancelled")}
        </p>
        {onRetry === undefined ? null : (
          <Button type="button" variant="outline" size="xs" onClick={onRetry}>
            {t("media.rejectRetry")}
          </Button>
        )}
      </div>
    );

  return null;
}

function formatPercent(locale: string, fraction: number): string {
  return new Intl.NumberFormat(locale, {
    style: "percent",
    maximumFractionDigits: 0,
  }).format(fraction);
}
