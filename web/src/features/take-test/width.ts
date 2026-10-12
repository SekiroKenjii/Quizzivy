import { createContext, useContext } from "react";
import { useMediaQuery } from "@/hooks/useMediaQuery";

/**
 * EngineWidthContext holds a measured width, in CSS pixels, for an engine
 * drawn inside a frame narrower than the window. Null, the default, means
 * the engine fills the window, as it does under FocusLayout.
 */
export const EngineWidthContext = createContext<number | null>(null);

/**
 * useEngineWidthAtLeast reports whether the engine is at least `px` wide:
 * the window's width through a media query, or the width an
 * EngineWidthContext measured. It is the one place the engine asks a width
 * question.
 */
export function useEngineWidthAtLeast(px: number): boolean {
  const measured = useContext(EngineWidthContext);
  const fits = useMediaQuery(`(min-width: ${px}px)`);
  return measured === null ? fits : measured >= px;
}
