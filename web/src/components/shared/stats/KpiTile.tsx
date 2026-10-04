import { Link } from "react-router";
import { ArrowRight, type LucideIcon } from "lucide-react";
import { LiveDot } from "@/components/shared/LiveDot";
import { cn } from "@/lib/utils";

type KpiTone = "warning" | "danger" | "success" | "muted";

type KpiFigure = {
  label: string;
  icon: LucideIcon;
  tone: KpiTone;
  value: string;
  hint: string;
  live?: boolean | undefined;
};

type KpiDestination = { to: string; action: string } | { to?: never; action?: never };

const ICONS: Record<KpiTone, string> = {
  warning: "text-warning",
  danger: "text-danger",
  success: "text-success",
  muted: "text-muted-fg",
};

const FRAME =
  "bg-card text-fg shadow-card flex flex-col gap-1.5 rounded-xl border px-4.5 py-4";

const FOOT =
  "text-muted-fg text-meta flex items-center justify-between gap-2 leading-4";

/**
 * KpiTileProps is what a KpiTile takes: the label, its icon and the icon's
 * tone, the value as the screen formats it, the hint under it, and whether a
 * live dot leads the label. `to` and `action` come together or not at all:
 * the route the tile opens and the words that say so.
 */
export type KpiTileProps = Readonly<KpiFigure & KpiDestination>;

/**
 * KpiTile is one of the dashboard's figures. With `to` the whole tile is a
 * link to that route, named by its label, value, hint and action, and its
 * border darkens under the pointer. Without `to` it is a plain card with the
 * same frame, label, icon, value and hint: no role, no tab stop, no hover and
 * no action, for a figure with nothing behind it to open.
 */
export function KpiTile({
  label,
  icon: Icon,
  tone,
  value,
  hint,
  live = false,
  to,
  action,
}: KpiTileProps) {
  const figure = (
    <>
      <div className="text-muted-fg flex items-center justify-between gap-2 text-sm leading-4 font-medium whitespace-nowrap">
        <div className="flex min-w-0 items-center gap-1.75">
          {live && <LiveDot />}
          <span className="truncate">{label}</span>
        </div>
        <Icon aria-hidden="true" className={cn("size-4 shrink-0", ICONS[tone])} />
      </div>
      <div className="text-kpi leading-[1.15] font-semibold tracking-[-0.02em] tabular-nums">
        {value}
      </div>
    </>
  );

  if (to === undefined) {
    return (
      <div className={FRAME}>
        {figure}
        <div className={FOOT}>
          <div>{hint}</div>
        </div>
      </div>
    );
  }
  return (
    <Link to={to} className={cn(FRAME, "hover:border-ring transition-colors")}>
      {figure}
      <div className={FOOT}>
        <div>{hint}</div>
        <div className="text-fg flex items-center gap-0.75 font-medium whitespace-nowrap">
          {action}
          <ArrowRight aria-hidden="true" className="size-[0.8125rem] shrink-0" />
        </div>
      </div>
    </Link>
  );
}
