import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { GroupMaterials } from "@/components/shared/content/GroupMaterials";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import type { components } from "@/lib/api/schema";
import { AudioPlayer } from "./AudioPlayer";

type StudentGroup = components["schemas"]["StudentGroup"];

/** ReviewGroup shows frozen context and only server-released transcripts without recording new plays. */
export function ReviewGroup({
  group,
  transcripts,
  plays,
  numbers,
  onQuestion,
  onRetry,
}: Readonly<{
  group: StudentGroup;
  transcripts: Readonly<Record<string, string>>;
  plays?: Readonly<Record<string, number>> | undefined;
  numbers: ReadonlyMap<string, number>;
  onQuestion?: ((questionId: string) => void) | undefined;
  onRetry: () => void;
}>) {
  const { t } = useTranslation();
  const assets = useMemo(
    () => new Map(group.assets.map((asset) => [asset.id, asset])),
    [group.assets],
  );
  return (
    <Card className="min-w-0">
      <CardHeader>
        <CardTitle>
          <h3 className="leading-relaxed">{group.title}</h3>
        </CardTitle>
        <CardDescription>
          {t("preview.sharedRange", {
            from: numbers.get(group.questionIds[0] ?? ""),
            to: numbers.get(group.questionIds.at(-1) ?? ""),
          })}
        </CardDescription>
      </CardHeader>
      <CardContent className="flex min-w-0 flex-col gap-5">
        {group.recordings.map((recording, index) => {
          const asset = assets.get(recording.assetId);
          const transcript = transcripts[recording.id];
          return (
            <div
              key={recording.id}
              className="flex flex-col gap-2"
              role="group"
              aria-label={t("takeTest.sharedAudioLabel", { n: index + 1 })}
            >
              <h4 className="text-sm font-medium">
                {t("takeTest.sharedAudioLabel", { n: index + 1 })}
              </h4>
              {asset?.kind === "audio" && asset.url ? (
                <AudioPlayer
                  src={asset.url}
                  label={t("takeTest.sharedAudioLabel", { n: index + 1 })}
                  durationMs={asset.durationMs}
                  allowSeek
                  hint={t("sharedReview.playbackHint")}
                  onRetry={onRetry}
                />
              ) : (
                <p role="status">{t("preview.materialUnavailable")}</p>
              )}
              {plays !== undefined && (
                <p className="text-muted-foreground text-xs">
                  {t(
                    recording.policy.maxPlays == null
                      ? "sharedReview.unlimitedCount"
                      : "sharedReview.count",
                    {
                      used: plays[recording.id] ?? 0,
                      limit: recording.policy.maxPlays,
                    },
                  )}
                </p>
              )}
              {transcript !== undefined && (
                <details className="bg-muted/30 rounded-md border p-3">
                  <summary className="cursor-pointer text-sm font-medium">
                    {t("sharedReview.transcript")}
                  </summary>
                  <p className="mt-3 text-sm leading-relaxed whitespace-pre-wrap">
                    {transcript}
                  </p>
                </details>
              )}
            </div>
          );
        })}
        <GroupMaterials
          group={group}
          onRetryMedia={onRetry}
          renderAudio={(node) => (
            <p className="text-muted-foreground text-sm">{node.label}</p>
          )}
          renderGap={(gap, label) => {
            const number = numbers.get(gap.questionId);
            if (!onQuestion || number === undefined)
              return <span className="content-gap">{label}</span>;
            return (
              <button
                className="content-gap"
                type="button"
                onClick={() => onQuestion(gap.questionId)}
                aria-label={t("preview.goToQuestion", { n: number, label })}
              >
                {label}
              </button>
            );
          }}
        />
      </CardContent>
    </Card>
  );
}
