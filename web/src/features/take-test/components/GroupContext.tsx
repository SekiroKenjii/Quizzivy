import { memo, useMemo, useRef, type Ref } from "react";
import { useTranslation } from "react-i18next";
import { ContentView } from "@/components/shared/content/ContentView";
import {
  GroupMaterials,
  type MaterialGap,
} from "@/components/shared/content/GroupMaterials";
import { materialsHead } from "@/components/shared/content/materials";
import { Button } from "@/components/ui/button";
import { AudioPlayer } from "@/features/media/components/AudioPlayer";
import { cn } from "@/lib/utils";
import { PassageBody } from "./PassageBody";
import { DrawerBar } from "./StimulusDrawer";
import { useKeptScroll } from "../panes";
import { usePaper } from "../paper";
import type { StudentGroup } from "../api";

/**
 * DrawerPane is how GroupContext lies over the question as the phone's
 * drawer: whether it is open, where a swipe has it (`offset`, in pixels from
 * open, or null), and the close button's ref and action.
 */
export interface DrawerPane {
  open: boolean;
  offset: number | null;
  closeRef: Ref<HTMLButtonElement>;
  onClose: () => void;
}

const DRAWER =
  "bg-bg absolute inset-y-0 left-0 z-60 w-[min(88%,420px)] touch-pan-y px-4 py-4.5 will-change-transform";
const SLIDE =
  "transition-transform duration-[320ms] ease-[cubic-bezier(.22,.8,.24,1)] motion-reduce:transition-none";

function paneClasses(wide: boolean, drawer: DrawerPane | undefined): string {
  if (drawer === undefined) {
    return cn("flex-[1_1_0]", wide ? "border-r px-8 py-7" : "px-4 py-4.5");
  }
  if (drawer.offset !== null) return cn(DRAWER, "shadow-float");
  return cn(
    DRAWER,
    SLIDE,
    drawer.open ? "translate-x-0 shadow-float" : "-translate-x-full",
  );
}

/**
 * GroupContext is the stimulus pane: the material a group of questions
 * shares, scrolling by itself beside the question, headed as materialsHead
 * says, with the section's title as the kicker when the heading already
 * says the group's. Its text cannot be selected. A gap in the material is a
 * button that goes to the question or blank it stands for. Given `drawer`,
 * below 768px, it is a drawer from the left over the question, with its bar
 * at the top; closed, it is out of sight and inert, and stays mounted, so it
 * keeps its place in the text and a recording keeps playing.
 */
export const GroupContext = memo(function GroupContext({
  group,
  sectionTitle,
  numbers,
  onGap,
  onRetryMedia,
  wide,
  drawer,
}: Readonly<{
  group: StudentGroup;
  sectionTitle?: string | undefined;
  numbers: ReadonlyMap<string, number>;
  onGap: (gap: MaterialGap) => void;
  onRetryMedia: () => void;
  wide: boolean;
  drawer?: DrawerPane | undefined;
}>) {
  const { t } = useTranslation();
  const pane = useRef<HTMLElement>(null);
  const onScroll = useKeptScroll(pane, false);
  const head = materialsHead(group, sectionTitle);
  const closed = drawer !== undefined && !drawer.open && drawer.offset === null;
  return (
    <article
      ref={pane}
      onScroll={onScroll}
      inert={closed}
      data-drawer-open={drawer?.open === true ? "" : undefined}
      aria-label={group.title}
      // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex -- Scroll regions need keyboard access independently of their content.
      tabIndex={0}
      className={cn(
        "min-w-0 overflow-y-auto -outline-offset-2! select-none",
        paneClasses(wide, drawer),
      )}
      style={
        drawer?.offset == null
          ? undefined
          : { transform: `translateX(${drawer.offset}px)` }
      }
    >
      {drawer !== undefined && (
        <DrawerBar closeRef={drawer.closeRef} onClose={drawer.onClose} />
      )}
      <PassageBody kicker={head.kicker} title={head.heading}>
        <GroupMaterials
          group={group}
          omitTitle={head.heading}
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
    <div data-shared-audio="" className="flex min-w-0 flex-col gap-3">
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
  const paper = usePaper();
  const startedAt = useRef<number | null>(null);
  const played = paper.recordingPlays[recording.id] ?? 0;
  const pending = paper.pendingRecordings.has(recording.id);
  const locked = paper.lock !== null || paper.submitState !== "idle";
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
          paper.noteRecordingPlay(recording.id);
        }}
        onEnded={() => {
          const durationMs =
            startedAt.current === null
              ? undefined
              : Math.max(0, Date.now() - startedAt.current);
          startedAt.current = null;
          paper.onRecordingEnded(recording.id, durationMs);
        }}
        onBlocked={() => {
          startedAt.current = null;
          paper.onRecordingBlocked(recording.id);
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
            onClick={paper.flushRecordingPlays}
          >
            {t("common.retry")}
          </Button>
        </div>
      )}
    </div>
  );
}
