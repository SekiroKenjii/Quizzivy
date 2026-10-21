import type { ComponentProps } from "react";
import { useTranslation } from "react-i18next";
import { LoaderCircle, type LucideIcon } from "lucide-react";
import { Tooltip } from "@/components/shared/Tooltip";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { AI_BLOCK_KEYS, type AiBlock } from "../useAiAvailable";

/**
 * AiEntryButton is a button that opens or runs an AI suggestion, outlined
 * unless `variant` says ghost, its icon in the accent ink. While `blocked` names why the assistant cannot be used, or
 * `unavailable` why this entry cannot, the button stays visible and focusable
 * but does nothing, and the reason is its tooltip (Q34). While `busy`, a
 * spinner takes the icon's place and a press does nothing.
 */
export function AiEntryButton({
  icon: Icon,
  label,
  blocked,
  unavailable,
  busy = false,
  variant = "outline",
  className,
  onClick,
  ...rest
}: Readonly<
  {
    icon: LucideIcon;
    label: string;
    blocked: AiBlock | null;
    unavailable?: string | null | undefined;
    busy?: boolean;
    variant?: "outline" | "ghost";
  } & Omit<ComponentProps<typeof Button>, "children" | "variant">
>) {
  const { t } = useTranslation();
  const reason = blocked === null ? (unavailable ?? null) : t(AI_BLOCK_KEYS[blocked]);
  const button = (
    <Button
      type="button"
      variant={variant}
      {...rest}
      aria-disabled={reason === null ? undefined : true}
      aria-busy={busy || undefined}
      className={cn(
        "aria-disabled:cursor-default",
        variant === "outline"
          ? "aria-disabled:hover:bg-card"
          : "aria-disabled:hover:text-muted-fg aria-disabled:hover:bg-transparent",
        className,
      )}
      onClick={(event) => {
        if (reason === null && !busy) onClick?.(event);
      }}
    >
      {busy ? (
        <LoaderCircle
          aria-hidden="true"
          className="animate-spin motion-reduce:animate-none"
        />
      ) : (
        <Icon aria-hidden="true" className="text-brand-ink" />
      )}
      {label}
    </Button>
  );
  if (reason === null) return button;
  return <Tooltip label={reason}>{button}</Tooltip>;
}
