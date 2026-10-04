import type { MouseEvent, ReactNode } from "react";
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

/**
 * RowMenu is a row's "more" button and the menu it opens, anchored to the
 * button's end. `title` heads the menu and `label` names the button, "Actions"
 * when absent. A click on the button or inside the menu stops there, so a row
 * that opens on click stays shut; pointer and key events still pass on. On a
 * deck surface the button is the deck's 30px square and the menu is 220px
 * wide, which a caller changes with a `data-[scale=deck]:w-*` class.
 */
export function RowMenu({
  children,
  className,
  title,
  label,
}: Readonly<{
  children: ReactNode;
  className?: string;
  title?: string;
  label?: string;
}>) {
  const { t } = useTranslation();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
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
