import { isMacOS, isiOS } from "@tiptap/core";

/** Shortcut is a key combination as a person reads it and as `aria-keyshortcuts` names it. */
export type Shortcut = { label: string; aria: string };

/** editorShortcut names the modifier combination the editor's keymap answers on this platform: Cmd on Apple devices, Ctrl elsewhere. */
export function editorShortcut(key: string, shift = false): Shortcut {
  if (isMacOS() || isiOS())
    return {
      label: `Cmd+${shift ? "Shift+" : ""}${key}`,
      aria: `Meta+${shift ? "Shift+" : ""}${key}`,
    };
  return {
    label: `Ctrl+${shift ? "Shift+" : ""}${key}`,
    aria: `Control+${shift ? "Shift+" : ""}${key}`,
  };
}
