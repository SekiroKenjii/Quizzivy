import { KeyGlyph } from "@/components/shared/KeyGlyph";
import { commandKeyLabel, isApplePlatform } from "./useCommandPalette";

/** CommandKey is the palette shortcut's modifier on a key cap: the ⌘ glyph on Apple devices, "Ctrl" elsewhere. */
export function CommandKey() {
  return isApplePlatform() ? <KeyGlyph name="command" /> : commandKeyLabel();
}
