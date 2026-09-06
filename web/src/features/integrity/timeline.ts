import type { IntegrityEvent } from "@/features/attempts/api";

/** The G-05 filter chips. */
export type TimelineFilter = "all" | "away" | "audio" | "network";

const AWAY_OPENERS = new Set(["tab_hidden", "window_blur", "fullscreen_exit"]);
const AWAY_CLOSERS = new Set(["tab_visible", "window_focus", "fullscreen_enter"]);
const AUDIO = new Set(["audio_play", "audio_ended", "audio_blocked"]);
const NETWORK = new Set(["network_offline", "network_online"]);
const CLOSERS = new Set([...AWAY_CLOSERS, "network_online", "audio_ended"]);

/** Which chip an event belongs under; `null` for kinds no chip narrows to. */
export function family(kind: string): Exclude<TimelineFilter, "all"> | null {
  if (AWAY_OPENERS.has(kind) || AWAY_CLOSERS.has(kind)) return "away";
  if (AUDIO.has(kind)) return "audio";
  if (NETWORK.has(kind)) return "network";
  return null;
}

/** One row of the Diễn biến table. */
export interface TimelineRow {
  event: IntegrityEvent;
  /** No return yet: the student is still away, still offline, still listening. */
  ongoing: boolean;
  /** For `audio_play`, which play of that question this was. */
  playNo: number | null;
}

/**
 * The rows the table draws. The server paired every episode onto its opening
 * event, so a closing event carries nothing the row above does not; and a
 * second leave inside an open episode (blur then hidden) is the same absence
 * twice. Both are dropped. An opener with no duration is kept only when nothing
 * later closed its family -- the "đang tiếp diễn" row (G-05b).
 */
export function timelineRows(
  events: IntegrityEvent[],
  filter: TimelineFilter,
): TimelineRow[] {
  const plays = new Map<string, number>();
  const rows: TimelineRow[] = [];
  events.forEach((event, index) => {
    if (CLOSERS.has(event.kind)) return;
    let playNo: number | null = null;
    if (event.kind === "audio_play") {
      const key = event.questionId ?? "";
      playNo = (plays.get(key) ?? 0) + 1;
      plays.set(key, playNo);
    }
    const opener =
      AWAY_OPENERS.has(event.kind) ||
      event.kind === "network_offline" ||
      event.kind === "audio_play";
    const ongoing = opener && event.durationMs == null && !closedLater(events, index);
    if (opener && event.durationMs == null && !ongoing) return;
    if (filter !== "all" && family(event.kind) !== filter) return;
    rows.push({ event, ongoing, playNo });
  });
  return rows;
}

function closedLater(events: IntegrityEvent[], index: number): boolean {
  const opener = events[index];
  if (opener === undefined) return false;
  const fam = family(opener.kind);
  for (let i = index + 1; i < events.length; i++) {
    const later = events[i];
    if (later === undefined) continue;
    if (!CLOSERS.has(later.kind)) continue;
    if (fam === "audio" && later.questionId !== opener.questionId) continue;
    if (family(later.kind) === fam) return true;
  }
  return false;
}

/** The trailing episode the strip must say it is not summing (G-05b). */
export function hasOpenEpisode(events: IntegrityEvent[]): boolean {
  return timelineRows(events, "away").some((row) => row.ongoing);
}

/** "2:41" from milliseconds, the strip's and the table's shape. */
export function clockSpan(ms: number): string {
  const total = Math.max(0, Math.round(ms / 1000));
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  const pad = (n: number) => String(n).padStart(2, "0");
  return h > 0 ? `${h}:${pad(m)}:${pad(s)}` : `${m}:${pad(s)}`;
}
