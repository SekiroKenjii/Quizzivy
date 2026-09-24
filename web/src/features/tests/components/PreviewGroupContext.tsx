import { PreviewImage } from "./PreviewImage";
import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import {
  ContentView,
  type AssetRenderer,
} from "@/components/shared/content/ContentView";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { AudioPlayer } from "@/features/media/components/AudioPlayer";
import type { components } from "@/lib/api/schema";

type StudentGroup = components["schemas"]["StudentGroup"];

/** PreviewGroupContext shows one frozen context and links its gaps to the displayed questions. */
export function PreviewGroupContext({
  group,
  numbers,
  questionAnchor,
  onRetryMedia,
}: Readonly<{
  group: StudentGroup;
  numbers: ReadonlyMap<string, number>;
  questionAnchor: (id: string) => string;
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
    ) {
      return <p role="status">{t("preview.materialUnavailable")}</p>;
    }
    if (node.type === "image")
      return <PreviewImage src={asset.url} alt={node.alt} onRetry={onRetryMedia} />;
    return (
      <AudioPlayer
        key={recording?.id}
        src={asset.url}
        label={node.label}
        durationMs={asset.durationMs}
        allowSeek={recording?.policy.allowSeek ?? false}
        hint={t("preview.sharedAudioHint")}
        onRetry={onRetryMedia}
      />
    );
  };
  return (
    <Card className="min-w-0">
      <CardHeader>
        <CardTitle>
          <h3>{group.title}</h3>
        </CardTitle>
        <CardDescription>
          {t("preview.sharedRange", {
            from: numbers.get(group.questionIds[0] ?? ""),
            to: numbers.get(group.questionIds.at(-1) ?? ""),
          })}
        </CardDescription>
      </CardHeader>
      <CardContent className="flex min-w-0 flex-col gap-5">
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
                  const number = binding ? numbers.get(binding.questionId) : undefined;
                  if (!binding || number == null)
                    return <span className="content-gap">{gap.label}</span>;
                  const anchor = questionAnchor(binding.questionId);
                  return (
                    <a
                      className="content-gap"
                      href={`#${anchor}`}
                      aria-label={t("preview.goToQuestion", {
                        n: number,
                        label: gap.label,
                      })}
                      onClick={() => document.getElementById(anchor)?.focus()}
                    >
                      {gap.label}
                    </a>
                  );
                }}
              />
            </section>
          );
        })}
      </CardContent>
    </Card>
  );
}
