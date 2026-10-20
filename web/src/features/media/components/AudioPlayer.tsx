import { useEffect, useEffectEvent, useRef, useState, type ReactNode } from "react";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { Headphones, Pause, Play, RotateCcw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import {
  forgetPlace,
  keepPlace,
  placeEra,
  recallPlace,
  releaseTurn,
  takeTurn,
} from "../playback";

type PlayerSize = "default" | "sm" | "row";

/**
 * PlayCount is what a card says about the allowance: the plays the server
 * and the tab have counted, and the limit, null for none.
 */
export interface PlayCount {
  played: number;
  limit: number | null;
}

type Phase = "ready" | "playing" | "paused" | "finished";

interface AudioPlayerProps {
  src: string;
  label: string;
  disabled?: boolean;
  /** Known before the file loads, so the total does not pop in on play. */
  durationMs?: number | null | undefined;
  /** §11.1: false locks the track to display-only for a student. */
  allowSeek?: boolean;
  /**
   * §11.1: false makes the button inert while the recording plays. A pause
   * from elsewhere still happens, and "Resume" continues it for no play.
   */
  allowPause?: boolean | undefined;
  /** Right-hand hint, e.g. "Còn 1 lượt nghe". Read from the server, never counted here. */
  hint?: string | undefined;
  /**
   * "sm" is a smaller card; "row" draws no frame of its own, with a 32px
   * button and the time beside the track, for a host that is the frame (the
   * question media block's player row).
   */
  size?: PlayerSize;
  /**
   * "card" is the engine's recording card: a 52px button, a 6px track, the
   * time and the play counter under it, and a status line. It takes the
   * place of `size` and `hint`.
   */
  layout?: "card";
  /** The card's counter and status, read from the host's count. */
  plays?: PlayCount | undefined;
  /**
   * The key a play in progress is kept under while the player is away
   * (playback.ts), so a player that comes back resumes there for no play.
   */
  place?: string | undefined;
  // §11.3 asks for `metadata` so the duration renders without downloading the file.
  preload?: "none" | "metadata";
  // Fired synchronously as a play starts, inside the gesture; later for a resume that lost its place.
  onPlay?: (() => void) | undefined;
  // Fired synchronously as a paused play resumes, which spends no play.
  onResume?: (() => void) | undefined;
  // Fired when playback reaches the end.
  onEnded?: (() => void) | undefined;
  // Fired when the browser refuses to start playback.
  onBlocked?: (() => void) | undefined;
  // Fired when a seek is refused.
  onSeekBlocked?: (() => void) | undefined;
  // Refetches whatever owns the asset, minting a fresh signed URL.
  onRetry?: (() => void) | undefined;
}

const PADDING = "p-3.5 px-3.5 py-2.5";
const RESTORE_SLACK_S = 0.5;

/**
 * The deck's `AudioPlayer` (foundations, §11.3): a round play button, one flat
 * track, and a time readout. Monochrome — no waveform, no equaliser. It takes
 * the frame of the engine's answer rows: the card colour, a 1.5px border, an
 * 11px radius and a 40px button; `layout="card"` is the engine's recording
 * card instead.
 *
 * A play is spent when playback starts and no play is in progress, that is
 * from 0:00 or after an end: the press calls `onPlay`. A press that continues
 * a paused play calls `onResume` and spends none. Given `place`, a play in
 * progress and its position outlive the player; the next one under that key
 * puts the position back once the element has its metadata, and calls
 * `onPlay` after all if playback runs short of it. A host that gives the
 * player another `place` gets a new player, element and all: the one leaving
 * keeps its place, and the next starts from its own. A place kept after
 * forgetAllPlaces closed the memory the player mounted under is dropped.
 * Starting one player pauses any other that is playing. Unmounting pauses it
 * (§11.3: one instance per question, and navigating away releases it). With
 * `allowPause` false a press while playing does nothing; the button says so
 * with `aria-disabled` and stays focusable.
 */
export function AudioPlayer(props: Readonly<AudioPlayerProps>) {
  return <PlacedPlayer key={props.place} {...props} />;
}

function PlacedPlayer({
  src,
  label,
  disabled = false,
  durationMs,
  allowSeek = true,
  allowPause = true,
  hint,
  size = "default",
  layout,
  plays,
  place,
  preload = "none",
  onPlay,
  onResume,
  onEnded,
  onBlocked,
  onSeekBlocked,
  onRetry,
}: Readonly<AudioPlayerProps>) {
  const { t } = useTranslation();
  const audio = useRef<HTMLAudioElement>(null);
  const [kept] = useState(() => (place === undefined ? null : recallPlace(place)));
  const [playing, setPlaying] = useState(false);
  const [position, setPosition] = useState(kept ?? 0);
  const [live, setLive] = useState(kept !== null);
  const liveRef = useRef(kept !== null);
  const restoreTo = useRef<number | null>(kept);
  const guardAt = useRef(0);
  const [loaded, setLoaded] = useState<number | null>(null);
  const [failedFor, setFailedFor] = useState<string | null>(null);
  const failed = failedFor === src;
  const [blockedFor, setBlockedFor] = useState<string | null>(null);
  const blocked = blockedFor === src;
  const ended = useEffectEvent(() => onEnded?.());
  const countLate = useEffectEvent(() => onPlay?.());
  const keepAway = useEffectEvent((at: number, since: number) => {
    if (place !== undefined) keepPlace(place, at, since);
  });
  const forgetAway = useEffectEvent(() => {
    if (place !== undefined) forgetPlace(place);
  });

  const { span, shownTotal } = lengths(durationMs, loaded);
  const shownPosition = shownTotal > 0 ? Math.min(position, shownTotal) : position;
  const fraction = span > 0 ? Math.min(1, position / span) : 0;

  useEffect(() => {
    const element = audio.current;
    if (!element) return;
    const since = placeEra();

    const restore = () => {
      const at = restoreTo.current;
      if (at === null) return;
      guardAt.current = at;
      element.currentTime = at;
    };
    const onTime = () => {
      const at = element.currentTime;
      const expected = restoreTo.current;
      if (expected !== null && !element.paused) {
        restoreTo.current = null;
        if (at < expected - RESTORE_SLACK_S) {
          forgetAway();
          countLate();
        }
      }
      guardAt.current = at;
      setPosition(at);
    };
    const onMeta = () => {
      setLoaded(Number.isFinite(element.duration) ? element.duration : null);
      restore();
    };
    const onEnd = () => {
      liveRef.current = false;
      restoreTo.current = null;
      guardAt.current = 0;
      forgetAway();
      setLive(false);
      setPlaying(false);
      setPosition(0);
      ended();
    };
    const onPause = () => setPlaying(false);
    const onPlaying = () => {
      takeTurn(element);
      setPlaying(true);
      setBlockedFor(null);
    };

    if (element.readyState >= HTMLMediaElement.HAVE_METADATA) restore();
    element.addEventListener("timeupdate", onTime);
    element.addEventListener("loadedmetadata", onMeta);
    element.addEventListener("ended", onEnd);
    element.addEventListener("pause", onPause);
    element.addEventListener("play", onPlaying);
    return () => {
      element.removeEventListener("timeupdate", onTime);
      element.removeEventListener("loadedmetadata", onMeta);
      element.removeEventListener("ended", onEnd);
      element.removeEventListener("pause", onPause);
      element.removeEventListener("play", onPlaying);
      const at = element.currentTime;
      if (liveRef.current && restoreTo.current === null && at > 0) {
        restoreTo.current = at;
        keepAway(at, since);
      }
      releaseTurn(element);
      element.pause();
    };
  }, [failed]);

  useEffect(() => {
    if (disabled) audio.current?.pause();
  }, [disabled]);

  function toggle() {
    if (disabled) return;
    const element = audio.current;
    if (!element) return;
    if (element.paused) {
      const started = element.play();
      if (liveRef.current && (element.currentTime > 0 || restoreTo.current !== null)) {
        onResume?.();
      } else {
        liveRef.current = true;
        setLive(true);
        onPlay?.();
      }
      setBlockedFor(null);
      void started.catch((cause: unknown) => {
        const name = cause instanceof DOMException ? cause.name : null;
        if (name === "AbortError") return;
        if (name === "NotAllowedError") {
          onBlocked?.();
          setBlockedFor(src);
          return;
        }
        setFailedFor(src);
      });
    } else if (allowPause) {
      element.pause();
    }
  }

  const card = layout === "card";

  if (failed) {
    return (
      <Expired
        frame={failureFrame(card, size)}
        onRetry={
          onRetry &&
          (() => {
            setFailedFor(null);
            onRetry();
          })
        }
      />
    );
  }

  const element = (
    // eslint-disable-next-line jsx-a11y/media-has-caption -- the transcript is a field on the question (§11.1)
    <audio
      ref={audio}
      src={src}
      preload={preload}
      aria-label={label}
      onError={() => setFailedFor(src)}
      onSeeking={(event) => {
        if (allowSeek) return;
        const target = event.currentTarget;
        if (Math.abs(target.currentTime - guardAt.current) < 0.5) return;
        target.currentTime = guardAt.current;
        onSeekBlocked?.();
      }}
    />
  );
  const phase = phaseOf(playing, live && position > 0, live, plays);
  const inert = playing && !allowPause;
  const seek = allowSeek ? (
    <SeekRange
      disabled={disabled}
      span={span}
      value={card && phase === "finished" ? span : position}
      onSeek={(seconds) => {
        const target = audio.current;
        if (!target) return;
        target.currentTime = seconds;
        guardAt.current = target.currentTime;
        setPosition(target.currentTime);
      }}
    />
  ) : null;

  if (card) {
    return (
      <RecordingCard
        label={label}
        phase={phase}
        plays={plays}
        blocked={blocked}
        disabled={disabled}
        allowSeek={allowSeek}
        allowPause={allowPause}
        inert={inert}
        fraction={fraction}
        position={shownPosition}
        total={shownTotal}
        seek={seek}
        onToggle={toggle}
      >
        {element}
      </RecordingCard>
    );
  }

  const row = size === "row";
  return (
    <div className={frameClass(size)}>
      <button
        type="button"
        onClick={toggle}
        disabled={disabled}
        aria-disabled={inert || undefined}
        aria-label={playing ? t("media.pause") : t("media.play")}
        className={cn(
          "bg-primary text-primary-foreground grid flex-none place-content-center rounded-full aria-disabled:opacity-45",
          buttonSize(size),
        )}
      >
        {playing ? (
          <Pause className={iconSize(size)} aria-hidden="true" />
        ) : (
          <Play className={iconSize(size)} aria-hidden="true" />
        )}
      </button>

      <div className="min-w-0 flex-1">
        <div
          data-seek={allowSeek}
          className="engine-lg:data-[seek=true]:h-3 relative flex h-3 items-center data-[seek=true]:h-11"
        >
          <div className="bg-secondary h-1 w-full overflow-hidden rounded-full">
            <div
              className="bg-primary h-full"
              style={{ width: `${fraction * 100}%` }}
            />
          </div>
          {seek}
        </div>

        {row ? null : (
          <Readout
            size={size}
            hint={hint}
            position={shownPosition}
            total={shownTotal}
          />
        )}

        {blocked ? (
          <p role="alert" className="text-danger-ink text-meta mt-1.5 leading-relaxed">
            {t("media.playBlocked")}
          </p>
        ) : null}
      </div>

      {row ? <Clock position={shownPosition} total={shownTotal} /> : null}

      {element}
    </div>
  );
}

function Expired({
  frame,
  onRetry,
}: Readonly<{ frame: string; onRetry: (() => void) | undefined }>) {
  const { t } = useTranslation();
  return (
    <div
      role="alert"
      className={cn(
        "flex items-center gap-3",
        "border-danger/25 bg-danger-soft rounded-[11px] border-[1.5px]",
        frame,
      )}
    >
      <p className="text-meta min-w-0 flex-1 leading-relaxed">{t("media.expired")}</p>
      {onRetry === undefined ? null : (
        <Button variant="outline" size="xs" onClick={onRetry}>
          {t("common.retry")}
        </Button>
      )}
    </div>
  );
}

function SeekRange({
  disabled,
  span,
  value,
  onSeek,
}: Readonly<{
  disabled: boolean;
  span: number;
  value: number;
  onSeek: (seconds: number) => void;
}>) {
  const { t } = useTranslation();
  return (
    <input
      type="range"
      disabled={disabled}
      min={0}
      max={span || 1}
      step={0.1}
      value={value}
      aria-label={t("media.seek")}
      onChange={(event) => onSeek(Number(event.target.value))}
      className={cn(
        "absolute inset-0 w-full cursor-pointer appearance-none bg-transparent",
        "[&::-webkit-slider-thumb]:bg-primary [&::-webkit-slider-thumb]:size-3 [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:rounded-full",
        "[&::-moz-range-thumb]:bg-primary [&::-moz-range-thumb]:size-3 [&::-moz-range-thumb]:rounded-full [&::-moz-range-thumb]:border-0",
        "rounded-full",
      )}
    />
  );
}

function phaseOf(
  playing: boolean,
  inProgress: boolean,
  live: boolean,
  plays: PlayCount | undefined,
): Phase {
  if (playing) return "playing";
  if (inProgress) return "paused";
  if (!live && (plays?.played ?? 0) > 0) return "finished";
  return "ready";
}

const BUTTON_ICON = {
  ready: Play,
  playing: Pause,
  paused: Play,
  finished: RotateCcw,
} as const;

const BUTTON_NAME = {
  ready: "media.play",
  playing: "media.pause",
  paused: "media.resume",
  finished: "media.playAgain",
} as const;

/**
 * RecordingCard is the deck's recording card (S 1074-1097): the 52px button,
 * the 6px track in the brand accent with the seek control over it when
 * seeking is allowed, the time and the play counter, and the status line
 * with its headphones, which also holds the sentence for a play the browser
 * refused. A finished play shows the whole track. At the limit the button
 * stays live (spec §11.4, DG-182). A recording that cannot be paused dims the
 * button while it plays, and says so before the first play and while playing.
 */
function RecordingCard({
  label,
  phase,
  plays,
  blocked,
  disabled,
  allowSeek,
  allowPause,
  inert,
  fraction,
  position,
  total,
  seek,
  onToggle,
  children,
}: Readonly<{
  label: string;
  phase: Phase;
  plays: PlayCount | undefined;
  blocked: boolean;
  disabled: boolean;
  allowSeek: boolean;
  allowPause: boolean;
  inert: boolean;
  fraction: number;
  position: number;
  total: number;
  seek: ReactNode;
  onToggle: () => void;
  children: ReactNode;
}>) {
  const { t } = useTranslation();
  const Icon = BUTTON_ICON[phase];
  const spent = spentAll(phase, plays);
  const done = phase === "finished";
  return (
    <div
      role="group"
      aria-label={label}
      data-phase={phase}
      className="bg-card shadow-card flex min-w-0 flex-col gap-3.5 rounded-2xl border p-4.5"
    >
      <div className="flex items-center gap-3.5">
        <button
          type="button"
          onClick={onToggle}
          disabled={disabled}
          aria-disabled={inert || undefined}
          aria-label={t(BUTTON_NAME[phase])}
          className="bg-primary text-primary-fg grid size-13 flex-none place-content-center rounded-full disabled:opacity-45 aria-disabled:opacity-45"
        >
          <Icon className="size-5.5" aria-hidden="true" />
        </button>
        <div className="flex min-w-0 flex-1 flex-col gap-2">
          <div
            data-seek={allowSeek}
            className="engine-lg:data-[seek=true]:h-3 relative flex h-1.5 items-center data-[seek=true]:h-11"
          >
            <div className="bg-muted h-1.5 w-full overflow-hidden rounded-[6px]">
              <div
                className="bg-brand h-full"
                style={{ width: `${done ? 100 : fraction * 100}%` }}
              />
            </div>
            {seek}
          </div>
          <div className="text-muted-fg text-meta flex flex-wrap justify-between gap-x-2.5 gap-y-1 leading-normal tabular-nums">
            <span className="whitespace-nowrap">
              {clock(done ? total : position)}
              {" / "}
              {clock(total)}
            </span>
            {plays === undefined ? null : <span>{counterText(t, plays, spent)}</span>}
          </div>
        </div>
      </div>
      {blocked ? (
        <p
          role="alert"
          className="text-danger-ink flex items-start gap-2 text-sm leading-normal"
        >
          <Headphones className="mt-0.5 size-[15px] flex-none" aria-hidden="true" />
          {t("media.playBlocked")}
        </p>
      ) : (
        <p
          aria-live="polite"
          className="text-muted-fg flex items-center gap-2 text-sm leading-normal"
        >
          <Headphones className="size-[15px] flex-none" aria-hidden="true" />
          {statusText(t, phase, plays, allowPause)}
        </p>
      )}
      {spent && <p className="text-muted-fg text-meta">{t("media.extraPlays")}</p>}
      {children}
    </div>
  );
}

function spentAll(phase: Phase, plays: PlayCount | undefined): boolean {
  if (plays === undefined || plays.limit === null) return false;
  if (plays.played > plays.limit) return true;
  return plays.played >= plays.limit && phase === "finished";
}

function counterText(t: TFunction, plays: PlayCount, spent: boolean): string {
  const n = Math.max(1, plays.played);
  if (plays.limit === null) return t("media.playN", { n });
  if (spent) return t("media.playedOf", { played: plays.played, limit: plays.limit });
  return t("media.playOf", { n, limit: plays.limit });
}

function statusText(
  t: TFunction,
  phase: Phase,
  plays: PlayCount | undefined,
  allowPause: boolean,
): string {
  if (phase === "playing") {
    return t(allowPause ? "media.statusPlaying" : "media.statusPlayingNoPause");
  }
  if (phase === "paused") return t("media.statusPaused");
  if (phase === "ready") {
    return t(allowPause ? "media.statusReady" : "media.statusReadyNoPause");
  }
  const limit = plays?.limit ?? null;
  if (limit === null) return t("media.statusFinished");
  const left = limit - (plays?.played ?? 0);
  if (left <= 0) return t("media.statusUsedAll", { count: limit });
  return t("media.statusFinishedLeft", { count: left });
}

function failureFrame(card: boolean, size: PlayerSize): string {
  if (card) return "rounded-2xl! p-4.5!";
  return size === "default" ? PADDING : "px-3 py-2.5";
}

function Readout({
  size,
  hint,
  position,
  total,
}: Readonly<{
  size: PlayerSize;
  hint: string | undefined;
  position: number;
  total: number;
}>) {
  return (
    <div
      className={cn(
        "flex flex-wrap items-center justify-between gap-x-3 gap-y-1",
        size === "sm" ? "mt-1.5" : "mt-2",
      )}
    >
      <Clock position={position} total={total} />
      {hint === undefined ? null : (
        <span
          aria-live="polite"
          className="text-muted-foreground text-meta wrap-break-word"
        >
          {hint}
        </span>
      )}
    </div>
  );
}

function Clock({ position, total }: Readonly<{ position: number; total: number }>) {
  return (
    <span className="text-muted-foreground text-meta shrink-0 whitespace-nowrap tabular-nums">
      {clock(position)}
      {" / "}
      {clock(total)}
    </span>
  );
}

function frameClass(size: PlayerSize): string {
  if (size === "row") return "flex items-center gap-2.5";
  return cn(
    "flex items-center",
    "bg-card gap-3 rounded-[11px] border-[1.5px]",
    size === "sm" ? "px-3 py-2.5" : PADDING,
  );
}

function buttonSize(size: PlayerSize): string {
  if (size === "row") return "size-8";
  return size === "sm" ? "size-9" : "size-10";
}

function iconSize(size: PlayerSize): string {
  if (size === "row") return "size-3.5 fill-current";
  return size === "sm" ? "size-4 fill-current" : "fill-current size-[17px]";
}

function lengths(
  durationMs: number | null | undefined,
  loaded: number | null,
): { span: number; shownTotal: number } {
  const probed = durationMs != null && durationMs > 0 ? durationMs / 1000 : null;
  return { span: loaded ?? probed ?? 0, shownTotal: probed ?? loaded ?? 0 };
}

function clock(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return "0:00";
  const total = Math.floor(seconds);
  return `${Math.floor(total / 60)}:${(total % 60).toString().padStart(2, "0")}`;
}
