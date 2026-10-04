import { useEffect, useEffectEvent, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Pause, Play } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

interface AudioPlayerProps {
  src: string;
  label: string;
  disabled?: boolean;
  /** Known before the file loads, so the total does not pop in on play. */
  durationMs?: number | null | undefined;
  /** §11.1: false locks the track to display-only for a student. */
  allowSeek?: boolean;
  /** Right-hand hint, e.g. "Còn 1 lượt nghe". Read from the server, never counted here. */
  hint?: string | undefined;
  /** A-05 puts a smaller one inside the question editor's chosen-audio card. */
  size?: "default" | "sm";
  // §11.3 asks for `metadata` so the duration renders without downloading the file.
  preload?: "none" | "metadata";
  // Fired synchronously as playback starts, inside the gesture.
  onPlay?: (() => void) | undefined;
  // Fired when playback reaches the end.
  onEnded?: (() => void) | undefined;
  // Fired when the browser refuses to start playback.
  onBlocked?: (() => void) | undefined;
  // Fired when a seek is refused.
  onSeekBlocked?: (() => void) | undefined;
  // Refetches whatever owns the asset, minting a fresh signed URL.
  onRetry?: (() => void) | undefined;
}

const PADDING = "p-3.5 px-4 in-data-[scale=deck]:px-3.5 in-data-[scale=deck]:py-2.5";

/**
 * The deck's `AudioPlayer` (foundations, §11.3): a round play button, one flat
 * track, and a time readout. Monochrome — no waveform, no equaliser. On a deck
 * surface it takes the frame of the engine's answer rows: the card colour, a
 * 1.5px border, an 11px radius and a 40px button.
 */
