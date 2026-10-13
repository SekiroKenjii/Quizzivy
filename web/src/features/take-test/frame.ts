import { createContext, useContext } from "react";

/**
 * EngineLayerContext holds the element a framed engine's overlays portal
 * into, so the question sheet opens over the frame and not over the window.
 * Null, the default, portals them into the body, as under FocusLayout.
 */
export const EngineLayerContext = createContext<HTMLElement | null>(null);

/** useEngineLayer returns the element the engine's overlays portal into, or null for the body. */
export function useEngineLayer(): HTMLElement | null {
  return useContext(EngineLayerContext);
}
