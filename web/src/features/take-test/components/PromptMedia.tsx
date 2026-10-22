import { useMemo, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { ContentImage } from "@/components/shared/content/ContentImage";
import { MediaAlt } from "@/components/shared/content/QuestionProse";
import {
  ProseMediaRendererContext,
  type ProseMediaRenderer,
} from "@/components/shared/content/proseMediaRenderer";
import { AudioPlayer } from "@/features/media/components/AudioPlayer";
import type { components } from "@/lib/api/schema";

type MediaAsset = components["schemas"]["MediaAsset"];

/**
 * PromptMedia draws the media of the question prose inside it (DG-109) from
 * the files its reader was signed: `StudentQuestion.promptAssets`,
 * `ResultQuestion.promptAssets` with `explanationAssets`, or a staff reader's
 * `contentAssets`. An image takes its own alt text, and an empty one keeps it
 * decorative. A prompt's audio marker draws `audio`, the question's recording
 * with its play limit, where the engine passes it, and nothing elsewhere; an
 * explanation's recording plays freely. A file the reader was not signed
 * shows the image's alt text.
 */
export function PromptMedia({
  assets,
  audio = null,
  onRetry,
  children,
}: Readonly<{
  assets: readonly MediaAsset[] | null | undefined;
  audio?: ReactNode;
  onRetry?: (() => void) | undefined;
  children: ReactNode;
}>) {
  const { t } = useTranslation();
  const byId = useMemo(
    () => new Map((assets ?? []).map((file) => [file.id.toLowerCase(), file])),
    [assets],
  );
  const render: ProseMediaRenderer = (node, field) => {
    if (node.type === "audio" && !("assetId" in node))
      return <>{field === "prompt" ? audio : null}</>;
    const file = "assetId" in node ? byId.get(node.assetId.toLowerCase()) : undefined;
    if (file?.kind !== node.type || !file.url) return <MediaAlt node={node} />;
    if (node.type === "image")
      return (
        <figure className="my-3">
          <ContentImage
            src={file.url}
            alt={node.alt}
            onRetry={onRetry}
            failure={t("takeTest.imageUnavailable")}
          />
        </figure>
      );
    return (
      <div className="my-3">
        <AudioPlayer
          src={file.url}
          label={t("result.explanationAudio")}
          durationMs={file.durationMs}
          allowSeek
          size="sm"
          preload="metadata"
          onRetry={onRetry}
        />
      </div>
    );
  };
  return (
    <ProseMediaRendererContext value={render}>{children}</ProseMediaRendererContext>
  );
}
