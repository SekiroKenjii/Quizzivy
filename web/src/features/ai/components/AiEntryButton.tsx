import type { ComponentProps } from "react";
import { useTranslation } from "react-i18next";
import type { LucideIcon } from "lucide-react";
import { Tooltip } from "@/components/shared/Tooltip";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { AI_BLOCK_KEYS, type AiBlock } from "../useAiAvailable";

/**
 * AiEntryButton is an outlined button that opens an AI panel, its icon in the
 * accent ink. While `blocked` names a reason the button stays visible and
 * focusable but does nothing, and the reason is its tooltip (Q34).
 */
export function AiEntryButton({
  icon: Icon,
  label,
  blocked,
  className,
  onClick,
  ...rest
}: Readonly<
  {
    icon: LucideIcon;
    label: string;
    blocked: AiBlock | null;
  } & Omit<ComponentProps<typeof Button>, "children" | "variant">
>) {
  const { t } = useTranslation();
  const button = (
    <Button
      type="button"
      variant="outline"
      {...rest}
      aria-disabled={blocked === null ? undefined : true}
      className={cn(
        "aria-disabled:hover:bg-card aria-disabled:cursor-default aria-disabled:opacity-50",
        className,
      )}
      onClick={(event) => {
        if (blocked === null) onClick?.(event);
      }}
    >
      <Icon aria-hidden="true" className="text-brand-ink" />
      {label}
    </Button>
  );
  if (blocked === null) return button;
  return <Tooltip label={t(AI_BLOCK_KEYS[blocked])}>{button}</Tooltip>;
}
