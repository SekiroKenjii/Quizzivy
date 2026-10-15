import type { LucideIcon } from "lucide-react";
import { DECK_ICONS } from "@/lib/icons";
import type { RoleColor, RoleIcon } from "@/features/roles/api";

/** CardLook is the classes that draw a role's card in one of the seven colours. */
export interface CardLook {
  card: string;
  tile: string;
  button: string;
  swatch: string;
}

/**
 * LOOKS draws a card as the deck does: the colour token behind the card, the
 * foreground on it, and the icon tile and the card's buttons set against it.
 * `dark` is the primary pair, so it follows the theme.
 */
export const LOOKS: Readonly<Record<RoleColor, CardLook>> = {
  dark: {
    card: "bg-primary text-primary-fg border-transparent",
    tile: "bg-primary-fg/20 text-primary-fg",
    button: "border-primary-fg/35 bg-primary-fg/10",
    swatch: "bg-primary text-primary-fg",
  },
  lime: {
    card: "bg-brand-soft text-fg",
    tile: "bg-card text-fg",
    button: "bg-card",
    swatch: "bg-brand-soft text-fg",
  },
  blue: {
    card: "bg-info-soft text-fg",
    tile: "bg-card text-fg",
    button: "bg-card",
    swatch: "bg-info-soft text-fg",
  },
  green: {
    card: "bg-success-soft text-fg",
    tile: "bg-card text-fg",
    button: "bg-card",
    swatch: "bg-success-soft text-fg",
  },
  amber: {
    card: "bg-warning-soft text-fg",
    tile: "bg-card text-fg",
    button: "bg-card",
    swatch: "bg-warning-soft text-fg",
  },
  rose: {
    card: "bg-danger-soft text-fg",
    tile: "bg-card text-fg",
    button: "bg-card",
    swatch: "bg-danger-soft text-fg",
  },
  gray: {
    card: "bg-muted text-fg",
    tile: "bg-card text-fg",
    button: "bg-card",
    swatch: "bg-muted text-fg",
  },
};

/** ROLE_COLORS are the seven colours, in the order the Edit role dialog offers them. */
export const ROLE_COLORS: readonly RoleColor[] = [
  "dark",
  "lime",
  "blue",
  "green",
  "amber",
  "rose",
  "gray",
];

/** ROLE_ICONS are the twelve icons, in the order the Edit role dialog offers them. */
export const ROLE_ICONS: readonly RoleIcon[] = [
  "shield",
  "graduation-cap",
  "hand-helping",
  "user",
  "key-round",
  "crown",
  "briefcase",
  "book-open",
  "users",
  "clipboard-check",
  "headset",
  "star",
];

/** iconOf returns the component for a role's icon. */
export function iconOf(icon: RoleIcon): LucideIcon {
  return DECK_ICONS[icon];
}
