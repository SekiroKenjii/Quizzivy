import {
  ArrowDown,
  ArrowUp,
  Command,
  CornerDownLeft,
  type LucideIcon,
} from "lucide-react";
import { useTranslation } from "react-i18next";

import { cn } from "@/lib/utils";

const GLYPHS = {
  command: Command,
  enter: CornerDownLeft,
  up: ArrowUp,
  down: ArrowDown,
} satisfies Record<string, LucideIcon>;

/** KeyName is a key whose symbol Be Vietnam Pro does not draw. */
export type KeyName = keyof typeof GLYPHS;

/**
 * KeyGlyph draws a key's symbol (⌘, ↵, ↑, ↓) as an icon one em tall, named
 * for assistive technology, in place of a character the brand font lacks.
 */
export function KeyGlyph({
  name,
  className,
}: Readonly<{ name: KeyName; className?: string }>) {
  const { t } = useTranslation();
  const Icon = GLYPHS[name];
  return (
    <Icon
      role="img"
      aria-label={t(`keyGlyph.${name}`)}
      strokeWidth={2}
      className={cn("inline-block size-[1em] shrink-0 align-[-0.125em]", className)}
    />
  );
}
