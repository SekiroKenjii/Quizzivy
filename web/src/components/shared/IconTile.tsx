import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";

type IconTileSize = 28 | 30 | 32 | 34 | 36 | 40 | 42 | 44;
type IconTileTone = "neutral" | "info" | "success" | "warning" | "danger" | "accent";

const SIZES: Record<IconTileSize, string> = {
  28: "size-7 rounded-seg [&>svg]:size-[0.9375rem]",
  30: "size-7.5 rounded-[0.5rem] [&>svg]:size-[0.9375rem]",
  32: "size-8 rounded-[0.5rem] [&>svg]:size-4",
  34: "size-8.5 rounded-[0.5rem] [&>svg]:size-4",
  36: "size-9 rounded-ctl [&>svg]:size-[1.0625rem]",
  40: "size-10 rounded-[0.625rem] [&>svg]:size-[1.1875rem]",
  42: "size-10.5 rounded-[0.6875rem] [&>svg]:size-[1.3125rem]",
  44: "size-11 rounded-xl [&>svg]:size-5",
};

const TONES: Record<IconTileTone, string> = {
  neutral: "bg-muted text-fg",
  info: "bg-info-soft text-info-ink",
  success: "bg-success-soft text-success-ink",
  warning: "bg-warning-soft text-warning-ink",
  danger: "bg-danger-soft text-danger-ink",
  accent: "bg-brand-soft text-brand-ink",
};

/**
 * IconTileProps is what an IconTile takes: the icon, the side of the square
 * in pixels (32 by default) and its tone (neutral by default).
 */
export type IconTileProps = Readonly<{
  icon: LucideIcon;
  size?: IconTileSize | undefined;
  tone?: IconTileTone | undefined;
  className?: string | undefined;
}>;

/**
 * IconTile is the deck's rounded square with an icon at its centre, beside a
 * row's title or above an empty state's sentence. Each size has the radius
 * and the icon size the deck draws with it; neutral is the muted fill with
 * the icon in the text colour, and a tone is its soft fill with its ink. It
 * is decoration, hidden from assistive technology: the text beside it says
 * what the row is. The icon's size is set from the square, so a caller that
 * needs another one restates `[&>svg]:size-*` in `className`, as it restates
 * the radius.
 */
export function IconTile({
  icon: Icon,
  size = 32,
  tone = "neutral",
  className,
}: IconTileProps) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        "grid shrink-0 place-items-center",
        SIZES[size],
        TONES[tone],
        className,
      )}
    >
      <Icon />
    </span>
  );
}
