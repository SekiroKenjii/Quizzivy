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

/** CalloutProps supplies the note presentation, optional actions, dismissal and announcement. */
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

/** Callout shows a static note and becomes an alert only when announcement is requested. */
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
