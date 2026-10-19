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
          className="data-[state=open]:bg-accent rounded-seg hover:bg-hover dark:hover:bg-hover size-7.5 [&_svg:not([class*='size-'])]:size-4"
          aria-label={label ?? t("common.actions")}
          onClick={stopClick}
        >
          <Ellipsis aria-hidden="true" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="end"
        className={cn("w-55", className)}
        onClick={stopClick}
      >
        {title !== undefined && <DropdownMenuLabel>{title}</DropdownMenuLabel>}
        {children}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
