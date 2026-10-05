import type { MouseEvent, ReactNode, RefObject } from "react";
import { useTranslation } from "react-i18next";
import { Ellipsis } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";

function stopClick(event: MouseEvent) {
  event.stopPropagation();
}

/** RowMenu anchors a row’s actions to its optional trigger ref and prevents menu clicks from opening the row. */
export function RowMenu({
  children,
  className,
  title,
  label,
  triggerRef,
}: Readonly<{
  children: ReactNode;
  className?: string;
  title?: string;
  label?: string;
  triggerRef?: RefObject<HTMLButtonElement | null>;
}>) {
  const { t } = useTranslation();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          ref={triggerRef}
          variant="ghost"
          size="icon-xs"
          className="data-[state=open]:bg-accent in-data-[scale=deck]:rounded-seg in-data-[scale=deck]:hover:bg-hover dark:in-data-[scale=deck]:hover:bg-hover in-data-[scale=deck]:size-7.5 in-data-[scale=deck]:[&_svg:not([class*='size-'])]:size-4"
          aria-label={label ?? t("common.actions")}
          onClick={stopClick}
        >
          <Ellipsis aria-hidden="true" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="end"
        className={cn("data-[scale=deck]:w-55", className ?? "w-52")}
        onClick={stopClick}
      >
        {title !== undefined && <DropdownMenuLabel>{title}</DropdownMenuLabel>}
        {children}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
