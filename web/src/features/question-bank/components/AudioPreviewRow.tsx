import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { AudioPlayer } from "@/features/media/components/AudioPlayer";
import type { MediaAsset } from "@/features/media/api";

interface AudioPreviewRowProps {
  asset: MediaAsset;
  onRetry: () => void;
  onClose: () => void;
}

/**
 * A-06's inline preview. Opening a dialog to hear ninety seconds and closing it
 * again, forty times, is the difference between a usable bank and an abandoned
 * one -- so the audio plays in the row it belongs to.
 */
export function AudioPreviewRow({
  asset,
  onRetry,
  onClose,
}: Readonly<AudioPreviewRowProps>) {
  const { t } = useTranslation();

  return (
    <div className="bg-muted/30 flex items-center gap-3 px-3 py-3">
      <div className="min-w-0 flex-1">
        <AudioPlayer
          src={asset.url}
          label={asset.originalFilename}
          durationMs={asset.durationMs}
          hint={asset.originalFilename}
          onRetry={onRetry}
        />
      </div>
      <Button
        variant="ghost"
        size="xs"
        className="text-muted-foreground"
        onClick={onClose}
      >
        {t("common.close")}
      </Button>
    </div>
  );
}
