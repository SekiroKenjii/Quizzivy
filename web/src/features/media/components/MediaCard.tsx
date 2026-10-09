import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import {
  CircleAlert,
  Download,
  Ellipsis,
  Image as ImageIcon,
  Link2,
  Pause,
  Pencil,
  Play,
  Repeat,
  Replace,
  Trash2,
} from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { toast } from "@/components/ui/sonner";
import { Waveform } from "@/components/shared/Waveform";
import type { LibraryAsset } from "@/features/media/api";
import { formatBytes, playLimitLabel } from "@/features/media/format";
import { audioLength } from "@/lib/i18n/datetime";
import { cn } from "@/lib/utils";

/** MediaCardActions are the card menu's actions, each handed the card's file. */
export type MediaCardActions = Readonly<{
  onRename: (asset: LibraryAsset) => void;
  onReplace: (asset: LibraryAsset) => void;
  onDelete: (asset: LibraryAsset) => void;
}>;

/**
 * MediaCard is one file of the Media page: a preview (the audio's play button
 * and decorative waveform, or the image's thumbnail), the name with its menu,
 * the length or size, how many questions use it and the audio's default play
 * limit. Playback follows AudioPlayer's rule: `.play()` runs in the click's
 * own tick, and `onPlay` is told after it. A card whose `active` turns false
 * pauses, so one file plays at a time. `onExpired` re-reads the library when
 * the signed URL no longer loads.
 */
export function MediaCard({
  asset,
  active,
  onPlay,
  onExpired,
  ...actions
}: Readonly<
  {
    asset: LibraryAsset;
    active: boolean;
    onPlay: (id: string) => void;
    onExpired: () => void;
  } & MediaCardActions
>) {
  const { t } = useTranslation();
  const used = asset.questionCount > 0;
  return (
    <article className="bg-card shadow-card flex min-w-0 flex-col overflow-hidden rounded-xl border">
      <div className="bg-muted relative flex h-28 items-center justify-center gap-3 px-4">
        {asset.kind === "audio" ? (
          <AudioPreview
            asset={asset}
            active={active}
            onPlay={onPlay}
            onExpired={onExpired}
          />
        ) : (
          <ImagePreview asset={asset} />
        )}
      </div>
      <div className="flex flex-1 flex-col gap-1.5 px-3.5 py-3">
        <div className="flex items-start gap-2">
          <h2 className="text-ui min-w-0 flex-1 leading-[1.35] font-medium [overflow-wrap:anywhere]">
            {asset.displayName}
          </h2>
          <CardMenu asset={asset} {...actions} />
        </div>
        <p className="text-meta text-muted-fg leading-normal">{metaLine(asset, t)}</p>
        <div className="mt-auto flex flex-wrap items-center gap-2 pt-1">
          <span
            className={cn(
              "inline-flex items-center gap-1.25 text-xs leading-normal",
              used ? "text-muted-fg" : "text-warning-ink",
            )}
          >
            {used ? (
              <Link2 aria-hidden="true" className="size-3.25" />
            ) : (
              <CircleAlert aria-hidden="true" className="size-3.25" />
            )}
            {used
              ? t("media.usedInQuestions", { count: asset.questionCount })
              : t("media.notUsed")}
          </span>
          {asset.kind === "audio" && asset.defaultMaxPlays !== null ? (
            <span className="text-muted-fg ml-auto inline-flex items-center gap-1.25 text-xs leading-normal">
              <Repeat aria-hidden="true" className="size-3.25" />
              {playLimitLabel(asset.defaultMaxPlays, t)}
            </span>
          ) : null}
        </div>
      </div>
    </article>
  );
}

function metaLine(asset: LibraryAsset, t: TFunction): string {
  const size = formatBytes(asset.bytes);
  if (asset.kind === "audio") return `${audioLength(asset.durationMs)} · ${size}`;
  if (asset.width === null || asset.height === null) return size;
  return `${t("media.dimensions", { width: asset.width, height: asset.height })} · ${size}`;
}

