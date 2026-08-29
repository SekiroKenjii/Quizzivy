import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { formatDuration } from "@/features/media/format";
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
 *
 * Native controls, deliberately: §11.1's play limit and seek lock are rules for
 * a STUDENT sitting a test. The teacher who wrote the question is not the person
 * they constrain, and a policy-enforcing player here would be a worse tool.
 *
 * The URL was signed when the list loaded and lives ten minutes (§11.2), and
 * `preload="none"` means it is not fetched until play is pressed -- so reviewing
 * a bank for longer than that, which is the whole scenario above, reaches a
 * control that silently refuses. Failing loudly and offering a refetch is what
 * keeps that from looking like a broken player.
 */
export function AudioPreviewRow({ asset, onRetry, onClose }: AudioPreviewRowProps) {
  const { t } = useTranslation();
  const [failed, setFailed] = useState(false);

  return (
    <div className="bg-muted/30 flex items-center gap-3 px-3 py-3">
      {failed ? (
        <>
          <p role="alert" className="flex-1 text-xs">
            {t("bank.previewExpired")}
          </p>
          <Button variant="outline" size="xs" onClick={onRetry}>
            {t("common.retry")}
          </Button>
        </>
      ) : (
        <>
          {/* eslint-disable-next-line jsx-a11y/media-has-caption -- the
              transcript is a field on the question, shown in the editor. */}
          <audio
            className="h-9 flex-1"
            controls
            preload="none"
            src={asset.url}
            aria-label={asset.originalFilename}
            onError={() => setFailed(true)}
          />
          <p className="text-muted-foreground shrink-0 text-xs tabular-nums">
            {formatDuration(asset.durationMs)} · {asset.originalFilename}
          </p>
        </>
      )}
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
