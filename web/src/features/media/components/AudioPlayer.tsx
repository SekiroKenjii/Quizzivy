import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Pause, Play } from "lucide-react";
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
  onError?: (() => void) | undefined;
}

/**
 * The deck's `AudioPlayer` (foundations, §11.3): a round play button, one flat
 * track, and a time readout. Monochrome — no waveform, no equaliser.
 *
 * Custom rather than `<audio controls>` because the native chrome cannot be
 * made to look like this, and because §11.1's rules need a track that can be
 * display-only: a browser's own control always offers seeking, which is exactly
 * what a listening question must not.
 *
 * The element itself stays in the tree with `preload="none"`, so nothing is
 * fetched until the teacher presses play.
 */
export function AudioPlayer({
  src,
  label,
  durationMs,
  allowSeek = true,
  hint,
  size = "default",
  onError,
}: AudioPlayerProps) {
  const { t } = useTranslation();
  const audio = useRef<HTMLAudioElement>(null);
  const [playing, setPlaying] = useState(false);
  const [position, setPosition] = useState(0);
  const [loaded, setLoaded] = useState<number | null>(null);

  const total =
    loaded ?? (durationMs != null && durationMs > 0 ? durationMs / 1000 : 0);
  const fraction = total > 0 ? Math.min(1, position / total) : 0;

  useEffect(() => {
    const element = audio.current;
    if (!element) return;

    // Listeners rather than an effect body that writes state: these fire from
    // the element, so nothing here runs during render.
    const onTime = () => setPosition(element.currentTime);
    const onMeta = () =>
      setLoaded(Number.isFinite(element.duration) ? element.duration : null);
    const onEnd = () => {
      setPlaying(false);
      setPosition(0);
    };
    element.addEventListener("timeupdate", onTime);
    element.addEventListener("loadedmetadata", onMeta);
    element.addEventListener("ended", onEnd);
    element.addEventListener("pause", () => setPlaying(false));
    element.addEventListener("play", () => setPlaying(true));
    return () => {
      element.removeEventListener("timeupdate", onTime);
      element.removeEventListener("loadedmetadata", onMeta);
      element.removeEventListener("ended", onEnd);
    };
  }, []);

  function toggle() {
    const element = audio.current;
    if (!element) return;
    if (element.paused) {
      // Called straight out of the click, not from a promise: iOS Safari only
      // honours play() inside the gesture that triggered it (§11.3).
      void element.play().catch(() => onError?.());
    } else {
      element.pause();
    }
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
        {/* One visual track for both cases, with the seek control laid over it
          when seeking is allowed. Styling a bare range input to look like the
          deck's 4px track means fighting three different engines over where the
          thumb sits; this way the track is the deck's and the input only has to
          be invisible. */}
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
            <span className="text-muted-foreground truncate text-xs">{hint}</span>
          )}
        </div>
      </div>

      {/* eslint-disable-next-line jsx-a11y/media-has-caption -- the transcript
          is a field on the question, shown separately per §11.1's policy. */}
      <audio
        ref={audio}
        src={src}
        preload="none"
        aria-label={label}
        onError={() => onError?.()}
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
