import { createContext } from "react";

/**
 * Where a screen's contextual bar lands: the element the admin shell renders
 * between the global topbar and the scrolling <main>. Null outside the shell
 * (tests, other layouts), in which case the bar renders in place.
 */
export const PageBarSlot = createContext<HTMLElement | null>(null);
