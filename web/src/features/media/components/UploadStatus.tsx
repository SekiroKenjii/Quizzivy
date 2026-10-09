import { useTranslation } from "react-i18next";
import { CircleAlert, FileAudio, FileImage, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { MediaKind } from "@/features/media/api";
import type { UploadState } from "@/features/media/useMediaUpload";

/**
 * UploadStatus draws useMediaUpload's state: the file being checked, the
 * upload's progress with a cancel, and a refusal or failure as an alert.
 * Idle draws nothing. `onRetry` adds "Choose another file" to the alert.
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

  if (state.status === "checking")
    return (
      <p className="text-muted-foreground text-sm" role="status" aria-live="polite">
        {state.name}
      </p>
    );

  if (state.status === "uploading")
    return (
      <div className="space-y-2 rounded-lg border p-3">
        <p className="flex items-center gap-2 text-sm font-medium">
          <FileIcon
            className="text-muted-foreground size-4 shrink-0"
            aria-hidden="true"
          />
          <span className="truncate">{state.name}</span>
        </p>
        <div className="flex items-center gap-3">
          <progress
            className="h-2 min-w-0 flex-1"
            value={state.fraction}
            max={1}
            aria-label={t("media.uploading")}
          />
          <span className="text-muted-foreground text-xs tabular-nums">
            {t("media.uploading")} · {formatPercent(i18n.language, state.fraction)}
          </span>
          <Button
            type="button"
            variant="ghost"
            size="icon-xs"
            aria-label={t("media.cancel")}
            onClick={onCancel}
          >
            <X aria-hidden="true" />
          </Button>
        </div>
      </div>
    );

  if (state.status === "error")
    return (
      <div
        role="alert"
        className="border-destructive/25 bg-destructive/5 flex items-start gap-3 rounded-lg border p-3.5"
      >
        <CircleAlert className="text-destructive size-5 shrink-0" aria-hidden="true" />
        <div className="min-w-0">
          <p className="text-sm font-medium">{t("media.rejectTitle")}</p>
          <p className="text-muted-foreground mt-1 text-xs leading-relaxed break-words">
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

  return null;
}

function formatPercent(locale: string, fraction: number): string {
  return new Intl.NumberFormat(locale, {
    style: "percent",
    maximumFractionDigits: 0,
  }).format(fraction);
}
