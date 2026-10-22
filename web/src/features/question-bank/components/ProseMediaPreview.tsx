import { useTranslation } from "react-i18next";
import { Image as ImageIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { QuestionMediaBlock } from "@/components/shared/content/questionContent";
import type { MediaAsset } from "@/features/media/api";
import { AudioPlayer } from "@/features/media/components/AudioPlayer";

/**
 * ProseMediaPreview is a prompt's or an explanation's media as the editor
 * shows it: the picture, the recording's player, or a tile that says the file
 * could not be loaded, with a retry when the page can fetch fresh links.
 */
export function ProseMediaPreview({
  node,
  file,
  onRefresh,
}: Readonly<{
  node: QuestionMediaBlock;
  file: MediaAsset | undefined;
  onRefresh: (() => void) | undefined;
}>) {
  const { t } = useTranslation();
  if (file === undefined)
    return (
      <div className="text-muted-fg flex h-42.5 flex-col items-center justify-center gap-1.5 px-3 text-center">
        <ImageIcon aria-hidden="true" className="size-6.5" />
        <span className="text-xs">{t("contentEditor.media.unavailable")}</span>
        {onRefresh ? (
          <Button type="button" variant="outline" size="sm" onClick={onRefresh}>
            {t("common.retry")}
          </Button>
        ) : null}
      </div>
    );
  if (node.type === "image")
    return (
      <img
        src={file.url}
        alt=""
        className="mx-auto block max-h-80 max-w-full"
        onError={onRefresh}
      />
    );
  return (
    <AudioPlayer
      src={file.url}
      label={file.originalFilename}
      durationMs={file.durationMs}
      size="row"
      {...(onRefresh ? { onRetry: onRefresh } : {})}
    />
  );
}