export function AudioPlayer({
  src,
  label,
  disabled = false,
  durationMs,
  allowSeek = true,
  hint,
  size = "default",
  preload = "none",
  onPlay,
  onEnded,
  onBlocked,
  onSeekBlocked,
  onRetry,
}: Readonly<AudioPlayerProps>) {
  const { t } = useTranslation();
  const audio = useRef<HTMLAudioElement>(null);
  const [playing, setPlaying] = useState(false);
  const [position, setPosition] = useState(0);
  const [loaded, setLoaded] = useState<number | null>(null);
  const [failedFor, setFailedFor] = useState<string | null>(null);
  const failed = failedFor === src;
  const [blockedFor, setBlockedFor] = useState<string | null>(null);
  const blocked = blockedFor === src;
  const ended = useEffectEvent(() => onEnded?.());

  const { span, shownTotal } = lengths(durationMs, loaded);
  const shownPosition = shownTotal > 0 ? Math.min(position, shownTotal) : position;
  const fraction = span > 0 ? Math.min(1, position / span) : 0;

  useEffect(() => {
    const element = audio.current;
    if (!element) return;

    const onTime = () => setPosition(element.currentTime);
    const onMeta = () =>
      setLoaded(Number.isFinite(element.duration) ? element.duration : null);
    const onEnd = () => {
      setPlaying(false);
      setPosition(0);
      ended();
    };
    const onPause = () => setPlaying(false);
    const onPlaying = () => {
      setPlaying(true);
      setBlockedFor(null);
    };

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
      // §11.3: one instance per question, and navigating away releases it.
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
      onPlay?.();
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
    } else {
      element.pause();
    }
  }

  if (failed) {
    return (
      <div
        role="alert"
        className={cn(
          "border-destructive/25 bg-destructive/5 flex items-center gap-3 rounded-lg border",
          "in-data-[scale=deck]:border-danger/25 in-data-[scale=deck]:bg-danger-soft in-data-[scale=deck]:rounded-[11px] in-data-[scale=deck]:border-[1.5px]",
          size === "sm" ? "px-3 py-2.5" : PADDING,
        )}
      >
        <p className="in-data-[scale=deck]:text-meta min-w-0 flex-1 text-xs leading-relaxed">
          {t("media.expired")}
        </p>
        {onRetry === undefined ? null : (
          <Button
            variant="outline"
            size="xs"
            onClick={() => {
              setFailedFor(null);
              onRetry();
            }}
          >
            {t("common.retry")}
          </Button>
        )}
      </div>
    );
  }

  return (
    <div
      className={cn(
        "bg-background flex items-center gap-3.5 rounded-lg border",
        "in-data-[scale=deck]:bg-card in-data-[scale=deck]:gap-3 in-data-[scale=deck]:rounded-[11px] in-data-[scale=deck]:border-[1.5px]",
        size === "sm" ? "px-3 py-2.5" : PADDING,
      )}
    >
      <button
        type="button"
        onClick={toggle}
        disabled={disabled}
        aria-label={playing ? t("media.pause") : t("media.play")}
        className={cn(
          "bg-primary text-primary-foreground grid flex-none place-content-center rounded-full",
          size === "sm" ? "size-9" : "size-11 in-data-[scale=deck]:size-10",
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
          className="relative flex h-3 items-center data-[seek=true]:h-11 lg:data-[seek=true]:h-3"
        >
          <div className="bg-secondary h-1 w-full overflow-hidden rounded-full">
            <div
              className="bg-primary h-full"
              style={{ width: `${fraction * 100}%` }}
            />
          </div>
          {allowSeek ? (
            <input
              type="range"
              disabled={disabled}
              min={0}
              max={span || 1}
              step={0.1}
              value={position}
              aria-label={t("media.seek")}
              onChange={(event) => {
                const element = audio.current;
                if (!element) return;
                element.currentTime = Number(event.target.value);
                setPosition(element.currentTime);
              }}
              className={cn(
                "absolute inset-0 w-full cursor-pointer appearance-none bg-transparent",
                "[&::-webkit-slider-thumb]:bg-primary [&::-webkit-slider-thumb]:size-3 [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:rounded-full",
                "[&::-moz-range-thumb]:bg-primary [&::-moz-range-thumb]:size-3 [&::-moz-range-thumb]:rounded-full [&::-moz-range-thumb]:border-0",
                "rounded-full",
              )}
            />
          ) : null}
        </div>

        <div
          className={cn(
            "flex flex-wrap items-center justify-between gap-x-3 gap-y-1",
            size === "sm" ? "mt-1.5" : "mt-2",
          )}
        >
          <span className="text-muted-foreground in-data-[scale=deck]:text-meta shrink-0 text-xs whitespace-nowrap tabular-nums">
            {clock(shownPosition)}
            {" / "}
            {clock(shownTotal)}
          </span>
          {hint === undefined ? null : (
            <span
              aria-live="polite"
              className="text-muted-foreground in-data-[scale=deck]:text-meta text-xs wrap-break-word"
            >
              {hint}
            </span>
          )}
        </div>

        {blocked ? (
          <p
            role="alert"
            className="text-danger-ink in-data-[scale=deck]:text-meta mt-1.5 text-xs leading-relaxed"
          >
            {t("media.playBlocked")}
          </p>
        ) : null}
      </div>

      {/* eslint-disable-next-line jsx-a11y/media-has-caption -- the transcript is a field on the question (§11.1) */}
      <audio
        ref={audio}
        src={src}
        preload={preload}
        aria-label={label}
        onError={() => setFailedFor(src)}
        onSeeking={(event) => {
          if (allowSeek) return;
          // Put it back and say so.
          const element = event.currentTarget;
          if (Math.abs(element.currentTime - position) < 0.5) return;
          element.currentTime = position;
          onSeekBlocked?.();
        }}
      />
    </div>
  );
}

function iconSize(size: "default" | "sm"): string {
  return size === "sm"
    ? "size-4 fill-current"
    : "size-5 fill-current in-data-[scale=deck]:size-[17px]";
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
