import { useEffect, useRef, useState } from "react";
import type { IntegrityPolicy } from "@/features/take-test/api";
import { beginSession, drain, pending, record } from "./buffer";
import { sendBeaconFlush } from "./beacon";
import { isFullscreen } from "./fullscreen";

/** What the screen renders from the monitor. Everything else it records. */
export interface IntegrityStatus {
  /** Away episodes at or over `minAwayMs`, as seen by this hook instance. */
  strikes: number;
  /** How long the latest counted episode lasted. Null until there is one. */
  lastAwayMs: number | null;
  fullscreen: boolean;
}

/** §10's signals, in one hook and one effect. */
export function useIntegrityMonitor({
  attemptId,
  sessionId,
  beaconToken,
  policy,
}: {
  attemptId: string | null;
  sessionId: string | null;
  beaconToken: string;
  policy: IntegrityPolicy | null;
}): IntegrityStatus {
  const [episodes, setEpisodes] = useState<
    Pick<IntegrityStatus, "strikes" | "lastAwayMs">
  >({
    strikes: 0,
    lastAwayMs: null,
  });
  const [fullscreen, setFullscreen] = useState(isFullscreen);

  const latest = useRef({ attemptId, sessionId, beaconToken, policy });
  useEffect(() => {
    latest.current = { attemptId, sessionId, beaconToken, policy };
  });

  useEffect(() => {
    if (attemptId === null || sessionId === null) return;
    beginSession(attemptId, sessionId);

    // One episode at a time.
    let awaySince: number | null = null;

    const note = (kind: string, meta?: Record<string, unknown>) =>
      record(attemptId, kind, meta === undefined ? {} : { meta });

    const leave = (kind: string) => {
      note(kind);
      if (awaySince === null) awaySince = Date.now();
    };

    const returned = (kind: string) => {
      if (awaySince === null) {
        note(kind);
        return;
      }
      const awayMs = Date.now() - awaySince;
      awaySince = null;
      note(kind, { awayMs });
      if (awayMs >= (latest.current.policy?.minAwayMs ?? 3000)) {
        setEpisodes((prev) => ({ strikes: prev.strikes + 1, lastAwayMs: awayMs }));
      }
    };

    const onVisibility = () =>
      document.hidden ? leave("tab_hidden") : returned("tab_visible");
    const onBlur = () => leave("window_blur");
    const onFocus = () => returned("window_focus");
    const onOffline = () => note("network_offline");
    const onOnline = () => note("network_online");

    const onFullscreen = () => {
      const active = isFullscreen();
      setFullscreen(active);
      // Recorded only when the teacher asked for fullscreen (§10.1).
      if (latest.current.policy?.requireFullscreen === true) {
        note(active ? "fullscreen_enter" : "fullscreen_exit");
      }
    };

    const onContextMenu = () => {
      note("context_menu");
    };

    const onClipboard = (event: ClipboardEvent) => {
      note(event.type);
      if (latest.current.policy?.blockCopyPaste === true) event.preventDefault();
    };

    const onPageHide = () => {
      note("page_hide");
      const { sessionId: session, beaconToken: token } = latest.current;
      if (session === null || token === "") return;
      sendBeaconFlush({
        attemptId,
        sessionId: session,
        beaconToken: token,
        events: drain(attemptId),
      });
    };

    document.addEventListener("visibilitychange", onVisibility);
    window.addEventListener("blur", onBlur);
    window.addEventListener("focus", onFocus);
    window.addEventListener("offline", onOffline);
    window.addEventListener("online", onOnline);
    document.addEventListener("fullscreenchange", onFullscreen);
    document.addEventListener("contextmenu", onContextMenu);
    document.addEventListener("copy", onClipboard);
    document.addEventListener("cut", onClipboard);
    document.addEventListener("paste", onClipboard);
    window.addEventListener("pagehide", onPageHide);

    return () => {
      document.removeEventListener("visibilitychange", onVisibility);
      window.removeEventListener("blur", onBlur);
      window.removeEventListener("focus", onFocus);
      window.removeEventListener("offline", onOffline);
      window.removeEventListener("online", onOnline);
      document.removeEventListener("fullscreenchange", onFullscreen);
      document.removeEventListener("contextmenu", onContextMenu);
      document.removeEventListener("copy", onClipboard);
      document.removeEventListener("cut", onClipboard);
      document.removeEventListener("paste", onClipboard);
      window.removeEventListener("pagehide", onPageHide);
    };
  }, [attemptId, sessionId]);

  return { ...episodes, fullscreen };
}

/** What the audio player reports, which no listener here can observe. */
export function recordAudioEvent(
  attemptId: string,
  kind: "audio_play" | "audio_ended" | "audio_blocked",
  questionId: string,
): void {
  record(attemptId, kind, { questionId });
}

export { pending };
