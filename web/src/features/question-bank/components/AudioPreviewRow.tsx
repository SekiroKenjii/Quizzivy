import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { formatDuration } from "@/features/media/format";
import type { MediaAsset } from "@/features/media/api";

/**
 * A-06's inline preview. Opening a dialog to hear ninety seconds and closing it
 * again, forty times, is the difference between a usable bank and an abandoned
 * one -- so the audio plays in the row it belongs to.
 *
 * Native controls, deliberately: §11.1's play limit and seek lock are rules for
 * a STUDENT sitting a test. The teacher who wrote the question is not the person
 * they constrain, and a policy-enforcing player here would be a worse tool.
 */
export function AudioPreviewRow({
  asset,
  onClose,
}: {
  asset: MediaAsset;
  onClose: () => void;
}) {
  const { t } = useTranslation();

  return (
    <div className="bg-muted/30 flex items-center gap-3 px-3 py-3">
      {/* eslint-disable-next-line jsx-a11y/media-has-caption -- the transcript
          is a field on the question, shown in the editor. */}
      <audio
        className="h-9 flex-1"
        controls
        preload="none"
        src={asset.url}
        aria-label={asset.originalFilename}
      />
      <p className="text-muted-foreground shrink-0 text-xs tabular-nums">
        {formatDuration(asset.durationMs)} · {asset.originalFilename}
      </p>
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
