import { createContext } from "react";

/**
 * Where a screen's chrome lands in the shell.
 *
 * AdminLayout owns the scroll container, but a screen owns what sits around
 * it: G-01's contextual bar above <main>, A-06's filter rail to its left, and
 * the deck's detail panel to its right. All three are rendered by the screen
 * and portalled into the element the shell provides here, so they sit OUTSIDE
 * the scrolling main and stay put the way the topbar does -- no sticky, no
 * negative margins.
 *
 * Null means "no shell": the component renders in place. A screen that lays
 * out its own row under its own bar (the builder) provides a nearer slot.
 */
export const PageBarSlot = createContext<HTMLElement | null>(null);
export const PageAsideSlot = createContext<HTMLElement | null>(null);
export const PageRailSlot = createContext<HTMLElement | null>(null);
