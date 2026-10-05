import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import {
  ArrowUpRight,
  Check,
  ChevronsUpDown,
  GraduationCap,
  type LucideIcon,
} from "lucide-react";
import { BrandMark } from "@/components/shared/Brand";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useWorkspace } from "@/features/auth/permissions";
import { cn } from "@/lib/utils";

/**
 * WorkspaceDestination is another console the signed-in user may open from
 * the workspace switcher. `label` is already translated.
 */
export interface WorkspaceDestination {
  id: string;
  label: string;
  to: string;
  icon: LucideIcon;
}

const ROW = "flex w-full items-center gap-2.5 rounded-md p-1.5 text-left";
const MENU_ROW =
  "focus:bg-hover focus:text-fg text-sm in-data-[scale=deck]:gap-2.5 in-data-[scale=deck]:p-2 in-data-[scale=deck]:leading-4";

/**
 * WorkspaceSwitcher is the block at the head of the teacher sidebar: the
 * brand mark in a tile, the product's name and the workspace's. With no
 * destination, or for a user without the admin workspace, it is a label.
 * With one or more and the admin workspace it is a button that opens the
 * deck's "Switch workspace" menu. `collapsed` leaves the tile alone.
 */
export function WorkspaceSwitcher({
  collapsed,
  destinations,
}: Readonly<{
  collapsed: boolean;
  destinations: readonly WorkspaceDestination[];
}>) {
  const { t } = useTranslation();
  const admin = useWorkspace("admin");
  const switches = admin && destinations.length > 0;

  const identity = (
    <>
      <span
        aria-hidden={collapsed ? undefined : true}
        className="bg-card grid size-8 flex-none place-items-center rounded-md border"
      >
        <BrandMark height={18} label={false} theme="auto" />
      </span>
      {!collapsed && (
        <span className="flex min-w-0 flex-1 flex-col">
          <span className="text-ui truncate leading-tight font-semibold">
            {t("app.name")}
          </span>
          <span className="text-muted-fg truncate text-xs leading-tight">
            {t("teacherShell.workspace")}
          </span>
        </span>
      )}
    </>
  );

  if (!switches) {
    return (
      <div data-slot="workspace" className={cn(ROW, collapsed && "justify-center")}>
        {identity}
      </div>
    );
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        data-slot="workspace"
        className={cn(
          ROW,
          "hover:bg-hover data-[state=open]:bg-hover",
          collapsed && "justify-center",
        )}
      >
        {identity}
        {!collapsed && (
          <ChevronsUpDown
            className="text-muted-fg size-[0.9375rem] flex-none"
            aria-hidden="true"
          />
        )}
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="start"
        sideOffset={0}
        className="data-[scale=deck]:w-60 data-[scale=deck]:p-1.5"
      >
        <DropdownMenuLabel>{t("teacherShell.switchWorkspace")}</DropdownMenuLabel>
        <DropdownMenuItem
          aria-current="true"
          className={cn(MENU_ROW, "bg-muted font-medium")}
        >
          <GraduationCap className="size-4" aria-hidden="true" />
          <span className="min-w-0 flex-1 truncate">{t("teacherShell.workspace")}</span>
          <Check className="size-[0.9375rem]" aria-hidden="true" />
        </DropdownMenuItem>
        {destinations.map((destination) => (
          <DropdownMenuItem key={destination.id} asChild className={MENU_ROW}>
            <Link to={destination.to}>
              <destination.icon className="size-4" aria-hidden="true" />
              <span className="min-w-0 flex-1 truncate">{destination.label}</span>
              <ArrowUpRight
                className="text-muted-fg size-3.5 flex-none"
                aria-hidden="true"
              />
            </Link>
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
