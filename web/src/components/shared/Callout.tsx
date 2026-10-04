import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Info, TriangleAlert, X, type LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";

type CalloutTone = "neutral" | "info" | "warning" | "danger" | "success";
type CalloutSize = "sm" | "md" | "lg";

const SIZES: Record<CalloutSize, string> = {
  lg: "gap-2.5 rounded-[0.625rem] px-3.5 py-3 text-sm leading-[1.55] [&>svg]:size-4",
  md: "gap-2 rounded-[0.5rem] px-3 py-2.5 text-sm leading-normal [&>svg]:size-[0.9375rem]",
  sm: "text-meta gap-2 rounded-[0.5rem] px-2.5 py-2 leading-normal [&>svg]:size-3.5",
};

const TONES: Record<CalloutTone, string> = {
  neutral: "bg-muted [&>svg]:text-fg",
  info: "bg-info-soft [&>svg]:text-info-ink",
  warning: "bg-warning-soft [&>svg]:text-warning-ink",
  danger: "bg-danger-soft [&>svg]:text-danger-ink",
  success: "bg-success-soft [&>svg]:text-success-ink",
};

const ICONS: Record<CalloutTone, LucideIcon> = {
  neutral: Info,
  info: Info,
  warning: TriangleAlert,
  danger: TriangleAlert,
  success: Info,
};

function textClass(actions: boolean, dismissible: boolean) {
  if (actions) return "min-w-0 flex-[1_1_17.5rem]";
  if (dismissible) return "min-w-0 flex-1";
  return "min-w-0";
}

/**
 * CalloutProps is what a Callout takes: its tone and size, an icon in place
 * of the tone's own, a bold lead before the text, the text, controls after
 * it, a handler that adds the dismiss button, and whether it is announced.
 */
export type CalloutProps = Readonly<{
  tone?: CalloutTone | undefined;
  size?: CalloutSize | undefined;
  icon?: LucideIcon | undefined;
  lead?: string | undefined;
  children: ReactNode;
  actions?: ReactNode;
  onDismiss?: (() => void) | undefined;
  announce?: boolean | undefined;
  className?: string | undefined;
}>;

/**
 * Callout is the deck's note beside a form or above a list: a tinted block
 * with an icon and a sentence. `lg`, the default, is the 13px note on a 10px
 * radius, `md` the tighter one on 8px and `sm` the 12.5px line under a field.
 * Neutral is the muted fill with the icon in the text colour; a tone is its
 * soft fill with the icon in its ink, and the text is the text colour in
 * every tone. It is a static note with no role; `announce` makes it an alert,
 * for a callout that appears in answer to something the user did. `actions`
 * are laid out after the text as items of the callout's own row, which then
 * wraps, and `onDismiss` adds a button that calls it. The icon's size, colour
 * and top margin are set from the root with `[&>svg]:`, and `className` is
 * applied last, so a screen restates any of them, the gap or the radius with
 * one class each.
 */
export function Callout({
  tone = "neutral",
  size = "lg",
  icon,
  lead,
  children,
  actions,
  onDismiss,
  announce = false,
  className,
}: CalloutProps) {
  const { t } = useTranslation();
  const Icon = icon ?? ICONS[tone];
  const hasActions = actions !== undefined && actions !== null && actions !== false;
  return (
    <div
      role={announce ? "alert" : undefined}
      className={cn(
        "text-fg flex items-start [&>svg]:mt-0.5 [&>svg]:shrink-0",
        SIZES[size],
        TONES[tone],
        hasActions && "flex-wrap",
        className,
      )}
    >
      <Icon aria-hidden="true" />
      <div className={textClass(hasActions, onDismiss !== undefined)}>
        {lead !== undefined && (
          <>
            <strong className="font-semibold">{lead}</strong>{" "}
          </>
        )}
        {children}
      </div>
      {hasActions && actions}
      {onDismiss !== undefined && (
        <button
          type="button"
          aria-label={t("display.dismiss")}
          onClick={onDismiss}
          className="text-muted-fg grid size-6 shrink-0 place-items-center rounded-[0.375rem]"
        >
          <X aria-hidden="true" className="size-3.5" />
        </button>
      )}
    </div>
  );
}
