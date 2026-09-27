import {
  BookOpen,
  Briefcase,
  CircleAlert,
  CircleCheck,
  ClipboardCheck,
  Crown,
  GraduationCap,
  HandHelping,
  Headset,
  Info,
  KeyRound,
  Shield,
  Star,
  TriangleAlert,
  User,
  Users,
  type LucideIcon,
} from "lucide-react";

/**
 * DECK_ICONS maps the deck's icon names to lucide-react components, for icons
 * that data rather than code chooses: the role icons an admin picks from, and
 * the toast tones. An icon a component always shows is imported directly.
 */
export const DECK_ICONS = {
  shield: Shield,
  "graduation-cap": GraduationCap,
  "hand-helping": HandHelping,
  user: User,
  "key-round": KeyRound,
  crown: Crown,
  briefcase: Briefcase,
  "book-open": BookOpen,
  users: Users,
  "clipboard-check": ClipboardCheck,
  headset: Headset,
  star: Star,
  "circle-check": CircleCheck,
  "circle-alert": CircleAlert,
  "triangle-alert": TriangleAlert,
  info: Info,
} as const satisfies Record<string, LucideIcon>;

/** DeckIconName is a name the deck uses for a data-chosen icon. */
export type DeckIconName = keyof typeof DECK_ICONS;
