import { createContext } from "react";

/**
 * Where a screen's chrome lands in the shell: the contextual bar above <main>,
 * the filter rail left of it, the detail panel right of it. Each is portalled
 * out of the scrolling main so it stays put.
 */
export const PageBarSlot = createContext<HTMLElement | null>(null);
export const PageAsideSlot = createContext<HTMLElement | null>(null);
export const PageRailSlot = createContext<HTMLElement | null>(null);
