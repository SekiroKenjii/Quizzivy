import { memo, useMemo, useRef } from "react";
import { useTranslation } from "react-i18next";
import { ContentView } from "@/components/shared/content/ContentView";
import {
  GroupMaterials,
  type MaterialGap,
} from "@/components/shared/content/GroupMaterials";
import { Button } from "@/components/ui/button";
import { recordSharedAudioEvent } from "@/features/integrity/useIntegrityMonitor";
import { AudioPlayer } from "@/features/media/components/AudioPlayer";
import { cn } from "@/lib/utils";
import { PassageBody } from "./PassageBody";
import { useGroupPlaybackStore, groupPlayCount } from "../groupPlayback";
import { useTakeTestStore } from "../store";
import { useKeptScroll } from "../panes";
import type { StudentGroup } from "../api";

/**
 * GroupContext is the passage pane: the reading material a group of
 * questions shares, scrolling by itself beside the question. Its text cannot
 * be selected. A gap in the material is a button that goes to the question
 * or blank it stands for. While `hidden`, on a phone showing the question,
 * it stays mounted and keeps its place in the text.
 */
export const GroupContext = memo(function GroupContext({
  group,
  eyebrow,
  numbers,
  onGap,
  onRetryMedia,
  wide,
  hidden,
}: Readonly<{
  group: StudentGroup;
  eyebrow?: string | undefined;
  numbers: ReadonlyMap<string, number>;
  onGap: (gap: MaterialGap) => void;
  onRetryMedia: () => void;
  wide: boolean;
  hidden: boolean;
}>) {
  const { t } = useTranslation();
  const pane = useRef<HTMLElement>(null);
  const onScroll = useKeptScroll(pane, hidden);
  return (
    <article
      ref={pane}
      onScroll={onScroll}
      hidden={hidden}
      aria-label={group.title}
      // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex -- Scroll regions need keyboard access independently of their content.
      tabIndex={0}
      className={cn(
        "min-w-0 flex-[1_1_0] overflow-y-auto -outline-offset-2! select-none",
        wide ? "border-r px-8 py-7" : "px-4 py-4.5",
      )}
    >
      <PassageBody eyebrow={eyebrow} title={group.title}>
        <GroupMaterials
          group={group}
          omitTitle={group.title}
          onRetryMedia={onRetryMedia}
          renderAudio={(node) => <p className="text-muted-fg text-ui">{node.label}</p>}
          renderGap={(gap, label) => {
            const number = numbers.get(gap.questionId);
            return number == null ? (
              <span className="content-gap">{label}</span>
            ) : (
              <button
                type="button"
                className="content-gap items-end"
                aria-label={t("preview.goToQuestion", { n: number, label })}
                onClick={() => onGap(gap)}
              >
                {label}
              </button>
            );
          }}
        />
      </PassageBody>
    </article>
  );
});

/**
 * GroupListening is what a group adds to the question pane: its shared
 * recordings, which stay mounted, and so keep playing, while the student
 * moves between the group's questions. A group with nothing to read also
 * states its title and instructions here, since it has no passage pane.
 */
export const GroupListening = memo(function GroupListening({
  group,
  onRetryMedia,
}: Readonly<{
  group: StudentGroup;
  onRetryMedia: () => void;
}>) {
  const { t } = useTranslation();
  const assets = useMemo(
    () => new Map(group.assets.map((asset) => [asset.id, asset])),
    [group.assets],
  );
  const standalone = group.stimuli.length === 0;
  if (!standalone && group.recordings.length === 0) return null;
  return (
    <div className="flex min-w-0 flex-col gap-3">
      {standalone && (
        <div className="flex flex-col gap-1.5">
          <h2 className="text-title font-semibold">{group.title}</h2>
          {group.instructions ? (
            <ContentView
              document={group.instructions}
              className="text-muted-fg text-ui"
            />
          ) : null}
        </div>
      )}
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
          <p key={recording.id} role="status" className="text-muted-fg text-ui">
            {t("preview.materialUnavailable")}
          </p>
        );
      })}
    </div>
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
  const attemptId = useTakeTestStore((state) => state.attemptId);
  const startedAt = useRef<number | null>(null);
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
    <div className="flex flex-col gap-1.5" role="group" aria-label={label}>
      <p className="text-ui font-medium">{label}</p>
      <AudioPlayer
        src={asset.url}
        label={label}
        durationMs={asset.durationMs}
        allowSeek={recording.policy.allowSeek}
        disabled={locked}
        preload="metadata"
        hint={hint}
        onPlay={() => {
          startedAt.current = Date.now();
          notePlay(recording.id);
        }}
        onEnded={() => {
          const durationMs =
            startedAt.current === null
              ? undefined
              : Math.max(0, Date.now() - startedAt.current);
          startedAt.current = null;
          if (attemptId !== null)
            recordSharedAudioEvent(attemptId, "audio_ended", recording.id, durationMs);
        }}
        onBlocked={() => {
          startedAt.current = null;
          if (attemptId !== null)
            recordSharedAudioEvent(attemptId, "audio_blocked", recording.id);
        }}
        onRetry={onRetry}
      />
      <p className="text-muted-fg text-meta">{t("takeTest.sharedAudioScope")}</p>
      {limit != null && played >= limit && (
        <p role="status" className="text-muted-fg text-meta">
          {t("takeTest.extraPlaysRecorded")}
        </p>
      )}
      {pending && (
        <div className="text-muted-fg text-meta flex flex-wrap items-center gap-2">
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
