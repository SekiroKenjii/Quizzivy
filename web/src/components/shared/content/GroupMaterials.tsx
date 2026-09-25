import { useMemo, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import type { components } from "@/lib/api/schema";
import { ContentView, type AssetRenderer } from "./ContentView";
import { ContentImage } from "./ContentImage";

type StudentGroup = components["schemas"]["StudentGroup"];
export type MaterialGap = StudentGroup["stimuli"][number]["gaps"][number];
type AudioNode = Extract<Parameters<AssetRenderer>[0], { type: "audio" }>;
export type GroupAudioRenderer = (
  node: AudioNode,
  asset: StudentGroup["assets"][number],
  recording: StudentGroup["recordings"][number],
) => ReactNode;

/** GroupMaterials renders frozen, learner-safe content using only the group's authorized asset bindings. */
export function GroupMaterials({
  group,
  renderAudio,
  renderGap,
  onRetryMedia,
}: Readonly<{
  group: StudentGroup;
  renderAudio: GroupAudioRenderer;
  renderGap: (gap: MaterialGap, label: string) => ReactNode;
  onRetryMedia?: (() => void) | undefined;
}>) {
  const { t } = useTranslation();
  const assets = useMemo(
    () => new Map(group.assets.map((asset) => [asset.id, asset])),
    [group.assets],
  );
  const recordings = useMemo(
    () => new Map(group.recordings.map((recording) => [recording.assetId, recording])),
    [group.recordings],
  );
  const renderAsset: AssetRenderer = (node) => {
    const asset = assets.get(node.assetId);
    const recording = recordings.get(node.assetId);
    if (
      !asset?.url ||
      asset.kind !== node.type ||
      (node.type === "audio" && !recording)
    )
      return <p role="status">{t("preview.materialUnavailable")}</p>;
    if (node.type === "image")
      return <ContentImage src={asset.url} alt={node.alt} onRetry={onRetryMedia} />;
    return <>{recording ? renderAudio(node, asset, recording) : null}</>;
  };
  return (
    <div className="flex min-w-0 flex-col gap-5">
      {group.instructions ? <ContentView document={group.instructions} /> : null}
      {group.stimuli.map((material) => {
        const gaps = new Map(material.gaps.map((gap) => [gap.gapId, gap]));
        return (
          <section
            key={material.id}
            className="flex min-w-0 flex-col gap-2"
            aria-label={material.title}
          >
            <h4 className="text-sm font-medium">{material.title}</h4>
            <ContentView
              document={material.content}
              renderAsset={renderAsset}
              renderGap={(gap) => {
                const binding = gaps.get(gap.id);
                return binding ? (
                  renderGap(binding, gap.label)
                ) : (
                  <span className="content-gap">{gap.label}</span>
                );
              }}
            />
          </section>
        );
      })}
    </div>
  );
}
