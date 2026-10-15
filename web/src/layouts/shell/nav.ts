import type { LucideIcon } from "lucide-react";
import type { Workspace } from "@/features/auth/permissions";

/**
 * NavTone is how a destination's count badge is drawn: `muted` for a figure
 * that informs, `urgent` for one that asks for action in the brand's accent,
 * `warning` for one that flags something to fix.
 */
export type NavTone = "muted" | "urgent" | "warning";

/**
 * NavCount names the figure of a destination's badge, the tone it is drawn in
 * and `sentence`, the locale key (with a `count`) that gives it to a screen
 * reader.
 */
export interface NavCount {
  figure: string;
  tone: NavTone;
  sentence: string;
}

/**
 * NavDestination is one entry of a console's sidebar. `label` is a locale
 * key. A figure that is null or absent from the counts draws no badge.
 */
export interface NavDestination {
  id: string;
  to: string;
  label: string;
  icon: LucideIcon;
  count?: NavCount;
}

/** NavGroup is a run of destinations under one label; the first has none. */
export interface NavGroup {
  id: string;
  label?: string;
  items: readonly NavDestination[];
}

/** NavFigures are the counts of a console's badges by figure name. */
export type NavFigures = Readonly<Record<string, number | null | undefined>>;

/** StaffWorkspace is a workspace with a console of its own: the teacher's or the Admin's. */
export type StaffWorkspace = Exclude<Workspace, "app">;
