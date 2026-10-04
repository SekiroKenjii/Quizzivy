/**
 * The Fullscreen API, read defensively.
 *
 * jsdom leaves `fullscreenElement` undefined rather than null, and iPhone
 * Safari has no element fullscreen at all, so every reader here treats "not
 * there" as "not fullscreen" instead of trusting a strict null check.
 */
export function isFullscreen(): boolean {
  return (document.fullscreenElement ?? null) !== null;
}

export function fullscreenSupported(): boolean {
  return document.fullscreenEnabled === true;
}

/**
 * Best effort, from a click. Browsers grant fullscreen only inside a user
 * gesture (§10.2), which is why the intro's "Bắt đầu" and the bar's "Quay lại
 * toàn màn hình" are the two callers and nothing runs this from an effect.
 */
export async function enterFullscreen(): Promise<void> {
  if (!fullscreenSupported() || isFullscreen()) return;
  try {
    await document.documentElement.requestFullscreen({ navigationUI: "hide" });
  } catch {
    // Refused, or the gesture had already been spent.
  }
}

/**
 * exitFullscreen leaves the fullscreen the engine asked for, best effort,
 * once an attempt has ended, and reports whether it did. It does nothing
 * when the document is not in fullscreen, and a refusal is swallowed: a paper
 * that is already handed in must not fail on the way out. Unlike entering,
 * leaving needs no gesture, so an effect may call it.
 */
export async function exitFullscreen(): Promise<boolean> {
  if (!isFullscreen()) return false;
  try {
    await document.exitFullscreen();
    return true;
  } catch {
    return false;
  }
}
