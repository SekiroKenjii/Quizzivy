import type { ComponentProps } from "react";
import { useTranslation } from "react-i18next";
import type { LucideIcon } from "lucide-react";
import { Tooltip } from "@/components/shared/Tooltip";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { AI_BLOCK_KEYS, type AiBlock } from "../useAiAvailable";

/**
 * AiEntryButton is an outlined button that opens an AI panel, its icon in the
 * accent ink. While `blocked` names why the assistant cannot be used, or
 * `unavailable` why this entry cannot, the button stays visible and focusable
 * but does nothing, and the reason is its tooltip (Q34).
 */
export function AiEntryButton({
  icon: Icon,
  label,
  blocked,
  unavailable,
  className,
  onClick,
  ...rest
}: Readonly<
  {
    icon: LucideIcon;
    label: string;
    blocked: AiBlock | null;
    unavailable?: string | null | undefined;
  } & Omit<ComponentProps<typeof Button>, "children" | "variant">
>) {
  const { t } = useTranslation();
  const reason = blocked === null ? (unavailable ?? null) : t(AI_BLOCK_KEYS[blocked]);
  const button = (
    <Button
      type="button"
      variant="outline"
      {...rest}
      aria-disabled={reason === null ? undefined : true}
      className={cn(
        "aria-disabled:hover:bg-card aria-disabled:cursor-default aria-disabled:opacity-50",
        className,
      )}
      onClick={(event) => {
        if (reason === null) onClick?.(event);
      }}
    >
      <Icon aria-hidden="true" className="text-brand-ink" />
      {label}
    </Button>
  );
  if (reason === null) return button;
  return <Tooltip label={reason}>{button}</Tooltip>;
}
