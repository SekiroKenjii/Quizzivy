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

/** What the trigger shows, so the hint matches the key that actually works. */
export function commandKeyLabel(): string {
  const platform =
    typeof navigator === "undefined"
      ? ""
      : ((navigator as { userAgentData?: { platform?: string } }).userAgentData
          ?.platform ??
        navigator.platform ??
        "");
  return /Mac|iPhone|iPad/.test(platform) ? "⌘" : "Ctrl";
}
