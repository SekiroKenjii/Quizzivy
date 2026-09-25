import { memo, useId, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { ChevronDown, ChevronUp } from "lucide-react";
import {
  GroupMaterials,
  type MaterialGap,
} from "@/components/shared/content/GroupMaterials";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { AudioPlayer } from "@/features/media/components/AudioPlayer";
import { useGroupPlaybackStore, groupPlayCount } from "../groupPlayback";
import { useTakeTestStore } from "../store";
import type { StudentGroup } from "../api";

/** GroupContext keeps shared listening controls mounted while navigating the group's questions. */
export const GroupContext = memo(function GroupContext({
  group,
  numbers,
  onGap,
  onRetryMedia,
  wide,
}: Readonly<{
  group: StudentGroup;
  numbers: ReadonlyMap<string, number>;
  onGap: (gap: MaterialGap) => void;
  onRetryMedia: () => void;
  wide: boolean;
}>) {
  const { t } = useTranslation();
  const contentId = useId();
  const [collapsed, setCollapsed] = useState(() => readCollapsed(group.id));
  const assets = useMemo(
    () => new Map(group.assets.map((asset) => [asset.id, asset])),
    [group.assets],
  );
  const show = wide || !collapsed;
  return (
    <Card
      className="min-w-0 gap-4 self-start lg:sticky lg:top-0 lg:max-h-[calc(100dvh-12rem)] lg:overflow-y-auto"
      aria-label={group.title}
    >
      <CardHeader className="gap-2">
        <div className="flex items-start justify-between gap-3">
          <CardTitle>
            <h2 className="text-base leading-relaxed">{group.title}</h2>
          </CardTitle>
          {!wide && (
            <Button
              variant="ghost"
              size="icon"
              className="size-11 shrink-0"
              aria-label={t(show ? "takeTest.hideMaterial" : "takeTest.showMaterial")}
              aria-expanded={show}
              aria-controls={contentId}
              onClick={() => {
                setCollapsed(!collapsed);
                writeCollapsed(group.id, !collapsed);
              }}
            >
              {show ? (
                <ChevronUp aria-hidden="true" />
              ) : (
                <ChevronDown aria-hidden="true" />
              )}
            </Button>
          )}
        </div>
        <CardDescription>
          {t("preview.sharedRange", {
            from: numbers.get(group.questionIds[0] ?? ""),
            to: numbers.get(group.questionIds.at(-1) ?? ""),
          })}
        </CardDescription>
      </CardHeader>
      <CardContent className="flex min-w-0 flex-col gap-4">
        {group.recordings.map((recording, index) => {
          const asset = assets.get(recording.assetId);
          return asset?.kind === "audio" && asset.url ? (
            <SharedAudio
              key={recording.id}
              recording={recording}
              label={t("takeTest.sharedAudioLabel", { n: index + 1 })}
              asset={asset}
              onRetry={onRetryMedia}
            />
          ) : (
            <p key={recording.id} role="status">
              {t("preview.materialUnavailable")}
            </p>
          );
        })}
        <div id={contentId} hidden={!show}>
          <GroupMaterials
            group={group}
            onRetryMedia={onRetryMedia}
            renderAudio={(node) => (
              <p className="text-muted-foreground text-sm">{node.label}</p>
            )}
            renderGap={(gap, label) => {
              const number = numbers.get(gap.questionId);
              return number == null ? (
                <span className="content-gap">{label}</span>
              ) : (
                <button
                  type="button"
                  className="content-gap"
                  aria-label={t("preview.goToQuestion", { n: number, label })}
                  onClick={() => onGap(gap)}
                >
                  {label}
                </button>
              );
            }}
          />
        </div>
      </CardContent>
    </Card>
  );
});

function SharedAudio({
  recording,
  label,
  asset,
  onRetry,
}: Readonly<{
  recording: StudentGroup["recordings"][number];
  label: string;
  asset: StudentGroup["assets"][number];
  onRetry: () => void;
}>) {
  const { t } = useTranslation();
  const played = useGroupPlaybackStore((state) => groupPlayCount(state, recording.id));
  const pending = useGroupPlaybackStore((state) =>
    state.pending.some((play) => play.recordingId === recording.id),
  );
  const notePlay = useGroupPlaybackStore((state) => state.notePlay);
  const flush = useGroupPlaybackStore((state) => state.flush);
  const locked = useTakeTestStore(
    (state) => state.lock !== null || state.submitState !== "idle",
  );
  const limit = recording.policy.maxPlays;
  const limitedHint =
    played >= (limit ?? 0)
      ? t("takeTest.playsUsed", { played, limit })
      : t("takeTest.playsLeft", { count: (limit ?? 0) - played });
  const hint = limit == null ? t("takeTest.sharedAudioUnlimited") : limitedHint;
  return (
    <div className="flex flex-col gap-2" role="group" aria-label={label}>
      <p className="text-sm font-medium">{label}</p>
      <AudioPlayer
        src={asset.url}
        label={label}
        durationMs={asset.durationMs}
        allowSeek={recording.policy.allowSeek}
        disabled={locked}
        preload="metadata"
        hint={hint}
        onPlay={() => notePlay(recording.id)}
        onRetry={onRetry}
      />
      <p className="text-muted-foreground text-xs leading-relaxed">
        {t("takeTest.sharedAudioScope")}
      </p>
      {limit != null && played >= limit && (
        <p role="status" className="text-muted-foreground text-xs leading-relaxed">
          {t("takeTest.extraPlaysRecorded")}
        </p>
      )}
      {pending && (
        <div className="text-muted-foreground flex flex-wrap items-center gap-2 text-xs">
          <span role="status">{t("takeTest.sharedAudioPending")}</span>
          <Button
            variant="ghost"
            size="sm"
            disabled={locked}
            onClick={() => void flush()}
          >
            {t("common.retry")}
          </Button>
        </div>
      )}
    </div>
  );
}

function readCollapsed(groupId: string): boolean {
  try {
    return sessionStorage.getItem(`quizzivy.material-collapsed.${groupId}`) === "true";
  } catch {
    return false;
  }
}

function writeCollapsed(groupId: string, value: boolean): void {
  try {
    sessionStorage.setItem(`quizzivy.material-collapsed.${groupId}`, String(value));
  } catch {
    return;
  }
}
