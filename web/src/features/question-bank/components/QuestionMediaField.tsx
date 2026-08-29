import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { AudioLines, FileAudio, Library, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AssetLibraryDialog } from "@/features/media/components/AssetLibraryDialog";
import {
  UploadPanel,
  type UploadHandle,
} from "@/features/media/components/UploadPanel";
import { useFileDrop } from "@/features/media/useFileDrop";
import type { MediaAsset } from "@/features/media/api";
import { formatBytes, formatDuration } from "@/features/media/format";

interface QuestionMediaFieldProps {
  value: MediaAsset | null;
  onChange: (asset: MediaAsset | null) => void;
}

/**
 * The deck's A-04 media block, in its two states: a dashed drop target that
 * states the format and the limits before a file is chosen (§11.1), and A-05's
 * attached-asset card once one is.
 */
export function QuestionMediaField({ value, onChange }: QuestionMediaFieldProps) {
  const { t } = useTranslation();
  const uploader = useRef<UploadHandle>(null);
  const [picking, setPicking] = useState(false);
  const dragging = useFileDrop((file) => uploader.current?.accept(file));

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
          <Button
            variant="outline"
            size="xs"
            className="mt-3"
            onClick={() => uploader.current?.choose()}
          >
            {t("questionEditor.mediaChoose")}
          </Button>
        </div>
      ) : (
        <div className="flex items-center gap-3 rounded-lg border p-3.5">
          <FileAudio className="text-muted-foreground size-5" aria-hidden="true" />
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-medium">{value.originalFilename}</p>
            <p className="text-muted-foreground text-xs tabular-nums">
              {formatDuration(value.durationMs)} · {formatBytes(value.bytes)}
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
      )}

      <UploadPanel ref={uploader} onUploaded={onChange} />

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
