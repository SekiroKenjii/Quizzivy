import type { CSSProperties } from "react";

/** EDITOR_BOX is the classes of the editor's bordered box, shared by the rich and the Markdown body. */
export const EDITOR_BOX =
  "content-editor bg-card shadow-card relative min-w-0 rounded-[10px] border transition-[border-color,box-shadow] duration-150 motion-reduce:transition-none";

/** frameStyle sets the box's body minimum height and text size, in pixels, where given. */
export function frameStyle(minHeight?: number, fontSize?: number): CSSProperties {
  return {
    ...(minHeight ? { "--content-editor-min-height": `${minHeight}px` } : {}),
    ...(fontSize ? { "--content-editor-font-size": `${fontSize}px` } : {}),
  } as CSSProperties;
}
