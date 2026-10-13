import { memo, useMemo, useRef, type Ref } from "react";
import { useTranslation } from "react-i18next";
import { GapChip } from "@/components/shared/content/GapChip";
import {
  GroupMaterials,
  type MaterialGap,
} from "@/components/shared/content/GroupMaterials";
import { materialsHead } from "@/components/shared/content/materials";
import { Button } from "@/components/ui/button";
import { AudioPlayer } from "@/features/media/components/AudioPlayer";
import { nfc } from "@/lib/nfc";
import { cn } from "@/lib/utils";
import { PassageBody } from "./PassageBody";
import { DrawerBar } from "./StimulusDrawer";
import { useKeptScroll } from "../panes";
import { usePaper } from "../paper";
import { questionKind } from "../questionType";
import { chosenWord } from "../stops";
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
 * says the group's. Its text cannot be selected. A material that holds a gap
 * is drawn in the cloze card. A gap bound to a choice question is a chip
 * with the question's number and the chosen word, open while `openId` is
 * its question; a gap bound to a blank is underlined. Either goes to the
 * question or blank it stands for. Each recording's
 * card stands at its audio node; with several, the node's label is printed
 * above each card, and a recording no node names, which the server refuses,
 * is drawn after the materials so no payload can hide one. Given `drawer`,
 * below 768px, it is a drawer from the left over the question, with its bar
 * at the top; closed, it is out of sight and inert, and stays mounted, so it
 * keeps its place in the text and a recording keeps playing.
 */
export const GroupContext = memo(function GroupContext({
  group,
  sectionTitle,
  numbers,
  openId,
  onGap,
  onRetryMedia,
  wide,
  drawer,
}: Readonly<{
  group: StudentGroup;
  sectionTitle?: string | undefined;
  numbers: ReadonlyMap<string, number>;
  openId?: string | undefined;
  onGap: (gap: MaterialGap) => void;
  onRetryMedia: () => void;
  wide: boolean;
  drawer?: DrawerPane | undefined;
}>) {
  const { t } = useTranslation();
  const { questions, answers } = usePaper();
  const pane = useRef<HTMLElement>(null);
  const onScroll = useKeptScroll(pane, false);
  const head = materialsHead(group, sectionTitle);
  const closed = drawer !== undefined && !drawer.open && drawer.offset === null;
  const ordinals = useMemo(
    () =>
      new Map(group.recordings.map((recording, index) => [recording.id, index + 1])),
    [group.recordings],
  );
  const unnamed = useMemo(() => unnamedRecordings(group), [group]);
  const several = group.recordings.length > 1;
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
          renderAudio={(node, asset, recording) => (
            <SharedAudio
              recording={recording}
              asset={asset}
              label={
                nfc(node.label).trim() ||
                t("takeTest.sharedAudioLabel", { n: ordinals.get(recording.id) ?? 1 })
              }
              printed={several}
              onRetry={onRetryMedia}
            />
          )}
          gapCard
          renderGap={(gap, label) => {
            const number = numbers.get(gap.questionId);
            if (number == null) return <span className="content-gap">{label}</span>;
            const question = questions[number - 1];
            if (
              gap.kind === "question" &&
              question !== undefined &&
              questionKind(question) === "choice"
            ) {
              const word = chosenWord(question, answers[question.id], t);
              return (
                <GapChip
                  number={number}
                  word={word}
                  open={question.id === openId}
                  label={t("takeTest.cloze.chip", {
                    n: number,
                    word: word ?? t("takeTest.cloze.empty"),
                  })}
                  onClick={() => onGap(gap)}
                />
              );
            }
            return (
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
        {unnamed.length > 0 && (
          <GroupListening
            group={group}
            recordings={unnamed}
            onRetryMedia={onRetryMedia}
          />
        )}
      </PassageBody>
    </article>
  );
});

type Recording = StudentGroup["recordings"][number];

function unnamedRecordings(group: StudentGroup): Recording[] {
  const named = new Set(
    group.stimuli.flatMap((material) => audioAssetIds(material.content)),
  );
  return group.recordings.filter((recording) => !named.has(recording.assetId));
}

function audioAssetIds(content: unknown): string[] {
  if (Array.isArray(content)) return content.flatMap(audioAssetIds);
  if (typeof content !== "object" || content === null) return [];
  const node = content as Record<string, unknown>;
  if (node.type === "audio" && typeof node.assetId === "string") return [node.assetId];
  return Object.values(node).flatMap(audioAssetIds);
}

/**
 * GroupListening draws the cards of a group's recordings one after another,
 * each labelled "Recording n", all of them unless `recordings` names some.
 * The stimulus pane uses it for a recording no audio node names.
 */
export const GroupListening = memo(function GroupListening({
  group,
  recordings = group.recordings,
  onRetryMedia,
}: Readonly<{
  group: StudentGroup;
  recordings?: readonly Recording[];
  onRetryMedia: () => void;
}>) {
  const { t } = useTranslation();
  const assets = useMemo(
    () => new Map(group.assets.map((asset) => [asset.id, asset])),
    [group.assets],
  );
  const several = group.recordings.length > 1;
  return (
    <div className="flex min-w-0 flex-col gap-5">
      {recordings.map((recording) => {
        const asset = assets.get(recording.assetId);
        const n = group.recordings.indexOf(recording) + 1;
        return asset?.kind === "audio" && asset.url ? (
          <SharedAudio
            key={recording.id}
            recording={recording}
            label={t("takeTest.sharedAudioLabel", { n })}
            printed={several}
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
  printed,
  asset,
  onRetry,
}: Readonly<{
  recording: Recording;
  label: string;
  printed: boolean;
  asset: StudentGroup["assets"][number];
  onRetry: () => void;
}>) {
  const { t } = useTranslation();
  const paper = usePaper();
  const startedAt = useRef<number | null>(null);
  const played = paper.recordingPlays[recording.id] ?? 0;
  const pending = paper.pendingRecordings.has(recording.id);
  const locked = paper.lock !== null || paper.submitState !== "idle";
  return (
    <div data-shared-audio="" className="flex min-w-0 flex-col gap-2">
      {printed && <p className="text-muted-fg text-ui">{label}</p>}
      <AudioPlayer
        layout="card"
        src={asset.url}
        label={label}
        durationMs={asset.durationMs}
        allowSeek={recording.policy.allowSeek}
        allowPause={recording.policy.allowPause}
        disabled={locked}
        preload="metadata"
        plays={{ played, limit: recording.policy.maxPlays ?? null }}
        place={`recording:${recording.id}`}
        onPlay={() => {
          startedAt.current = Date.now();
          paper.noteRecordingPlay(recording.id);
        }}
        onResume={() => {
          startedAt.current = Date.now();
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
