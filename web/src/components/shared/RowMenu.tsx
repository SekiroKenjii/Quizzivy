import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Ellipsis } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

/** A table row's kebab: one trigger shape, one label, the menu anchored to the row. */
export function RowMenu({
  children,
  className,
}: Readonly<{
  children: ReactNode;
  className?: string;
}>) {
  const { t } = useTranslation();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon-xs" aria-label={t("common.actions")}>
          <Ellipsis aria-hidden="true" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className={className ?? "w-52"}>
        {children}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
