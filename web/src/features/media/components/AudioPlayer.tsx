import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Pause, Play } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

interface AudioPlayerProps {
  src: string;
  label: string;
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
  // Fired when a seek is refused.
  onSeekBlocked?: (() => void) | undefined;
  // Refetches whatever owns the asset, minting a fresh signed URL.
  onRetry?: (() => void) | undefined;
}

/**
 * The deck's `AudioPlayer` (foundations, §11.3): a round play button, one flat
 * track, and a time readout. Monochrome — no waveform, no equaliser.
 */
export function AudioPlayer({
  src,
  label,
  durationMs,
  allowSeek = true,
  hint,
  size = "default",
  preload = "none",
  onPlay,
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

  const total =
    loaded ?? (durationMs != null && durationMs > 0 ? durationMs / 1000 : 0);
  const fraction = total > 0 ? Math.min(1, position / total) : 0;

  useEffect(() => {
    const element = audio.current;
    if (!element) return;

    const onTime = () => setPosition(element.currentTime);
    const onMeta = () =>
      setLoaded(Number.isFinite(element.duration) ? element.duration : null);
    const onEnd = () => {
      setPlaying(false);
      setPosition(0);
    };
    const onPause = () => setPlaying(false);
    const onPlaying = () => setPlaying(true);

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
  }, []);

  function toggle() {
    const element = audio.current;
    if (!element) return;
    if (element.paused) {
      const started = element.play();
      onPlay?.();
      void started.catch(() => setFailedFor(src));
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
          size === "sm" ? "px-3 py-2.5" : "p-3.5 px-4",
        )}
      >
        <p className="min-w-0 flex-1 text-xs leading-relaxed">{t("media.expired")}</p>
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
        size === "sm" ? "px-3 py-2.5" : "p-3.5 px-4",
      )}
    >
      <button
        type="button"
        onClick={toggle}
        aria-label={playing ? t("media.pause") : t("media.play")}
        className={cn(
          "bg-primary text-primary-foreground focus-visible:ring-ring grid flex-none place-content-center rounded-full focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none",
          size === "sm" ? "size-9" : "size-11",
        )}
      >
        {playing ? (
          <Pause className={iconSize(size)} aria-hidden="true" />
        ) : (
          <Play className={iconSize(size)} aria-hidden="true" />
        )}
      </button>

      <div className="min-w-0 flex-1">
        <div className="relative flex h-3 items-center">
          <div className="bg-secondary h-1 w-full overflow-hidden rounded-full">
            <div
              className="bg-primary h-full"
              style={{ width: `${fraction * 100}%` }}
            />
          </div>
          {allowSeek ? (
            <input
              type="range"
              min={0}
              max={total || 1}
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
                "focus-visible:ring-ring rounded-full focus-visible:ring-2 focus-visible:outline-none",
              )}
            />
          ) : null}
        </div>

        <div
          className={cn(
            "flex items-center justify-between gap-3",
            size === "sm" ? "mt-1.5" : "mt-2",
          )}
        >
          <span className="text-muted-foreground text-xs tabular-nums">
            {clock(position)}
            {" / "}
            {clock(total)}
          </span>
          {hint === undefined ? null : (
            <span aria-live="polite" className="text-muted-foreground truncate text-xs">
              {hint}
            </span>
          )}
        </div>
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
  return size === "sm" ? "size-4 fill-current" : "size-5 fill-current";
}

function clock(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return "0:00";
  const total = Math.floor(seconds);
  return `${Math.floor(total / 60)}:${(total % 60).toString().padStart(2, "0")}`;
}
