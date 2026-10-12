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

/** IconTileProps supplies the decorative icon, square size and tone. */
export type IconTileProps = Readonly<{
  icon: LucideIcon;
  size?: IconTileSize | undefined;
  tone?: IconTileTone | undefined;
  className?: string | undefined;
}>;

/** IconTile hides its decorative icon from assistive technology and allows caller size overrides. */
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
