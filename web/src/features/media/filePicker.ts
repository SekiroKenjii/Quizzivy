import { ACCEPT_ATTRIBUTE, IMAGE_ACCEPT_ATTRIBUTE, acceptAttribute } from "./limits";
import type { MediaKind } from "./api";

/** ANY_MEDIA_ACCEPT is a file input's `accept` for a file of either kind. */
export const ANY_MEDIA_ACCEPT = `${ACCEPT_ATTRIBUTE},${IMAGE_ACCEPT_ATTRIBUTE}`;

/**
 * openFilePicker opens a FileInput for one kind of file, or either kind, in
 * the click's own tick: the `accept` is set on the element before the click,
 * so the picker filters by it even before React renders the new value.
 */
export function openFilePicker(
  input: HTMLInputElement | null,
  kind: MediaKind | "any",
): void {
  if (input === null) return;
  input.accept = kind === "any" ? ANY_MEDIA_ACCEPT : acceptAttribute(kind);
  input.click();
}
