import { createContext } from "react";

/**
 * Where a screen's side columns land: the filter rail left of the content and
 * the detail panel right of it, portalled out of the scrolling content so
 * they stay put. The test builder provides the panel's.
 */
export const PageAsideSlot = createContext<HTMLElement | null>(null);
export const PageRailSlot = createContext<HTMLElement | null>(null);
