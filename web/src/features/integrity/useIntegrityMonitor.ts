import { useEffect, useRef, useState } from "react";
import type { IntegrityPolicy } from "@/features/take-test/api";
import { beginSession, drain, pending, record } from "./buffer";
import { sendBeaconFlush } from "./beacon";

/**
 * §10's signals, in one hook and one effect.
 *
 * "One `useIntegrityMonitor` hook owns all listeners, registered and torn down
 * in a single `useEffect`. No scattered listeners" (§10.6) -- because listeners
 * added from several components are listeners removed from several components,
 * and the one that gets missed keeps firing into a test the student has left.
 *
 * Everything here is observational. §10.6: integrity failure never blocks
 * input, and nothing in this file can prevent an answer being typed or a paper
 * being submitted. The single exception is copy/paste, which the teacher may
 * ask to have blocked -- and even that is recorded either way.
 *
 * Devtools detection is deliberately absent (§10.1). Window-size deltas and
 * `debugger` timing false-positive on zoom, split-screen and extensions, and are
 * bypassed in seconds by anyone who cares. Do not add them.
 */
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
}): { strikes: number } {
  const [strikes, setStrikes] = useState(0);

  // Read inside listeners that outlive a render, so they must not close over a
  // stale copy -- the token in particular is reissued on every resume.
  //
  // Written from an effect rather than during render: a ref assigned while
  // rendering is a write React cannot see, and the rule against it exists
  // because the value then disagrees with what was painted.
  const latest = useRef({ attemptId, sessionId, beaconToken, policy });
  useEffect(() => {
    latest.current = { attemptId, sessionId, beaconToken, policy };
  });

  useEffect(() => {
    if (attemptId === null || sessionId === null) return;
    beginSession(attemptId, sessionId);

    // One episode at a time. A tab switch fires window blur AND
    // visibilitychange, and both are recorded as the distinct signals they are
    // -- but the student left once, so they are away once.
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
      // Both endpoints are stored, so the teacher sees duration rather than a
      // bare count: a 2-second blur is a notification and a 90-second one is a
      // search (§10.1).
      note(kind, { awayMs });
      if (awayMs >= (latest.current.policy?.minAwayMs ?? 3000)) {
        setStrikes((n) => n + 1);
      }
    };

    const onVisibility = () =>
      document.hidden ? leave("tab_hidden") : returned("tab_visible");
    const onBlur = () => leave("window_blur");
    const onFocus = () => returned("window_focus");
    const onOffline = () => note("network_offline");
    const onOnline = () => note("network_online");
    const onFullscreen = () =>
      note(
        document.fullscreenElement === null ? "fullscreen_exit" : "fullscreen_enter",
      );

    const onContextMenu = () => {
      // Recorded, never blocked: blocking it breaks assistive tooling and the
      // spell-check a language learner has every right to (§10.1).
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
      // Drained, not peeked: if the beacon lands these are gone, and if it does
      // not the tab is going away with them. There is no later.
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

  return { strikes };
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