function AudioPreview({
  asset,
  active,
  onPlay,
  onExpired,
}: Readonly<{
  asset: LibraryAsset;
  active: boolean;
  onPlay: (id: string) => void;
  onExpired: () => void;
}>) {
  const { t } = useTranslation();
  const audio = useRef<HTMLAudioElement>(null);
  const [playing, setPlaying] = useState(false);
  const [progress, setProgress] = useState(0);
  const durationMs = asset.durationMs ?? 0;

  useEffect(() => {
    const element = audio.current;
    if (!element) return;
    const onTime = () => {
      const seconds =
        Number.isFinite(element.duration) && element.duration > 0
          ? element.duration
          : durationMs / 1000;
      setProgress(seconds > 0 ? element.currentTime / seconds : 0);
    };
    const onPlaying = () => setPlaying(true);
    const onPause = () => setPlaying(false);
    const onEnd = () => {
      setPlaying(false);
      setProgress(0);
    };
    element.addEventListener("timeupdate", onTime);
    element.addEventListener("play", onPlaying);
    element.addEventListener("pause", onPause);
    element.addEventListener("ended", onEnd);
    return () => {
      element.removeEventListener("timeupdate", onTime);
      element.removeEventListener("play", onPlaying);
      element.removeEventListener("pause", onPause);
      element.removeEventListener("ended", onEnd);
      element.pause();
    };
  }, [durationMs]);

  useEffect(() => {
    if (!active) audio.current?.pause();
  }, [active]);

  function toggle() {
    const element = audio.current;
    if (!element) return;
    if (!element.paused) {
      element.pause();
      return;
    }
    const started = element.play();
    onPlay(asset.id);
    void started.catch((cause: unknown) => {
      if (cause instanceof DOMException && cause.name === "NotAllowedError")
        toast(t("media.playBlocked"));
    });
  }

  const label = playing ? "media.pauseNamed" : "media.playNamed";
  return (
    <>
      {/* eslint-disable-next-line jsx-a11y/media-has-caption -- a library file carries no captions; a question's transcript is its own field (§11.1) */}
      <audio
        ref={audio}
        src={asset.url}
        preload="none"
        onError={() => {
          setPlaying(false);
          onExpired();
        }}
      />
      <button
        type="button"
        aria-label={t(label, { name: asset.displayName })}
        onClick={toggle}
        className="bg-primary text-primary-fg grid size-10 flex-none cursor-pointer place-items-center rounded-full"
      >
        {playing ? (
          <Pause aria-hidden="true" className="size-4.25" />
        ) : (
          <Play aria-hidden="true" className="size-4.25" />
        )}
      </button>
      <Waveform seed={asset.id} progress={progress} />
    </>
  );
}

function ImagePreview({ asset }: Readonly<{ asset: LibraryAsset }>) {
  const { t } = useTranslation();
  const [brokenFor, setBrokenFor] = useState<string | null>(null);
  if (brokenFor !== asset.url)
    return (
      <img
        src={asset.url}
        alt=""
        loading="lazy"
        decoding="async"
        className="absolute inset-0 size-full object-cover"
        onError={() => setBrokenFor(asset.url)}
      />
    );
  return (
    <span className="text-muted-fg flex flex-col items-center gap-1.5 text-xs">
      <ImageIcon aria-hidden="true" className="size-6" />
      {asset.width === null || asset.height === null
        ? null
        : t("media.dimensions", { width: asset.width, height: asset.height })}
    </span>
  );
}

function CardMenu({
  asset,
  onRename,
  onReplace,
  onDelete,
}: Readonly<{ asset: LibraryAsset } & MediaCardActions>) {
  const { t } = useTranslation();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label={t("media.actionsNamed", { name: asset.displayName })}
          className="text-muted-fg hover:bg-hover hover:text-fg data-[state=open]:bg-hover grid size-6.5 flex-none cursor-pointer place-items-center rounded-sm"
        >
          <Ellipsis aria-hidden="true" className="size-3.75" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-52 data-[scale=deck]:w-55">
        <DropdownMenuItem onSelect={() => onRename(asset)}>
          <Pencil aria-hidden="true" />
          {t("media.rename")}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => onReplace(asset)}>
          <Replace aria-hidden="true" />
          {t("media.replace")}
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <a href={asset.url} target="_blank" rel="noopener noreferrer">
            <Download aria-hidden="true" />
            {t("media.download")}
          </a>
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive" onSelect={() => onDelete(asset)}>
          <Trash2 aria-hidden="true" />
          {t("media.delete")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
