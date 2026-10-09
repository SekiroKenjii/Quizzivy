import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { AudioLines, FileAudio, Library, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AssetLibraryDialog } from "@/features/media/components/AssetLibraryDialog";
import { UploadStatus } from "@/features/media/components/UploadStatus";
import { useMediaUpload } from "@/features/media/useMediaUpload";
import { ACCEPT_ATTRIBUTE } from "@/features/media/limits";
import { useFileDrop } from "@/hooks/useFileDrop";
import type { MediaAsset } from "@/features/media/api";
import { AudioPlayer } from "@/features/media/components/AudioPlayer";
import { formatBytes } from "@/features/media/format";
import { audioLength } from "@/lib/i18n/datetime";

interface QuestionMediaFieldProps {
  /** Refetches the question, minting a fresh signed URL when one expires. */
  onRefresh?: (() => void) | undefined;
  value: MediaAsset | null;
  onChange: (asset: MediaAsset | null) => void;
}

/**
 * The deck's A-04 media block, in its two states: a dashed drop target that
 * states the format and the limits before a file is chosen (§11.1), and A-05's
 * attached-asset card once one is.
 */
export function QuestionMediaField({
  value,
  onChange,
  onRefresh,
}: Readonly<QuestionMediaFieldProps>) {
  const { t } = useTranslation();
  const fileInput = useRef<HTMLInputElement>(null);
  const [picking, setPicking] = useState(false);
  const upload = useMediaUpload({ onUploaded: onChange });
  const dragging = useFileDrop(upload.dropped);
  const choose = () => fileInput.current?.click();

  const retryProps = onRefresh ? { onRetry: onRefresh } : {};
  return (
    <div className="space-y-2">
      {value === null ? (
        <div
          className={`rounded-lg border border-dashed p-5 text-center ${
            dragging ? "border-primary bg-accent" : ""
          }`}
        >
          <AudioLines
            className="text-muted-foreground mx-auto size-5"
            aria-hidden="true"
          />
          <p className="mt-2 text-xs">{t("questionEditor.mediaDrop")}</p>
          <p className="text-muted-foreground mt-1 text-xs">
            {t("questionEditor.mediaHint")}
          </p>
          <Button variant="outline" size="xs" className="mt-3" onClick={choose}>
            {t("questionEditor.mediaChoose")}
          </Button>
        </div>
      ) : (
        <div className="space-y-3 rounded-lg border p-3.5">
          <div className="flex items-center gap-3">
            <FileAudio className="text-muted-foreground size-5" aria-hidden="true" />
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium">{value.originalFilename}</p>
              <p className="text-muted-foreground text-xs tabular-nums">
                {audioLength(value.durationMs)} · {formatBytes(value.bytes)}
              </p>
            </div>
            <Button
              variant="ghost"
              size="icon-xs"
              aria-label={t("questionEditor.mediaRemove")}
              onClick={() => onChange(null)}
            >
              <Trash2 aria-hidden="true" />
            </Button>
          </div>

          {value.kind === "audio" ? (
            <AudioPlayer
              src={value.url}
              label={value.originalFilename}
              durationMs={value.durationMs}
              size="sm"
              {...retryProps}
            />
          ) : null}
        </div>
      )}

      <input
        ref={fileInput}
        type="file"
        accept={ACCEPT_ATTRIBUTE}
        className="sr-only"
        aria-label={t("media.chooseFile")}
        onChange={(event) => {
          const file = event.target.files?.[0];
          event.target.value = "";
          if (file) void upload.start(file);
        }}
      />
      <UploadStatus state={upload.state} onCancel={upload.cancel} onRetry={choose} />

      <Button
        variant="ghost"
        size="sm"
        className="text-muted-foreground w-full justify-start"
        onClick={() => setPicking(true)}
      >
        <Library aria-hidden="true" />
        {t("questionEditor.mediaFromLibrary")}
      </Button>

      <AssetLibraryDialog open={picking} onOpenChange={setPicking} onPick={onChange} />
    </div>
  );
}
