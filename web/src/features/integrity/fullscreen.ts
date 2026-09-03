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
