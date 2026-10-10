import { useEffect, useState } from "react";

/**
 * ⌘K on macOS, Ctrl+K elsewhere.
 *
 * Bound on the window rather than on a component, because the palette's whole
 * claim is that it works from anywhere -- A-02 calls it the thing that keeps the
 * sidebar honest.
 */
export function useCommandPalette() {
  const [open, setOpen] = useState(false);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key.toLowerCase() !== "k" || !(event.metaKey || event.ctrlKey)) return;
      event.preventDefault();
      setOpen((wasOpen) => !wasOpen);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  return { open, setOpen };
}

/** isApplePlatform says whether this device's command modifier is ⌘ rather than Ctrl. */
export function isApplePlatform(): boolean {
  const platform =
    typeof navigator === "undefined"
      ? ""
      : ((navigator as { userAgentData?: { platform?: string } }).userAgentData
          ?.platform ??
        navigator.platform ??
        "");
  return /mac|iphone|ipad/i.test(platform);
}

/** commandKeyLabel names the command modifier in running text: "Cmd" on Apple devices, "Ctrl" elsewhere. */
export function commandKeyLabel(): string {
  return isApplePlatform() ? "Cmd" : "Ctrl";
}
